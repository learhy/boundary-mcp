package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// WriteTools returns create/update/delete tools for targets, host catalogs,
// host sets, hosts, credential stores, credentials, and credential libraries.
// These enable the agent to set up Boundary resources programmatically.
func WriteTools(c *bclient.Client) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{

		// ── Host Catalogs ──
		Registration(
			"create_host_catalog",
			`Create a static host catalog in a scope. Returns the host catalog ID needed to create host sets and hosts.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":    Prop("string", "Scope ID (org or project) to create the catalog in.", nil),
				"name":        Prop("string", "Name for the host catalog.", nil),
				"description": Prop("string", "Optional description.", nil),
			}, []string{"scope_id", "name"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					ScopeID     string `json:"scope_id"`
					Name        string `json:"name"`
					Description string `json:"description"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.ScopeID == "" || p.Name == "" {
					return nil, fmt.Errorf("scope_id and name are required")
				}
				body := map[string]interface{}{
					"scope_id": p.ScopeID,
					"name":     p.Name,
					"type":     "static",
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/host-catalogs", body)
				if err != nil {
					return nil, fmt.Errorf("create host catalog failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create host catalog failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),

		// ── Host Sets ──
		Registration(
			"create_host_set",
			`Create a static host set within a host catalog. Host sets group hosts for target assignment.`,
			ToolSchema(map[string]json.RawMessage{
				"host_catalog_id": Prop("string", "Host catalog ID to create the set in.", nil),
				"name":            Prop("string", "Name for the host set.", nil),
				"description":     Prop("string", "Optional description.", nil),
			}, []string{"host_catalog_id", "name"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					HostCatalogID string `json:"host_catalog_id"`
					Name          string `json:"name"`
					Description   string `json:"description"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.HostCatalogID == "" || p.Name == "" {
					return nil, fmt.Errorf("host_catalog_id and name are required")
				}
				body := map[string]interface{}{
					"host_catalog_id": p.HostCatalogID,
					"name":            p.Name,
					"type":            "static",
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/host-sets", body)
				if err != nil {
					return nil, fmt.Errorf("create host set failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create host set failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),

		// ── Hosts ──
		Registration(
			"create_host",
			`Create a static host in a host catalog. The address is the IP or hostname Boundary workers will connect to.`,
			ToolSchema(map[string]json.RawMessage{
				"host_catalog_id":   Prop("string", "Host catalog ID.", nil),
				"name":              Prop("string", "Name for the host.", nil),
				"address":           Prop("string", "IP address or hostname of the target host.", nil),
				"description":       Prop("string", "Optional description.", nil),
			}, []string{"host_catalog_id", "name", "address"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					HostCatalogID string `json:"host_catalog_id"`
					Name          string `json:"name"`
					Address       string `json:"address"`
					Description   string `json:"description"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.HostCatalogID == "" || p.Name == "" || p.Address == "" {
					return nil, fmt.Errorf("host_catalog_id, name, and address are required")
				}
				body := map[string]interface{}{
					"host_catalog_id": p.HostCatalogID,
					"name":            p.Name,
					"address":         p.Address,
					"type":            "static",
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/hosts", body)
				if err != nil {
					return nil, fmt.Errorf("create host failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create host failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),

		// ── Targets ──
		Registration(
			"create_tcp_target",
			`Create a TCP target in a scope. TCP targets are used for generic TCP access (telnet, raw TCP, etc.). Associate host sets and brokered credential sources after creation.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":                  Prop("string", "Scope ID (project) to create the target in.", nil),
				"name":                      Prop("string", "Name for the target.", nil),
				"description":               Prop("string", "Optional description.", nil),
				"default_port":              Prop("integer", "Default port to connect to on the target host.", nil),
				"host_source_ids":           ArrayProp("string", "List of host set IDs to associate as host sources."),
				"brokered_credential_source_ids":    ArrayProp("string", "Credential source IDs to broker when connecting."),
				"injected_application_credential_source_ids": ArrayProp("string", "Credential source IDs to inject."),
				"session_max_seconds":       Prop("integer", "Max session duration in seconds (default: 28800 = 8h).", nil),
			}, []string{"scope_id", "name", "default_port"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					ScopeID                          string   `json:"scope_id"`
					Name                             string   `json:"name"`
					Description                      string   `json:"description"`
					DefaultPort                      int      `json:"default_port"`
					HostSourceIDs                    []string `json:"host_source_ids"`
					BrokeredCredentialSourceIDs      []string `json:"brokered_credential_source_ids"`
					InjectedAppCredentialSourceIDs   []string `json:"injected_application_credential_source_ids"`
					SessionMaxSeconds                int      `json:"session_max_seconds"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.ScopeID == "" || p.Name == "" || p.DefaultPort == 0 {
					return nil, fmt.Errorf("scope_id, name, and default_port are required")
				}
				body := map[string]interface{}{
					"scope_id":   p.ScopeID,
					"name":       p.Name,
					"type":       "tcp",
					"attributes": map[string]interface{}{
						"default_port": p.DefaultPort,
					},
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				if len(p.HostSourceIDs) > 0 {
					body["host_source_id"] = p.HostSourceIDs
				}
				if len(p.BrokeredCredentialSourceIDs) > 0 {
					body["brokered_credential_source_id"] = p.BrokeredCredentialSourceIDs
				}
				if len(p.InjectedAppCredentialSourceIDs) > 0 {
					body["injected_application_credential_source_id"] = p.InjectedAppCredentialSourceIDs
				}
				if p.SessionMaxSeconds > 0 {
					body["session_max_seconds"] = p.SessionMaxSeconds
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/targets", body)
				if err != nil {
					return nil, fmt.Errorf("create tcp target failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create tcp target failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),
		Registration(
			"create_ssh_target",
			`Create an SSH target in a scope. SSH targets are used for SSH access to hosts. Associate host sets and credential sources (brokered or injected) after creation.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":                  Prop("string", "Scope ID (project) to create the target in.", nil),
				"name":                      Prop("string", "Name for the target.", nil),
				"description":               Prop("string", "Optional description.", nil),
				"default_port":              Prop("integer", "Default SSH port (typically 22).", nil),
				"host_source_ids":           ArrayProp("string", "List of host set IDs to associate as host sources."),
				"brokered_credential_source_ids":    ArrayProp("string", "Credential source IDs to broker (username/password or ssh_private_key)."),
				"injected_application_credential_source_ids": ArrayProp("string", "Credential source IDs to inject."),
				"session_max_seconds":       Prop("integer", "Max session duration in seconds (default: 28800 = 8h).", nil),
			}, []string{"scope_id", "name", "default_port"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					ScopeID                          string   `json:"scope_id"`
					Name                             string   `json:"name"`
					Description                      string   `json:"description"`
					DefaultPort                      int      `json:"default_port"`
					HostSourceIDs                    []string `json:"host_source_ids"`
					BrokeredCredentialSourceIDs      []string `json:"brokered_credential_source_ids"`
					InjectedAppCredentialSourceIDs   []string `json:"injected_application_credential_source_ids"`
					SessionMaxSeconds                int      `json:"session_max_seconds"`
				}
				if len(args) >0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.ScopeID == "" || p.Name == "" || p.DefaultPort == 0 {
					return nil, fmt.Errorf("scope_id, name, and default_port are required")
				}
				body := map[string]interface{}{
					"scope_id":   p.ScopeID,
					"name":       p.Name,
					"type":       "ssh",
					"attributes": map[string]interface{}{
						"default_port": p.DefaultPort,
					},
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				if len(p.HostSourceIDs) > 0 {
					body["host_source_id"] = p.HostSourceIDs
				}
				if len(p.BrokeredCredentialSourceIDs) > 0 {
					body["brokered_credential_source_id"] = p.BrokeredCredentialSourceIDs
				}
				if len(p.InjectedAppCredentialSourceIDs) > 0 {
					body["injected_application_credential_source_id"] = p.InjectedAppCredentialSourceIDs
				}
				if p.SessionMaxSeconds > 0 {
					body["session_max_seconds"] = p.SessionMaxSeconds
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/targets", body)
				if err != nil {
					return nil, fmt.Errorf("create ssh target failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create ssh target failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),

		// ── Credential Stores ──
		Registration(
			"create_credential_store",
			`Create a static credential store in a scope. Static stores hold username/password and SSH key credentials directly in Boundary.`,
			ToolSchema(map[string]json.RawMessage{
				"scope_id":    Prop("string", "Scope ID (project) to create the store in.", nil),
				"name":        Prop("string", "Name for the credential store.", nil),
				"description": Prop("string", "Optional description.", nil),
			}, []string{"scope_id", "name"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					ScopeID     string `json:"scope_id"`
					Name        string `json:"name"`
					Description string `json:"description"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.ScopeID == "" || p.Name == "" {
					return nil, fmt.Errorf("scope_id and name are required")
				}
				body := map[string]interface{}{
					"scope_id": p.ScopeID,
					"name":     p.Name,
					"type":     "static",
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/credential-stores", body)
				if err != nil {
					return nil, fmt.Errorf("create credential store failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create credential store failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),

		// ── Credentials ──
		Registration(
			"create_username_password_credential",
			`Create a username/password credential in a static credential store. This can be brokered or injected into targets.`,
			ToolSchema(map[string]json.RawMessage{
				"credential_store_id": Prop("string", "Credential store ID.", nil),
				"name":                Prop("string", "Name for the credential.", nil),
				"username":           Prop("string", "Username for authentication.", nil),
				"password":           Prop("string", "Password for authentication.", nil),
				"description":        Prop("string", "Optional description.", nil),
			}, []string{"credential_store_id", "name", "username", "password"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					CredentialStoreID string `json:"credential_store_id"`
					Name              string `json:"name"`
					Username          string `json:"username"`
					Password          string `json:"password"`
					Description        string `json:"description"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.CredentialStoreID == "" || p.Name == "" || p.Username == "" || p.Password == "" {
					return nil, fmt.Errorf("credential_store_id, name, username, and password are required")
				}
				body := map[string]interface{}{
					"credential_store_id": p.CredentialStoreID,
					"name":                p.Name,
					"type":                "username_password",
					"attributes": map[string]interface{}{
						"username": p.Username,
						"password": p.Password,
					},
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/credentials", body)
				if err != nil {
					return nil, fmt.Errorf("create credential failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create credential failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),
		Registration(
			"create_ssh_private_key_credential",
			`Create an SSH private key credential in a static credential store. Can be brokered or injected for SSH targets.`,
			ToolSchema(map[string]json.RawMessage{
				"credential_store_id":     Prop("string", "Credential store ID.", nil),
				"name":                    Prop("string", "Name for the credential.", nil),
				"username":                Prop("string", "SSH username.", nil),
				"private_key":             Prop("string", "PEM-encoded SSH private key.", nil),
				"private_key_passphrase":  Prop("string", "Optional passphrase for the private key.", nil),
				"description":             Prop("string", "Optional description.", nil),
			}, []string{"credential_store_id", "name", "username", "private_key"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					CredentialStoreID string `json:"credential_store_id"`
					Name              string `json:"name"`
					Username          string `json:"username"`
					PrivateKey        string `json:"private_key"`
					Passphrase         string `json:"private_key_passphrase"`
					Description        string `json:"description"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.CredentialStoreID == "" || p.Name == "" || p.Username == "" || p.PrivateKey == "" {
					return nil, fmt.Errorf("credential_store_id, name, username, and private_key are required")
				}
				attrs := map[string]interface{}{
					"username":    p.Username,
					"private_key": p.PrivateKey,
				}
				if p.Passphrase != "" {
					attrs["private_key_passphrase"] = p.Passphrase
				}
				body := map[string]interface{}{
					"credential_store_id": p.CredentialStoreID,
					"name":                p.Name,
					"type":                "ssh_private_key",
					"attributes":          attrs,
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/credentials", body)
				if err != nil {
					return nil, fmt.Errorf("create credential failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create credential failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),

		// ── Credential Libraries (for credential brokering/injection) ──
		Registration(
			"create_credential_library",
			`Create a credential library in a credential store. Credential libraries map stored credentials to a credential type that targets can broker or inject. For static stores, use the same name and credential_type as the credential.`,
			ToolSchema(map[string]json.RawMessage{
				"credential_store_id":     Prop("string", "Credential store ID.", nil),
				"name":                    Prop("string", "Name for the credential library.", nil),
				"credential_type":         Prop("string", "Credential type: 'username_password' or 'ssh_private_key'.", nil),
				"description":             Prop("string", "Optional description.", nil),
			}, []string{"credential_store_id", "name", "credential_type"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					CredentialStoreID string `json:"credential_store_id"`
					Name              string `json:"name"`
					CredentialType    string `json:"credential_type"`
					Description       string `json:"description"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.CredentialStoreID == "" || p.Name == "" || p.CredentialType == "" {
					return nil, fmt.Errorf("credential_store_id, name, and credential_type are required")
				}
				body := map[string]interface{}{
					"credential_store_id": p.CredentialStoreID,
					"name":                p.Name,
					"type":                "static",
					"credential_type":     p.CredentialType,
				}
				if p.Description != "" {
					body["description"] = p.Description
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/credential-libraries", body)
				if err != nil {
					return nil, fmt.Errorf("create credential library failed: %w", err)
				}
				if status != 201 && status != 200 {
					return nil, fmt.Errorf("create credential library failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),

		// ── Update Target (add host sources / credential sources) ──
		Registration(
			"update_target",
			`Update an existing target. Can add or change host source IDs, brokered credential source IDs, injected credential source IDs, name, description, or session limits. Pass only the fields you want to change. Version is auto-incremented by the API.`,
			ToolSchema(map[string]json.RawMessage{
				"id":                                  Prop("string", "Target ID to update.", nil),
				"host_source_ids":                     ArrayProp("string", "Host set IDs to associate as host sources."),
				"brokered_credential_source_ids":      ArrayProp("string", "Credential source IDs to broker."),
				"injected_application_credential_source_ids": ArrayProp("string", "Credential source IDs to inject."),
				"version":                             Prop("integer", "Current version for optimistic concurrency. If omitted, the server uses the latest.", nil),
			}, []string{"id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var p struct {
					ID                          string   `json:"id"`
					HostSourceIDs                []string `json:"host_source_ids"`
					BrokeredCredentialSourceIDs  []string `json:"brokered_credential_source_ids"`
					InjectedAppCredentialSourceIDs []string `json:"injected_application_credential_source_ids"`
					Version                      int      `json:"version"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &p); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if p.ID == "" {
					return nil, fmt.Errorf("id is required")
				}
				body := map[string]interface{}{}
				if len(p.HostSourceIDs) > 0 {
					body["host_source_id"] = p.HostSourceIDs
				}
				if len(p.BrokeredCredentialSourceIDs) > 0 {
					body["brokered_credential_source_id"] = p.BrokeredCredentialSourceIDs
				}
				if len(p.InjectedAppCredentialSourceIDs) > 0 {
					body["injected_application_credential_source_id"] = p.InjectedAppCredentialSourceIDs
				}
				if p.Version > 0 {
					body["version"] = p.Version
				}
				resp, status, err := c.DoPost(context.Background(), "/v1/targets/"+p.ID, body)
				if err != nil {
					return nil, fmt.Errorf("update target failed: %w", err)
				}
				if status != 200 {
					return nil, fmt.Errorf("update target failed (HTTP %d): %s", status, string(resp))
				}
				return mcp.TextResult(string(resp)), nil
			},
		),
	}
}
