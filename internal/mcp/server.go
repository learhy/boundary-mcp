package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// ToolHandler is the function signature for a registered tool handler.
// It receives raw JSON arguments and returns a tool result or an error.
type ToolHandler func(args json.RawMessage) (*ToolCallResult, error)

// ToolRegistration bundles a tool definition with its handler.
type ToolRegistration struct {
	Definition ToolDefinition
	Handler    ToolHandler
}

// Server is the MCP server. It reads JSON-RPC messages from stdin, dispatches
// to registered handlers, and writes responses to stdout.
type Server struct {
	r         io.Reader
	w         io.Writer
	logger    *Logger
	tools     map[string]*ToolRegistration
	mu        sync.Mutex
	clientInfo ClientInfo
	toolCalls map[string]int // tool name → call count
	callMu    sync.Mutex
	name      string
	version   string
}

// NewServer creates a new MCP server reading from stdin and writing to stdout.
func NewServer(name, version string) *Server {
	return &Server{
		r:         os.Stdin,
		w:         os.Stdout,
		logger:    NewLogger(),
		tools:     make(map[string]*ToolRegistration),
		toolCalls: make(map[string]int),
		name:      name,
		version:   version,
	}
}

// Logger returns the server's logger for use by tool handlers.
func (s *Server) Logger() *Logger {
	return s.logger
}

// RegisterTool adds a tool to the server's registry.
func (s *Server) RegisterTool(reg *ToolRegistration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[reg.Definition.Name] = reg
}

// RegisterTools adds multiple tools at once.
func (s *Server) RegisterTools(regs ...*ToolRegistration) {
	for _, reg := range regs {
		s.RegisterTool(reg)
	}
}

// ClientInfo returns info about the connected MCP client.
func (s *Server) ClientInfo() ClientInfo {
	return s.clientInfo
}

// ToolCallCount returns the total calls for a given tool.
func (s *Server) ToolCallCount(name string) int {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	return s.toolCalls[name]
}

// AllToolCallCounts returns a copy of the call count map.
func (s *Server) AllToolCallCounts() map[string]int {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	result := make(map[string]int, len(s.toolCalls))
	for k, v := range s.toolCalls {
		result[k] = v
	}
	return result
}

// incrementToolCall tracks usage.
func (s *Server) incrementToolCall(name string) {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	s.toolCalls[name]++
}

// Serve starts the server loop. It blocks until stdin is closed or a fatal
// error occurs.
func (s *Server) Serve() error {
	s.logger.Info("server starting", map[string]interface{}{
		"name":    s.name,
		"version": s.version,
	})

	// Handle SIGTERM/SIGINT for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		s.logger.Info("received signal, shutting down", map[string]interface{}{
			"signal": sig.String(),
		})
		os.Exit(0)
	}()

	dec := json.NewDecoder(s.r)

	for {
		var msg Request
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				s.logger.Info("stdin closed, shutting down")
				return nil
			}
			if err == io.ErrUnexpectedEOF {
				s.logger.Info("connection ended")
				return nil
			}
			s.logger.Error("failed to read message", map[string]interface{}{
				"error": err.Error(),
			})
			// Send parse error response with null id
			resp := NewErrorResponse(json.RawMessage("null"), ErrorCodeParseError, "Parse error: "+err.Error())
			if writeErr := WriteMessage(s.w, resp); writeErr != nil {
				return writeErr
			}
			continue
		}

		if msg.JSONRPC == "" {
			msg.JSONRPC = "2.0"
		}
		if msg.JSONRPC != "2.0" {
			s.logger.Warn("invalid JSON-RPC version", map[string]interface{}{
				"version": msg.JSONRPC,
			})
			continue
		}

		// Check if notification (no id or null id)
		isNotification := len(msg.ID) == 0 || string(msg.ID) == "null"

		s.handleMessage(&msg, isNotification)
	}
}

// handleMessage dispatches a single JSON-RPC message.
func (s *Server) handleMessage(msg *Request, isNotification bool) {
	switch msg.Method {
	case "initialize":
		s.handleInitialize(msg)
	case "notifications/initialized":
		// Client confirms initialization — no response needed for notifications
		s.logger.Debug("client initialized")
	case "tools/list":
		s.handleToolsList(msg, isNotification)
	case "tools/call":
		s.handleToolsCall(msg, isNotification)
	case "ping":
		s.handlePing(msg, isNotification)
	default:
		if !isNotification {
			resp := NewErrorResponse(msg.ID, ErrorCodeMethodNotFound, "Method not found: "+msg.Method)
			_ = WriteMessage(s.w, resp)
		}
		s.logger.Warn("unknown method", map[string]interface{}{
			"method": msg.Method,
		})
	}
}

func (s *Server) handleInitialize(msg *Request) {
	var params InitializeRequest
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			resp := NewErrorResponse(msg.ID, ErrorCodeInvalidParams, "Invalid initialize params: "+err.Error())
			_ = WriteMessage(s.w, resp)
			return
		}
	}
	s.clientInfo = params.ClientInfo

	s.logger.Info("client connected", map[string]interface{}{
		"clientName":    params.ClientInfo.Name,
		"clientVersion": params.ClientInfo.Version,
		"protocol":      params.ProtocolVersion,
	})

	result := InitializeResult{
		ProtocolVersion: "2025-06-18",
		Capabilities: ServerCaps{
			Tools: ToolsCapability{ListChanged: false},
		},
		ServerInfo: ServerInfoType{
			Name:    s.name,
			Version: s.version,
		},
	}

	resp, err := NewResponse(msg.ID, result)
	if err != nil {
		_ = WriteMessage(s.w, NewErrorResponse(msg.ID, ErrorCodeInternalError, err.Error()))
		return
	}
	_ = WriteMessage(s.w, resp)
}

func (s *Server) handlePing(msg *Request, isNotification bool) {
	if isNotification {
		return
	}
	resp, _ := NewResponse(msg.ID, map[string]interface{}{})
	_ = WriteMessage(s.w, resp)
}

func (s *Server) handleToolsList(msg *Request, isNotification bool) {
	if isNotification {
		return
	}

	s.mu.Lock()
	tools := make([]ToolDefinition, 0, len(s.tools))
	for _, reg := range s.tools {
		tools = append(tools, reg.Definition)
	}
	s.mu.Unlock()

	result := ToolsListResult{Tools: tools}
	resp, err := NewResponse(msg.ID, result)
	if err != nil {
		_ = WriteMessage(s.w, NewErrorResponse(msg.ID, ErrorCodeInternalError, err.Error()))
		return
	}
	_ = WriteMessage(s.w, resp)
}

func (s *Server) handleToolsCall(msg *Request, isNotification bool) {
	if isNotification {
		return
	}

	var params ToolCallParams
	if len(msg.Params) > 0 {
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			resp := NewErrorResponse(msg.ID, ErrorCodeInvalidParams, "Invalid tools/call params: "+err.Error())
			_ = WriteMessage(s.w, resp)
			return
		}
	}

	s.mu.Lock()
	reg, ok := s.tools[params.Name]
	s.mu.Unlock()

	if !ok {
		errResult := ErrorResult(fmt.Sprintf("Unknown tool: %s", params.Name))
		resp, _ := NewResponse(msg.ID, errResult)
		_ = WriteMessage(s.w, resp)
		return
	}

	s.incrementToolCall(params.Name)

	s.logger.Debug("tool call", map[string]interface{}{
		"tool": params.Name,
	})

	result, err := reg.Handler(params.Arguments)
	if err != nil {
		result = ErrorResult(err.Error())
	}

	resp, err := NewResponse(msg.ID, result)
	if err != nil {
		_ = WriteMessage(s.w, NewErrorResponse(msg.ID, ErrorCodeInternalError, err.Error()))
		return
	}
	_ = WriteMessage(s.w, resp)
}