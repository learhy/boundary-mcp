package tools

import (
	"context"
	"encoding/json"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// WorkerTools returns worker-related tools.
func WorkerTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_workers",
			`List workers within a scope. Shows status (online/offline via last_status_time), address, tags, release version, and active connection count. Supports filtering and pagination.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":  Prop("string", "Scope ID (org or project). Use 'global' for global scope.", nil),
				"recursive":  Prop("boolean", "Recursively list in child scopes.", false),
				"filter":     Prop("string", "bexpr filter expression. Example: 'name == \"worker-prod\"', 'tags contains \"region:us-east\"'", nil),
				"page_size":  PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token": Prop("string", "Token from previous paginated response.", nil),
			}, []string{"scope_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/workers", p)
			},
		),
		Registration(
			"read_worker",
			`Read worker details by ID, including tags, storage state, downstream workers, and configuration.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The worker ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/workers", p.ID)
			},
		),
	}
}

// SessionTools returns session-related tools: list, read, cancel.
func SessionTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_sessions",
			`List sessions within a scope. Sessions represent active or past connections to targets. Supports filtering by user, target, status (active, canceling, terminated). Example filter: '"user_id" == "u_12345" && "status" == "active"'.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":  Prop("string", "Scope ID (org or project).", nil),
				"recursive":  Prop("boolean", "Recursively list in child scopes.", false),
				"filter":     Prop("string", `bexpr filter expression. Examples: '"status" == "active"', '"target_id" == "ttcp_12345"'`, nil),
				"page_size":  PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token": Prop("string", "Token from previous paginated response.", nil),
			}, []string{"scope_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/sessions", p)
			},
		),
		Registration(
			"read_session",
			`Read session details by ID, including connections, state, certificate, and authorization info.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The session ID, e.g. 's_1234567890'.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/sessions", p.ID)
			},
		),
		Registration(
			"cancel_session",
			`Cancel an active session by ID. This terminates the connection to the target. The session must be active. Example: cancel all active sessions for a target by first listing sessions with a filter, then canceling each by ID.`,
			ToolSchema(map[string]json.RawMessage{
				"id":      Prop("string", "The session ID to cancel, e.g. 's_1234567890'.", nil),
				"version": Prop("integer", "Optional version for optimistic concurrency. If omitted, the server uses automatic versioning.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var params struct {
					ID      string `json:"id"`
					Version int    `json:"version"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &params); err != nil {
						return nil, err
					}
				}
				if params.ID == "" {
					return nil, errInvalidArgs("id is required")
				}
				reqBody := map[string]interface{}{}
				if params.Version > 0 {
					reqBody["version"] = params.Version
				}
				return apibase.ExecutePost(context.Background(), c, "/v1/sessions/"+params.ID+":cancel", reqBody)
			},
		),
	}
}

// errInvalidArgs returns a formatted invalid arguments error.
func errInvalidArgs(msg string) error {
	return &invalidArgsError{msg: msg}
}

type invalidArgsError struct{ msg string }

func (e *invalidArgsError) Error() string { return e.msg }