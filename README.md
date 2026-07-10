# boundary-mcp

An MCP (Model Context Protocol) server for HashiCorp Boundary. Exposes Boundary cluster operations as MCP tools, enabling AI assistants (VS Code, Claude Desktop, Cursor) to query and manage Boundary resources through natural language.

## Overview

`boundary-mcp` is a standalone Go binary that bridges an MCP client and a Boundary controller's REST API (`/v1/`). It runs as a child process of the MCP client (stdio transport) and maintains a long-lived connection to one Boundary controller.

### Phase 1 (Beta) Features

- **35 tools** covering read/list operations across all Boundary resource types
- Session authorization and proxy management
- bexpr filter support for all list operations
- Client-directed pagination
- Pre-authenticated token auth
- Single cluster, local execution
- Docker image

### Supported Resource Types

Scopes, Targets, Host Catalogs, Host Sets, Hosts, Workers, Sessions, Users, Groups, Roles, Auth Methods, Auth Tokens, Credential Stores, Credential Libraries, Session Recordings.

## Quick Start

### Build

```bash
go build -o boundary-mcp ./cmd/boundary-mcp/
```

### Run (stdio mode)

```bash
BOUNDARY_ADDR=https://boundary.example.com:9200 \
BOUNDARY_TOKEN=at_1234567890_... \
./boundary-mcp
```

### Docker

```bash
docker run -i --rm \
  -e BOUNDARY_ADDR=https://boundary.example.com:9200 \
  -e BOUNDARY_TOKEN=at_1234567890_... \
  boundary-mcp:latest
```

## Configuration

All configuration is via environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `BOUNDARY_ADDR` | Yes | Controller address, e.g. `https://boundary.example.com` |
| `BOUNDARY_TOKEN` | Yes | Pre-authenticated app or user token |
| `BOUNDARY_CACERT` | No | Path to CA cert PEM file |
| `BOUNDARY_CAPATH` | No | Path to directory of CA cert PEM files |
| `BOUNDARY_TLS_INSECURE` | No | Skip TLS verification (dev only) |
| `BOUNDARY_CLIENT_TIMEOUT` | No | HTTP client timeout in seconds (default: 60) |
| `BOUNDARY_MAX_RETRIES` | No | Max retries on 5xx (default: 2) |

## Client Integration

### VS Code (`.vscode/mcp.json`)

```json
{
  "servers": {
    "boundary": {
      "command": "boundary-mcp",
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_1234567890_..."
      }
    }
  }
}
```

### Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "boundary": {
      "command": "boundary-mcp",
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_1234567890_..."
      }
    }
  }
}
```

### Cursor

```json
{
  "mcpServers": {
    "boundary": {
      "command": "boundary-mcp",
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_1234567890_..."
      }
    }
  }
}
```

## Tools (35)

| # | Tool | Description |
|---|------|-------------|
| 1 | `check_connection` | Verify controller connectivity and token validity |
| 2 | `list_scopes` | List orgs or projects within a scope |
| 3 | `read_scope` | Read scope details |
| 4 | `list_targets` | List targets within a scope |
| 5 | `read_target` | Read target details |
| 6 | `list_host_catalogs` | List host catalogs |
| 7 | `read_host_catalog` | Read host catalog details |
| 8 | `list_host_sets` | List host sets |
| 9 | `read_host_set` | Read host set details |
| 10 | `list_hosts` | List hosts |
| 11 | `read_host` | Read host details |
| 12 | `list_sessions` | List sessions |
| 13 | `read_session` | Read session details |
| 14 | `cancel_session` | Cancel an active session |
| 15 | `list_workers` | List workers |
| 16 | `read_worker` | Read worker details |
| 17 | `list_users` | List users |
| 18 | `read_user` | Read user details |
| 19 | `list_groups` | List groups |
| 20 | `read_group` | Read group details |
| 21 | `list_roles` | List roles |
| 22 | `read_role` | Read role details |
| 23 | `list_auth_methods` | List auth methods |
| 24 | `read_auth_method` | Read auth method details |
| 25 | `list_auth_tokens` | List auth tokens |
| 26 | `list_credential_stores` | List credential stores |
| 27 | `read_credential_store` | Read credential store details |
| 28 | `list_credential_libraries` | List credential libraries |
| 29 | `read_credential_library` | Read credential library details |
| 30 | `list_session_recordings` | List session recordings |
| 31 | `read_session_recording` | Read session recording metadata |
| 32 | `authorize_session` | Authorize a session to a target |
| 33 | `start_proxy` | Start a local TCP proxy to a target |
| 34 | `close_proxy` | Close an active proxy |
| 35 | `server_info` | Server status and tool call counts |

## Example Prompts

- "List all SSH targets with credential injection"
- "Show me all active sessions for user john.doe"
- "Why can't User A see target A? Check their roles and grants."
- "Cancel all active sessions for target ttcp_12345"
- "Authorize a session to the postgres-prod target"
- "List all workers that are offline"

## Security

- Token is required for all operations. Without `BOUNDARY_TOKEN`, tool calls return an authentication error.
- Token is never logged in full. Log messages use a truncated form (`at_12...7890`).
- TLS is enforced when `BOUNDARY_ADDR` uses `https://`. Insecure mode is supported for development but logs a warning.
- The proxy listener binds to `127.0.0.1` only — no remote access.
- The server runs locally as a child process. The token never leaves the user's machine.

## Development

```bash
# Build
go build -o boundary-mcp ./cmd/boundary-mcp/

# Vet
go vet ./...

# Test
go test ./...
```

## License

MPL-2.0 (consistent with HashiCorp Boundary)