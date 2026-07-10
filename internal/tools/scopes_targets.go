package tools

import (
	"context"
	"encoding/json"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// ScopeTools returns the scope-related tools.
func ScopeTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_scopes",
			`List Boundary scopes (organizations or projects) within a parent scope. A scope is a container for resources — the global scope contains orgs, orgs contain projects. Use recursive=true to list all descendant scopes. Supports bexpr filtering (e.g. 'name == "my-org"' or 'type == "project"'). Returns paginated results.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":  Prop("string", `The scope ID to list children of. Use "global" for the global scope.`, "global"),
				"recursive": Prop("boolean", "If true, recursively list all descendant scopes.", false),
				"filter":     Prop("string", `bexpr filter expression. Examples: 'name == "production"', 'type == "project"'. See: https://developer.hashicorp.com/boundary/docs/filtering-and-listing-resources`, nil),
				"page_size": PropWithConstraints("integer", "Page size for client-directed pagination. If omitted, all results are fetched automatically.", 1, 1000),
			}, []string{"scope_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				if p.ScopeID == "" {
					p.ScopeID = "global"
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/scopes", p)
			},
		),
		Registration(
			"read_scope",
			`Read details of a specific scope by ID. Returns the scope name, description, type (org or project), parent scope, authorized actions, and primary auth method.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The scope ID to read. Use 'global' for the global scope.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/scopes", p.ID)
			},
		),
	}
}

// TargetTools returns the target-related tools.
func TargetTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_targets",
			`List Boundary targets within a scope. Targets define how users connect to hosts — they specify the target type (tcp, ssh, rdp), host sources, credential sources, session limits, and worker filters. Supports recursive listing across child scopes. Common filters: type ('"type" == "ssh"'), credential injection ('len(brokered_credential_source_ids) > 0'), created time ('"created_time" > "2025-06-01T00:00:00Z"'), egress filter presence ('egress_worker_filter == ""'). Use page_size for large result sets; if results are truncated, a list_token is returned for fetching the next page.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":   Prop("string", "Scope ID (org or project).", nil),
				"recursive":   Prop("boolean", "Recursively list targets in all child scopes.", false),
				"filter":      Prop("string", `bexpr filter expression. Examples: '"type" == "ssh"', 'name matches "web-.*"', 'len(brokered_credential_source_ids) > 0'`, nil),
				"page_size":   PropWithConstraints("integer", "Page size. If omitted, all results are fetched. Use for large sets (>100 targets).", 1, 1000),
				"list_token":  Prop("string", "List token from a previous paginated response to fetch the next page.", nil),
			}, []string{"scope_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/targets", p)
			},
		),
		Registration(
			"read_target",
			`Read full details of a target by ID, including credential sources, host sources, session limits, worker filters, and authorized actions.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The target ID to read, e.g. 'ttcp_1234567890'.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/targets", p.ID)
			},
		),
	}
}