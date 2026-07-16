// Package tools registry: aggregates all tool groups for registration.
package tools

import (
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/config"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// AllTools returns all tool registrations (Phase 1 read tools + Phase 2 write/connect tools).
func AllTools(c *bclient.Client, cfg *config.Config, server *mcp.Server, pm *ProxyManager) []*mcp.ToolRegistration {
	var all []*mcp.ToolRegistration
	all = append(all, ScopeTools(c)...)          // 2: list_scopes, read_scope
	all = append(all, TargetTools(c)...)         // 2: list_targets, read_target
	all = append(all, HostTools(c)...)            // 6: host catalogs, sets, hosts (list+read each)
	all = append(all, WorkerTools(c)...)          // 2: list_workers, read_worker
	all = append(all, SessionTools(c)...)        // 3: list_sessions, read_session, cancel_session
	all = append(all, SessionProxyTools(c, pm)...) // 3: authorize_session, start_proxy, close_proxy
	all = append(all, UserGroupRoleTools(c)...)   // 6: users, groups, roles (list+read each)
	all = append(all, AuthTools(c)...)           // 3: auth methods (list+read), auth tokens (list)
	all = append(all, CredentialTools(c)...)     // 4: credential stores (list+read), credential libraries (list+read)
	all = append(all, RecordingTools(c)...)      // 2: session recordings (list+read)
	all = append(all, ServerInfoTools(c, cfg, server)...) // 2: check_connection, server_info
	// Phase 2: write tools (create targets, hosts, credentials, etc.)
	all = append(all, WriteTools(c)...)           // 10: create/update for targets, hosts, credentials
	// Phase 2: connect tools (run commands on targets via boundary CLI)
	all = append(all, ConnectTools(c)...)        // 3: connect_ssh, connect_tcp, connect_ssh_interactive
	return all
}