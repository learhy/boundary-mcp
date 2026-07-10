package tools

import (
	"context"
	"encoding/json"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// HostTools returns host-related tools: catalogs, sets, hosts.
func HostTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"list_host_catalogs",
			`List host catalogs within a scope. Host catalogs contain host sets and hosts. Supports recursive listing, filtering, and pagination.`,
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
				return apibase.ExecuteList(context.Background(), c, "/v1/host-catalogs", p)
			},
		),
		Registration(
			"read_host_catalog",
			`Read host catalog details by ID, including type and attributes.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The host catalog ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/host-catalogs", p.ID)
			},
		),
		Registration(
			"list_host_sets",
			`List host sets within a host catalog. Host sets group hosts for targeting.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":         Prop("string", "Scope ID (org or project).", nil),
				"host_catalog_id":  Prop("string", "Host catalog ID to list sets within. If provided, lists sets in that catalog. If omitted, lists all host sets in the scope.", nil),
				"recursive":         Prop("boolean", "Recursively list in child scopes.", false),
				"filter":            Prop("string", "bexpr filter expression.", nil),
				"page_size":         PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token":        Prop("string", "Token from previous paginated response.", nil),
			}, nil),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				// host_catalog_id is already parsed into p.HostCatalogID via ListParams
				return apibase.ExecuteList(context.Background(), c, "/v1/host-sets", p)
			},
		),
		Registration(
			"read_host_set",
			`Read host set details by ID, including host IDs and type.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The host set ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/host-sets", p.ID)
			},
		),
		Registration(
			"list_hosts",
			`List hosts within a host catalog. Shows IP addresses, DNS names, and attributes.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":        Prop("string", "Scope ID (org or project).", nil),
				"host_catalog_id":  Prop("string", "Host catalog ID to list hosts within. If provided, lists hosts in that catalog.", nil),
				"recursive":         Prop("boolean", "Recursively list in child scopes.", false),
				"filter":            Prop("string", "bexpr filter expression.", nil),
				"page_size":         PropWithConstraints("integer", "Page size (1-1000).", 1, 1000),
				"list_token":        Prop("string", "Token from previous paginated response.", nil),
			}, nil),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseListParams(args)
				if err != nil {
					return nil, err
				}
				// host_catalog_id is already parsed into p.HostCatalogID via ListParams
				return apibase.ExecuteList(context.Background(), c, "/v1/hosts", p)
			},
		),
		Registration(
			"read_host",
			`Read host details by ID, including IP addresses, DNS names, and attributes.`,
			ToolSchema(map[string]json.RawMessage{
				"id": Prop("string", "The host ID.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				p, err := apibase.ParseReadParams(args)
				if err != nil {
					return nil, err
				}
				return apibase.ExecuteRead(context.Background(), c, "/v1/hosts", p.ID)
			},
		),
	}
}