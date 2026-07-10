package tools

import (
	"context"
	"encoding/json"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// UserGroupRoleTools returns user, group, and role tools.
func UserGroupRoleTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_users",
			`List users within a scope. Shows user names, descriptions, and account info.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":  Prop("string", "Scope ID (org or project).", nil),
				"recursive":  Prop("boolean", "Recursively list in child scopes.", false),
				"filter":     Prop("string", "bexpr filter. Example: 'name == \"john.doe\"'", nil),
				"page_size":  PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token": Prop("string", "Token from previous paginated response.", nil),
			}, []string{"scope_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/users", p)
			},
		),
		Registration(
			"read_user",
			`Read user details by ID, including accounts, login name, email, and authorized actions.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The user ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/users", p.ID)
			},
		),
		Registration(
			"list_groups",
			`List groups within a scope. Groups contain user members and can be used for role principal assignment.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":  Prop("string", "Scope ID (org or project).", nil),
				"recursive":  Prop("boolean", "Recursively list in child scopes.", false),
				"filter":     Prop("string", "bexpr filter expression.", nil),
				"page_size":  PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token": Prop("string", "Token from previous paginated response.", nil),
			}, []string{"scope_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/groups", p)
			},
		),
		Registration(
			"read_group",
			`Read group details by ID, including member IDs and authorized actions.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The group ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/groups", p.ID)
			},
		),
		Registration(
			"list_roles",
			`List roles within a scope. Roles define permissions via grants. Supports filtering by name. Useful for permission auditing: "Why User A cannot see target A", "Show me users allowed to access target A".`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":  Prop("string", "Scope ID (org or project).", nil),
				"recursive":  Prop("boolean", "Recursively list in child scopes.", false),
				"filter":     Prop("string", `bexpr filter. Example: 'name == "intern"'`, nil),
				"page_size":  PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token": Prop("string", "Token from previous paginated response.", nil),
			}, []string{"scope_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/roles", p)
			},
		),
		Registration(
			"read_role",
			`Read role details by ID, including principals (users/groups), grants, and grant scope IDs. Used for permission auditing: check if a role has session access to a specific target.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The role ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/roles", p.ID)
			},
		),
	}
}