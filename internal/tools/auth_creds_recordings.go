package tools

import (
	"context"
	"encoding/json"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// AuthTools returns auth method and auth token tools.
func AuthTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_auth_methods",
			`List auth methods within a scope. Shows type (password, oidc), attributes, and state.`,
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
				return apibase.ExecuteList(context.Background(), c, "/v1/auth-methods", p)
			},
		),
		Registration(
			"read_auth_method",
			`Read auth method details by ID, including type, attributes, and configuration.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The auth method ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/auth-methods", p.ID)
			},
		),
		Registration(
			"list_auth_tokens",
			`List auth tokens within a scope. Shows user ID, expiration time, last used time, and token status. Useful for auditing active sessions and token rotation.`,
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
				return apibase.ExecuteList(context.Background(), c, "/v1/auth-tokens", p)
			},
		),
	}
}

// CredentialTools returns credential store and library tools.
func CredentialTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_credential_stores",
			`List credential stores within a scope. Shows type (vault, static), attributes.`,
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
				return apibase.ExecuteList(context.Background(), c, "/v1/credential-stores", p)
			},
		),
		Registration(
			"read_credential_store",
			`Read credential store details by ID, including type, attributes, and authorized actions.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The credential store ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/credential-stores", p.ID)
			},
		),
		Registration(
			"list_credential_libraries",
			`List credential libraries within a credential store. Credential libraries define how credentials are sourced (e.g. Vault secrets, static credentials).`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":            Prop("string", "Scope ID (org or project).", nil),
				"credential_store_id":  Prop("string", "Credential store ID to list libraries within. If provided, lists libraries in that store.", nil),
				"recursive":             Prop("boolean", "Recursively list in child scopes.", false),
				"filter":                Prop("string", "bexpr filter expression.", nil),
				"page_size":             PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token":            Prop("string", "Token from previous paginated response.", nil),
			}, nil),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				var extra struct {
					CredentialStoreID string `json:"credential_store_id"`
				}
				json.Unmarshal(args, &extra)
				if extra.CredentialStoreID != "" {
					return apibase.ExecuteList(context.Background(), c, "/v1/credential-stores/"+extra.CredentialStoreID+"/credential-libraries", p)
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/credential-libraries", p)
			},
		),
		Registration(
			"read_credential_library",
			`Read credential library details by ID, including type, attributes, and credential mapping.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The credential library ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/credential-libraries", p.ID)
			},
		),
	}
}

// RecordingTools returns session recording tools.
func RecordingTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_session_recordings",
			`List session recordings. Supports filtering by session ID, user, target, and time range. Returns metadata only — actual recording content (video, asciicast) requires a separate download mechanism outside MCP scope.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":  Prop("string", "Scope ID (optional, some deployments list recordings globally).", nil),
				"recursive":  Prop("boolean", "Recursively list in child scopes.", false),
				"filter":     Prop("string", `bexpr filter. Example: '"session_id" == "s_12345"', '"create_time" > "2025-01-01T00:00:00Z"'`, nil),
				"page_size":  PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token": Prop("string", "Token from previous paginated response.", nil),
			}, nil),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteList(context.Background(), c, "/v1/session-recordings", p)
			},
		),
		Registration(
			"read_session_recording",
			`Read session recording metadata by ID, including states, connection recordings, start/end times, and associated session info. Does not return the recording content itself.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The session recording ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/session-recordings", p.ID)
			},
		),
	}
}