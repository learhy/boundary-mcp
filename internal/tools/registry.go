// Package tools registry: aggregates all tool groups for registration.
package tools

import (
	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/config"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// AllTools returns all Phase 1 tool registrations (35 tools total).
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
	return all
}