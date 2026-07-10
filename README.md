# boundary-mcp

An MCP (Model Context Protocol) server for HashiCorp Boundary. Exposes Boundary cluster operations as MCP tools, enabling AI assistants (VS Code, Claude Desktop, Cursor) to query and manage Boundary resources through natural language.

## Demo

https://github.com/learhy/boundary-mcp/releases/download/full-demo/boundary-mcp-full-demo.mp4

> **6-scenario demo** — A real AI agent (glm-5.2:cloud) uses boundary-mcp through Claude Code. No simulation: every tool call is real, including a live 403 PermissionDenied, the agent's diagnosis of the missing grant, and actual connections through the Boundary worker to SSH, PostgreSQL, and web targets.
>
> 1. **Discover and connect** — check_connection, list_scopes, list_targets, read_target, authorize_session
> 2. **Troubleshoot access** — "How can I access the postgres target?" → agent traces the permission chain, finds the missing grant
> 3. **Natural language to bexpr** — "targets not on port 5432 or 22" → agent constructs the filter
> 4. **Session lifecycle** — authorize → list → read → cancel → confirm gone
> 5. **Real 403** — agent hits PermissionDenied, reads the role, explains the fix
> 6. **Connect through Boundary** — agent authorizes sessions and actually connects to SSH (hostname), PostgreSQL (SQL query), and web (curl /health) through the Boundary worker proxy
>
> [Download the demo video](https://github.com/learhy/boundary-mcp/releases/download/full-demo/boundary-mcp-full-demo.mp4) (2.5 MB, 129s)

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

## Getting Your Boundary Token

The MCP server requires a pre-authenticated Boundary token (`BOUNDARY_TOKEN`). Tokens expire, so you need to retrieve one and keep it current. Here are the methods:

### Option A: Boundary CLI (password auth)

If your Boundary cluster uses password authentication:

```bash
# Authenticate and get a token
BOUNDARY_ADDR=https://boundary.example.com:9200 \
  boundary authenticate password \
  -auth-method-id ampw_1234567890 \
  -login-name your.username \
  -password env://BOUNDARY_PASSWORD \
  -keyring-type none

# The output includes a line like:
#   The token is: at_abc123...
#
# Copy the token value (starts with "at_") and set it as BOUNDARY_TOKEN
export BOUNDARY_TOKEN=at_abc123...
```

### Option B: API call (no CLI needed)

```bash
# Authenticate via the REST API
curl -s -X POST \
  https://boundary.example.com:9200/v1/auth-methods/ampw_1234567890:authenticate \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"login_name":"your.username","password":"yourpassword"}}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['attributes']['token'])"

# Output: at_abc123...
# Set it as your token:
export BOUNDARY_TOKEN=at_abc123...
```

### Option C: Use an existing token from your environment

If you're already authenticated to Boundary (e.g., via the CLI or a previous session):

```bash
# Check if you have a token in the environment
echo $BOUNDARY_TOKEN

# Or read it from the Boundary CLI cache (if keyring is enabled)
boundary config get token 2>/dev/null
```

### Token expiration

User tokens typically expire after 7 days (configurable by your Boundary admin). App tokens (`apt_...`) can have custom expiration times. When the token expires:

1. Tool calls will return: `"Authentication failed. Please configure a valid BOUNDARY_TOKEN."`
2. Re-authenticate using one of the methods above
3. Update the `BOUNDARY_TOKEN` value in your MCP client config (see below)
4. Restart the MCP server (close and reopen the chat window, or reload the MCP config)

### Keeping your token fresh in VS Code

In `.vscode/mcp.json`, update the `BOUNDARY_TOKEN` value and reload the window:

```json
{
  "servers": {
    "boundary": {
      "command": "boundary-mcp",
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_NEW_TOKEN_HERE"
      }
    }
  }
}
```

Then: `Ctrl+Shift+P` -> "Developer: Reload Window"

### Keeping your token fresh in Claude Desktop

In `claude_desktop_config.json` (macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "boundary": {
      "command": "boundary-mcp",
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_NEW_TOKEN_HERE"
      }
    }
  }
}
```

Then: quit and restart Claude Desktop.

### Keeping your token fresh in Cursor

In `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "boundary": {
      "command": "boundary-mcp",
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_NEW_TOKEN_HERE"
      }
    }
  }
}
```

Then: restart Cursor or reload the MCP server from settings.

### Auto-refresh script

For convenience, save this as `refresh-boundary-token.sh` and run it when your token expires:

```bash
#!/bin/bash
# Refreshes the Boundary token and prints the updated config snippet

BOUNDARY_ADDR=${BOUNDARY_ADDR:-"https://boundary.example.com:9200"}
AUTH_METHOD_ID=${AUTH_METHOD_ID:-"ampw_1234567890"}

echo "Enter your Boundary username:"
read -r USERNAME
echo "Enter your Boundary password:"
read -rs PASSWORD
echo ""

TOKEN=$(curl -s -X POST \
  "$BOUNDARY_ADDR/v1/auth-methods/$AUTH_METHOD_ID:authenticate" \
  -H "Content-Type: application/json" \
  -d "{\"attributes\":{\"login_name\":\"$USERNAME\",\"password\":\"$PASSWORD\"}}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['attributes']['token'])" 2>/dev/null)

if [ -z "$TOKEN" ]; then
  echo "ERROR: Failed to authenticate"
  exit 1
fi

echo "New token: ${TOKEN:0:15}..."
echo ""
echo "Add this to your MCP config:"
echo "  \"BOUNDARY_TOKEN\": \"$TOKEN\""
echo ""
echo "Or export it:"
echo "  export BOUNDARY_TOKEN=$TOKEN"
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