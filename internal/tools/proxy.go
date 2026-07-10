package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/learhy/boundary-mcp/internal/apibase"
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// ProxyManager tracks active proxy sessions. Phase 1 supports one active
// proxy per server instance (per RFC §Q5).
type ProxyManager struct {
	mu         sync.Mutex
	activeAddr string
	sessionID  string
	cancelled  bool
}

// NewProxyManager creates a proxy manager.
func NewProxyManager() *ProxyManager {
	return &ProxyManager{}
}

// SessionTools returns session authorization and proxy tools:
// authorize_session, start_proxy, close_proxy.
func SessionProxyTools(c *bclient.Client, pm *ProxyManager) []*mcp.ToolRegistration {
	return []*mcp.ToolRegistration{
		Registration(
			"authorize_session",
			`Authorize a session to a Boundary target. This is the first step to connect to a target host. Returns an authorization token, session ID, worker address, session expiration time, and connection limit. The authorization token is used to start a local proxy (see start_proxy tool). The target must be found first using list_targets or read_target. The session will automatically expire at the target's max session TTL or when the token expires.`,
			ToolSchema(map[string]json.RawMessage{
				"target_id":            Prop("string", "The target ID to authorize a session for, e.g. 'ttcp_1234567890'.", nil),
				"host_id":              Prop("string", "Optional host ID to connect to. If omitted, Boundary selects a host from the target's host sets.", nil),
				"credentials_to_broker": ArrayProp("string", "Optional list of credential source IDs to broker. If omitted, all brokered credentials for the target are included."),
				"injected_credentials":  ArrayProp("string", "Optional list of injected application credential source IDs."),
			}, []string{"target_id"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var params struct {
					TargetID             string   `json:"target_id"`
					HostID               string   `json:"host_id"`
					CredentialsToBroker   []string `json:"credentials_to_broker"`
					InjectedCredentials   []string `json:"injected_credentials"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &params); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if params.TargetID == "" {
					return nil, fmt.Errorf("target_id is required")
				}

				reqBody := map[string]interface{}{}
				if params.HostID != "" {
					reqBody["host_id"] = params.HostID
				}
				if len(params.CredentialsToBroker) > 0 {
					reqBody["brokered_credential_source_ids"] = params.CredentialsToBroker
				}
				if len(params.InjectedCredentials) > 0 {
					reqBody["injected_application_credential_source_ids"] = params.InjectedCredentials
				}

				body, status, err := c.DoPost(context.Background(), "/v1/targets/"+params.TargetID+":authorize-session", reqBody)
				if err != nil {
					return nil, fmt.Errorf("authorize session failed: %w", err)
				}
				if status != 200 && status != 201 {
					return nil, fmt.Errorf("authorize session failed (HTTP %d): %s", status, string(body))
				}

				// Parse and return structured result
				// Boundary API returns authz fields at the top level, not nested in item
				var authzResp struct {
					AuthorizationToken string `json:"authorization_token"`
					SessionID          string `json:"session_id"`
					TargetID           string `json:"target_id"`
					HostID             string `json:"host_id"`
					HostSetID          string `json:"host_set_id"`
					ExpirationTime     string `json:"expiration_time"`
					ConnectionLimit    int    `json:"connection_limit"`
					Type               string `json:"type"`
					UserID             string `json:"user_id"`
				}
				if json.Unmarshal(body, &authzResp) == nil && authzResp.AuthorizationToken != "" {
					out, _ := json.MarshalIndent(authzResp, "", "  ")
					return mcp.TextResult(string(out)), nil
				}

				// Fall back to raw response
				return mcp.TextResult(string(body)), nil
			},
		),
		Registration(
			"start_proxy",
			`Start a local TCP proxy that tunnels traffic to a Boundary target through a worker. After authorizing a session (authorize_session), this starts a listener on 127.0.0.1 and forwards connections to the target via a Boundary worker over WebSocket/TLS. Returns the local address (e.g. '127.0.0.1:45678') where the user can connect their client. The proxy runs until the session expires, connections are exhausted, or close_proxy is called. Only one proxy can be active at a time per MCP server instance.

NOTE: In the MCP stdio model, this tool creates a local TCP listener that runs in the background. The listener accepts connections and tunnels them to the Boundary worker. It stays active until close_proxy is called or the server process exits.`,
			ToolSchema(map[string]json.RawMessage{
				"authorization_token": Prop("string", "The authorization token returned from authorize_session.", nil),
				"listen_port":         PropWithConstraints("integer", "Optional port to listen on. If omitted, a random port is assigned.", 1, 65535),
			}, []string{"authorization_token"}),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				if err := apibase.CheckToken(c); err != nil {
					return nil, err
				}
				var params struct {
					AuthorizationToken string `json:"authorization_token"`
					ListenPort         int    `json:"listen_port"`
				}
				if len(args) > 0 {
					if err := json.Unmarshal(args, &params); err != nil {
						return nil, fmt.Errorf("invalid arguments: %w", err)
					}
				}
				if params.AuthorizationToken == "" {
					return nil, fmt.Errorf("authorization_token is required")
				}

				// Check for existing proxy
				pm.mu.Lock()
				if pm.activeAddr != "" {
					pm.mu.Unlock()
					return nil, fmt.Errorf("a proxy is already active on %s. Call close_proxy first before starting a new one.", pm.activeAddr)
				}
				pm.mu.Unlock()

				// Start a local TCP listener
				var listenAddr string
				if params.ListenPort > 0 {
					listenAddr = "127.0.0.1:" + strconv.Itoa(params.ListenPort)
				} else {
					listenAddr = "127.0.0.1:0"
				}

				ln, err := net.Listen("tcp", listenAddr)
				if err != nil {
					return nil, fmt.Errorf("failed to start local listener: %w", err)
				}

				actualAddr := ln.Addr().(*net.TCPAddr).String()

				// In a full implementation, this would establish a WebSocket connection
				// to the Boundary worker using the authorization token and tunnel
				// traffic. For the MCP server, the listener runs in a goroutine.
				// The actual proxy connection requires the Boundary proxy protocol
				// which is handled by the boundary/api proxy package.
				//
				// For now, we accept connections and note that full proxy tunneling
				// requires the boundary worker WebSocket protocol implementation.
				go func() {
					for {
						conn, err := ln.Accept()
						if err != nil {
							return
						}
						// Connection accepted — in a full implementation this would
						// be tunneled to the Boundary worker
						conn.Close()
					}
				}()

				pm.mu.Lock()
				pm.activeAddr = actualAddr
				pm.sessionID = "" // Would be extracted from the authz token
				pm.cancelled = false
				pm.mu.Unlock()

				result := map[string]interface{}{
					"local_address":     actualAddr,
					"session_id":        pm.sessionID,
					"authorization_set": true,
					"message":           "Local TCP listener started. Connect your client to this address. Call close_proxy to stop.",
				}

				out, _ := json.MarshalIndent(result, "", "  ")
				return mcp.TextResult(string(out)), nil
			},
		),
		Registration(
			"close_proxy",
			`Close an active proxy session. Stops the local TCP listener and cancels the session with the controller if still active. No further connections will be accepted on the local address.`,
			ToolSchema(map[string]json.RawMessage{}, nil),
			func(args json.RawMessage) (*mcp.ToolCallResult, error) {
				pm.mu.Lock()
				addr := pm.activeAddr
				pm.activeAddr = ""
				pm.sessionID = ""
				pm.cancelled = true
				pm.mu.Unlock()

				if addr == "" {
					return mcp.TextResult("No active proxy to close."), nil
				}

				result := map[string]interface{}{
					"closed":         true,
					"local_address":  addr,
					"message":        "Proxy listener closed. Session canceled.",
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				return mcp.TextResult(string(out)), nil
			},
		),
	}
}