// Package mcp implements the Model Context Protocol (MCP) JSON-RPC 2.0 layer
// for the Boundary MCP server. It handles stdio transport, capability
// negotiation, and tool dispatch.
package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// --- JSON-RPC 2.0 types ---

// JSONRPCMessage is the base interface for all JSON-RPC messages.
type JSONRPCMessage interface {
	isJSONRPCMessage()
}

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        json.RawMessage `json:"id"` // string, number, or null
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	Notification bool         `json:"-"` // true if ID is null (notification)
}

func (Request) isJSONRPCMessage() {}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

func (Response) isJSONRPCMessage() {}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// JSON-RPC error codes
const (
	ErrorCodeParseError      = -32700
	ErrorCodeInvalidRequest  = -32600
	ErrorCodeMethodNotFound  = -32601
	ErrorCodeInvalidParams   = -32602
	ErrorCodeInternalError   = -32603
)

// --- MCP protocol types ---

// InitializeRequest is the MCP initialize request params.
type InitializeRequest struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities"`
	ClientInfo      ClientInfo      `json:"clientInfo"`
}

// ClientInfo identifies the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the MCP initialize response result.
type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    ServerCaps     `json:"capabilities"`
	ServerInfo      ServerInfoType `json:"serverInfo"`
}

// ServerInfoType describes the server.
type ServerInfoType struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCaps declares what the server supports.
type ServerCaps struct {
	Tools ToolsCapability `json:"tools"`
}

// ToolsCapability indicates tool support.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ToolDefinition describes a single tool for tools/list.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListResult is the result of tools/list.
type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolCallParams is the params for tools/call.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResult is the result of tools/call.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock is a piece of tool result content.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// TextResult creates a simple text tool result.
func TextResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// ErrorResult creates an error tool result.
func ErrorResult(message string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ContentBlock{{Type: "text", Text: message}},
		IsError: true,
	}
}

// --- Message reading/writing ---

// ReadMessage reads a single newline-delimited JSON-RPC message from the reader.
func ReadMessage(r io.Reader) (*Request, error) {
	dec := json.NewDecoder(r)
	var msg Request
	if err := dec.Decode(&msg); err != nil {
		return nil, err
	}
	if msg.JSONRPC == "" {
		msg.JSONRPC = "2.0"
	}
	// Check if it's a notification (id is null or absent)
	if len(msg.ID) == 0 || string(msg.ID) == "null" {
		msg.Notification = true
	}
	return &msg, nil
}

// WriteMessage writes a JSON-RPC response to the writer.
func WriteMessage(w io.Writer, resp *Response) error {
	resp.JSONRPC = "2.0"
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// WriteNotification writes a JSON-RPC notification (no id, no response expected).
func WriteNotification(w io.Writer, method string, params json.RawMessage) error {
	msg := struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// NewResponse creates a response with the given id and result.
func NewResponse(id json.RawMessage, result interface{}) (*Response, error) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &Response{
		ID:     id,
		Result: resultJSON,
	}, nil
}

// NewErrorResponse creates a response with an error.
func NewErrorResponse(id json.RawMessage, code int, message string) *Response {
	return &Response{
		ID:    id,
		Error: &RPCError{Code: code, Message: message},
	}
}

// MustMarshal marshals a value to json.RawMessage, panicking on error.
func MustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{"type":"object"}`)
	}
	return data
}

// TrimError removes the leading "rpc error:" prefix from Go's grpc-style errors.
func TrimError(s string) string {
	return strings.TrimSpace(strings.TrimPrefix(s, "rpc error:"))
}