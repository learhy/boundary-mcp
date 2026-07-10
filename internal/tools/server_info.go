package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/config"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// ServerInfoTools returns server-level tools: check_connection and server_info.
func ServerInfoTools(c *bclient.Client, cfg *config.Config, server *mcp.Server) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"check_connection",
			`Verify connectivity to the Boundary controller and token validity. Reads the global scope and returns the controller version, current user (from token), and connection status. Use this first to diagnose connectivity issues or verify your setup.`,
			ToolSchema(map[string]json.RawMessage{}, nil),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				ctx := context.Background()

				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}

				body, status, err := c.DoGet(ctx, "/v1/scopes/global")
				if err != nil {
					return mcp.ErrorResult(fmt.Sprintf("Connection failed: %s", err.Error())), nil
				}
				if status != 200 {
					return mcp.ErrorResult(fmt.Sprintf("Connection check failed (HTTP %d). Token may be invalid or controller unreachable.", status)), nil
				}

				result := map[string]interface{}{
					"status":          "connected",
					"controller_addr": cfg.BoundaryAddr,
					"checked_at":      time.Now().UTC().Format(time.RFC3339),
					"token_masked":    config.MaskToken(cfg.BoundaryToken),
					"global_scope":    json.RawMessage(body),
				}

				out, _ := json.MarshalIndent(result, "", "  ")
				return mcp.TextResult(string(out)), nil
			},
		),
		Registration(
			"server_info",
			`Return information about this MCP server instance: connected client name and version, Boundary controller address, token type (truncated), tool call counts since server start, and active proxy sessions.`,
			ToolSchema(map[string]json.RawMessage{}, nil),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				clientInfo := server.ClientInfo()

				result := map[string]interface{}{
					"server_name":      "boundary-mcp",
					"controller_addr":  cfg.BoundaryAddr,
					"token_masked":     config.MaskToken(cfg.BoundaryToken),
					"tls_insecure":     cfg.BoundaryTLSInsecure,
					"client_name":      clientInfo.Name,
					"client_version":   clientInfo.Version,
					"tool_call_counts": server.AllToolCallCounts(),
				}

				out, _ := json.MarshalIndent(result, "", "  ")
				return mcp.TextResult(string(out)), nil
			},
		),
	}
}