# RFC: Boundary MCP Server

**Document:** RFC for [PRD ICU-060] Boundary MCP Server  
**Author:** Principal Engineering  
**Status:** Draft for Review  
**Date:** 2026-07-09  
**PRD Owner:** krishnan.ramachandran@ibm.com  

---

## 1. Overview

This RFC defines the technical architecture for a HashiCorp Boundary Model Context Protocol (MCP) Server — a standalone Go binary that exposes Boundary cluster operations as MCP tools, enabling AI assistants (VS Code, Claude Desktop, Cursor) to query and manage Boundary resources through natural language.

The server acts as a bridge between an MCP client and a Boundary controller's REST API (`/v1/`). It uses the existing `github.com/hashicorp/boundary/api` Go client library for all API interactions, ensuring protocol fidelity and reducing maintenance surface.

### Scope

| Phase | PRD Requirements | This RFC Covers |
|-------|-----------------|----------------|
| Phase 1 (Beta) | Read/list + filtering, authorize session, token auth, single cluster, local execution, Docker image, one-click installs | **Full design** |
| Phase 2 | Create/write/delete, connect to targets, OIDC auth, password auth | **Architecture outline + open questions** |
| Phase 3 | Full CRUD (host catalogs, credential stores, roles, managed groups, user provisioning) | **Architecture outline only** |

### Non-Goals (Beta)

- Multiple cluster support (PRD: optional, not required)
- Write/create/delete operations (Phase 2+)
- OIDC/password authentication (Phase 2+)
- Server-side aggregation, vector search (PRD: explicitly excluded)
- Performance telemetry (CPU/memory) — PRD lists as consideration only
- Partner marketplace deployment (post-beta)

---

## 2. Architecture

### 2.1 High-Level Design

```
┌─────────────────────────────────────────────────────────┐
│                   MCP Client                            │
│  (VS Code / Claude Desktop / Cursor)                   │
└──────────────────────┬──────────────────────────────────┘
                       │ MCP Protocol (stdio)
                       │ JSON-RPC 2.0
┌──────────────────────▼──────────────────────────────────┐
│              Boundary MCP Server                         │
│  (standalone Go binary)                                 │
│                                                         │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ MCP Server  │  │ Tool Registry│  │ Session Mgr   │  │
│  │ (JSON-RPC)  │  │ (tool defs)  │  │ (proxy lifecycle)│  │
│  └──────┬──────┘  └──────┬───────┘  └───────┬───────┘  │
│         │                │                  │           │
│  ┌──────▼────────────────▼──────────────────▼───────┐  │
│  │            Boundary API Client Layer             │  │
│  │  (github.com/hashicorp/boundary/api/*)            │  │
│  │  targets, sessions, hosts, workers, scopes,       │  │
│  │  roles, users, groups, authmethods, authtokens,    │  │
│  │  hostcatalogs, hostsets, credentialstores,         │  │
│  │  credentiallibraries, sessionrecordings            │  │
│  └──────────────────────┬───────────────────────────┘  │
│                         │ HTTPS                         │
└─────────────────────────┼───────────────────────────────┘
                          │
                ┌─────────▼─────────┐
                │  Boundary         │
                │  Controller       │
                │  (REST /v1/)      │
                └───────────────────┘
```

### 2.2 Process Model

The MCP server is a single-process, single-cluster, local-execution binary. It runs as a child process of the MCP client (stdio transport) and maintains a long-lived connection to one Boundary controller.

**Lifecycle:**
1. MCP client spawns the binary with configuration (env vars or config file)
2. Server initializes: validates config, creates `api.Client` with token, performs a liveness check against the controller
3. Server registers tools and begins serving MCP requests over stdin/stdout
4. On client disconnect (stdin closed), server gracefully shuts down: cancels any active session proxies, exits

### 2.3 Configuration

Configuration is supplied via environment variables (the standard MCP client pattern) with a JSON config file fallback:

```jsonc
// Example: VS Code .vscode/mcp.json
{
  "servers": {
    "boundary": {
      "command": "boundary-mcp",
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_1234567890_...",
        "BOUNDARY_CACERT": "/path/to/ca.pem"
      }
    }
  }
}
```

**Environment variables (Phase 1):**

| Variable | Required | Description |
|----------|----------|-------------|
| `BOUNDARY_ADDR` | Yes | Controller address, e.g. `https://boundary.example.com` |
| `BOUNDARY_TOKEN` | Yes | Pre-authenticated app or user token |
| `BOUNDARY_CACERT` | No | Path to CA cert PEM file |
| `BOUNDARY_CAPATH` | No | Path to directory of CA cert PEM files |
| `BOUNDARY_TLS_INSECURE` | No | Skip TLS verification (dev only) |
| `BOUNDARY_CLIENT_TIMEOUT` | No | HTTP client timeout (default: 60s) |
| `BOUNDARY_MAX_RETRIES` | No | Max retries on 5xx (default: 2) |

These map directly to the existing `api.Config.ReadEnvironment()` constants (`EnvBoundaryAddr`, `EnvBoundaryToken`, etc. — see `api/client.go:36-47`). The MCP server delegates to `api.DefaultConfig()` which already reads all of these.

**Token prompt behavior:** If `BOUNDARY_TOKEN` is not set, the server emits an MCP tool result with an error message prompting the user to configure their token. The server does not block on startup waiting for input — it starts and returns a clear error when a tool is invoked without a valid token.

---

## 3. MCP Protocol Implementation

### 3.1 Transport

Phase 1 uses **stdio transport only** (JSON-RPC 2.0 over stdin/stdout). This is the standard for local MCP execution and is supported by VS Code, Claude Desktop, and Cursor.

SSE transport is a Phase 2 concern (the PRD references `"transport_mode": "sse"` in the OIDC config example, which is Phase 2).

### 3.2 MCP Capabilities

The server advertises:

| Capability | Phase 1 | Justification |
|-----------|---------|---------------|
| `tools` | Yes | Primary interface — all Boundary operations are tools |
| `resources` | No | Boundary resources are dynamic and scoped; tools with parameters are more appropriate than static resource URIs |
| `prompts` | No | Not needed for beta; the LLM client provides the natural language interface |
| `logging` | Yes | Debug/diagnostic logging to stderr |

### 3.3 Error Handling

MCP tool errors map from Boundary API errors:

| Boundary API Response | MCP Error Response |
|----------------------|-------------------|
| 401 (invalid/missing token) | Tool error: "Authentication failed. Please configure a valid BOUNDARY_TOKEN. Current token is missing or expired." |
| 403 (insufficient permissions) | Tool error: "Your token does not have permission to perform this operation: [action]. Required grant: [grant details from response]." |
| 404 (not found) | Tool error: "Resource not found: [resource_type] with ID [id] does not exist or you do not have access to it." |
| 400 (bad input) | Tool error: "Invalid request: [message from API response]." |
| 429 (rate limited) | Tool error with Retry-After info: "Rate limited. Retry after [N] seconds." |
| 5xx (server error) | Tool error: "Boundary controller error: [message]. The server may be experiencing issues." |

The `api.Response.Decode()` method already distinguishes between API-level errors (`*api.ServerError`) and transport errors. The server translates these into human-readable MCP tool results.

---

## 4. Tool Catalog (Phase 1)

### 4.1 Design Principles

1. **One tool per API operation** — Each Boundary API endpoint maps to one MCP tool. This gives the LLM maximum composability. The LLM decides which tools to call based on the user's natural language query.

2. **Rich schema descriptions** — Tool descriptions include what the tool does, what parameters it accepts, and example usage. The LLM relies on these to select the right tool. Descriptions are the primary UX surface.

3. **Structured output, not prose** — Tools return JSON-structured results. The LLM formats the response for the user. This prevents the server from making presentation decisions that the LLM is better suited for.

4. **Token permission awareness** — Every tool result includes the `authorized_actions` array from the API response when available, so the LLM can tell the user what they're allowed to do.

### 4.2 Tool Inventory

#### 4.2.1 Scope Operations (Organizations and Projects)

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_scopes` | `scopes.Client.List()` | List scopes (orgs or projects) within a parent scope. Supports `recursive`, `filter`, `page_size`. |
| `read_scope` | `scopes.Client.Read()` | Read details of a specific scope by ID. |

**`list_scopes` schema:**
```json
{
  "name": "list_scopes",
  "description": "List Boundary scopes (organizations or projects) within a parent scope. A scope is a container for resources — the global scope contains orgs, orgs contain projects. Use recursive=true to list all descendant scopes. Supports bexpr filtering (e.g. 'name == \"my-org\"' or 'type == \"project\"'). Returns paginated results.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "scope_id": {
        "type": "string",
        "description": "The scope ID to list children of. Use 'global' for the global scope. Required.",
        "default": "global"
      },
      "recursive": {
        "type": "boolean",
        "description": "If true, recursively list all descendant scopes. Default: false.",
        "default": false
      },
      "filter": {
        "type": "string",
        "description": "bexpr filter expression. Examples: 'name == \"production\"', 'type == \"project\"'. See: https://developer.hashicorp.com/boundary/docs/filtering-and-listing-resources"
      },
      "page_size": {
        "type": "integer",
        "description": "Page size for client-directed pagination. If omitted, all results are fetched automatically. Use this for large result sets.",
        "minimum": 1,
        "maximum": 1000
      }
    },
    "required": ["scope_id"]
  }
}
```

**Output structure:**
```json
{
  "items": [
    {
      "id": "o_1234567890",
      "name": "production-org",
      "description": "Production organization",
      "type": "org",
      "scope_id": "global",
      "created_time": "2025-01-15T10:30:00Z",
      "updated_time": "2025-06-20T14:22:00Z",
      "authorized_actions": ["read", "list", "create", "update", "delete"],
      "primary_auth_method_id": "amoidc_12345"
    }
  ],
  "est_item_count": 12,
  "response_type": "complete",
  "list_token": null,
  "removed_ids": []
}
```

#### 4.2.2 Target Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_targets` | `targets.Client.List()` | List targets within a scope. Supports recursive, filter, pagination. |
| `read_target` | `targets.Client.Read()` | Read full details of a target by ID, including credential sources, host sources, and authorized actions. |

**Key design decision — `list_targets` includes credential injection info:** The PRD's example prompts ask "Give me the target list of all my SSH targets configured with credential injection created between <date1> and <date2>." This requires the `list_targets` tool to expose:
- `type` (tcp, ssh, rdp) — for filtering by target type
- `brokered_credential_source_ids` and `injected_application_credential_source_ids` — for "configured with credential injection"
- `created_time` — for date range filtering
- `worker_filter`, `egress_worker_filter` — for "no egress filters setup"
- `session_max_seconds`, `session_connection_limit` — for session-related queries

The bexpr filter can handle these: `'"type" == "ssh" && "created_time" > "2025-01-01T00:00:00Z"'`. However, bexpr's date comparison requires RFC3339 formatted timestamps. The tool description should document this.

**`list_targets` schema:**
```json
{
  "name": "list_targets",
  "description": "List Boundary targets within a scope. Targets define how users connect to hosts — they specify the target type (tcp, ssh, rdp), host sources, credential sources, session limits, and worker filters. Supports recursive listing across child scopes. Common filters: type ('\"type\" == \"ssh\"'), credential injection ('len(brokered_credential_source_ids) > 0'), created time ('\"created_time\" > \"2025-06-01T00:00:00Z\"'), egress filter presence ('egress_worker_filter == \"\"'). Use page_size for large result sets; if results are truncated, a list_token is returned for fetching the next page.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "scope_id": { "type": "string", "description": "Scope ID (org or project). Required." },
      "recursive": { "type": "boolean", "description": "Recursively list targets in all child scopes. Default: false." },
      "filter": { "type": "string", "description": "bexpr filter expression. Examples: '\"type\" == \"ssh\"', 'name matches \"web-.*\"', 'len(brokered_credential_source_ids) > 0'" },
      "page_size": { "type": "integer", "minimum": 1, "maximum": 1000, "description": "Page size. If omitted, all results are fetched. Use for large sets (>100 targets)." },
      "list_token": { "type": "string", "description": "List token from a previous paginated response to fetch the next page." }
    },
    "required": ["scope_id"]
  }
}
```

#### 4.2.3 Host Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_host_catalogs` | `hostcatalogs.Client.List()` | List host catalogs within a scope. |
| `read_host_catalog` | `hostcatalogs.Client.Read()` | Read host catalog details. |
| `list_host_sets` | `hostsets.Client.List()` | List host sets within a host catalog. |
| `read_host_set` | `hostsets.Client.Read()` | Read host set details, including host IDs. |
| `list_hosts` | `hosts.Client.List()` | List hosts within a host catalog. |
| `read_host` | `hosts.Client.Read()` | Read host details (IP addresses, DNS names, attributes). |

#### 4.2.4 Session Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_sessions` | `sessions.Client.List()` | List sessions within a scope. Supports filtering by user, target, status. |
| `read_session` | `sessions.Client.Read()` | Read session details including connections, state, certificate. |
| `cancel_session` | `sessions.Client.Cancel()` | Cancel an active session by ID. *(Note: technically a write operation, but PRD Phase 1 example prompts include "Cancel all the active target A sessions that is active for more than 5 hours" — see §4.3.)* |

#### 4.2.5 Worker Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_workers` | `workers.Client.List()` | List workers within a scope. Shows status (online/offline via `last_status_time`), address, tags, release version, active connection count. |
| `read_worker` | `workers.Client.Read()` | Read worker details including tags, storage state, downstream workers. |

#### 4.2.6 User and Group Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_users` | `users.Client.List()` | List users within a scope. |
| `read_user` | `users.Client.Read()` | Read user details including accounts, login name, email. |
| `list_groups` | `groups.Client.List()` | List groups within a scope. |
| `read_group` | `groups.Client.Read()` | Read group details including member IDs. |

#### 4.2.7 Role and Permission Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_roles` | `roles.Client.List()` | List roles within a scope. |
| `read_role` | `roles.Client.Read()` | Read role details including principals, grants, grant scope IDs. |

These tools directly support the PRD's permission-related example prompts:
- "Why User A cannot see target A" — LLM calls `read_user` to get user's groups, then `list_roles` + `read_role` to check grants
- "Show me the users who are allowed to access the target A" — LLM calls `list_roles` with filter, then `read_role` for principals
- "Does the role 'intern' have any grants that allow session access to DB-SSH-PROD" — LLM calls `list_roles` with filter `name == "intern"`, then `read_role` to inspect grants

#### 4.2.8 Auth Method and Auth Token Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_auth_methods` | `authmethods.Client.List()` | List auth methods within a scope. |
| `read_auth_method` | `authmethods.Client.Read()` | Read auth method details (type, attributes). |
| `list_auth_tokens` | `authtokens.Client.List()` | List auth tokens within a scope. Shows user, expiration, last used. |

#### 4.2.9 Credential Store and Library Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_credential_stores` | `credentialstores.Client.List()` | List credential stores within a scope. |
| `read_credential_store` | `credentialstores.Client.Read()` | Read credential store details (type, attributes). |
| `list_credential_libraries` | `credentiallibraries.Client.List()` | List credential libraries within a credential store. |
| `read_credential_library` | `credentiallibraries.Client.Read()` | Read credential library details. |

#### 4.2.10 Session Recording Operations

| Tool | API Method | Description |
|------|-----------|-------------|
| `list_session_recordings` | `sessionrecordings.Client.List()` (via custom.go) | List session recordings. Supports filtering by session ID, user, target, time range. |
| `read_session_recording` | `sessionrecordings.Client.Read()` (via custom.go) | Read session recording metadata, states, and connection recordings. |

#### 4.2.11 Session Authorization (Connect to Target)

| Tool | API Method | Description |
|------|-----------|-------------|
| `authorize_session` | `targets.Client.AuthorizeSession()` | Authorize a session to a target. Returns authorization token, session ID, worker address, expiration, and connection limit. |
| `start_proxy` | `proxy.New()` + `proxy.ClientProxy.Start()` | Start a local TCP proxy on 127.0.0.1 that tunnels to the target through a Boundary worker. Returns the local listening address and session metadata. |
| `close_proxy` | `proxy.ClientProxy.CloseSession()` or context cancel | Close an active proxy session. Cancels the session with the controller if still active. |

**`authorize_session` schema:**
```json
{
  "name": "authorize_session",
  "description": "Authorize a session to a Boundary target. This is the first step to connect to a target host. Returns an authorization token, session ID, worker address, session expiration time, and connection limit. The authorization token is used to start a local proxy (see start_proxy tool). The target must be found first using list_targets or read_target. The session will automatically expire at the target's max session TTL or when the token expires.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "target_id": { "type": "string", "description": "The target ID to authorize a session for. e.g. 'ttcp_1234567890'." },
      "host_id": { "type": "string", "description": "Optional host ID to connect to. If omitted, Boundary selects a host from the target's host sets." },
      "credentials_to_broker": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Optional list of credential source IDs to broker. If omitted, all brokered credentials for the target are included."
      },
      "injected_credentials": {
        "type": "array",
        "items": { "type": "string" },
        "description": "Optional list of injected application credential source IDs."
      }
    },
    "required": ["target_id"]
  }
}
```

**`start_proxy` schema:**
```json
{
  "name": "start_proxy",
  "description": "Start a local TCP proxy that tunnels traffic to a Boundary target through a worker. After authorizing a session (authorize_session), this starts a listener on 127.0.0.1 and forwards connections to the target via a Boundary worker over WebSocket/TLS. Returns the local address (e.g. '127.0.0.1:45678') where the user can connect their client. The proxy runs until the session expires, connections are exhausted, or close_proxy is called. Only one proxy can be active at a time per MCP server instance.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "authorization_token": { "type": "string", "description": "The authorization token returned from authorize_session." },
      "listen_port": { "type": "integer", "description": "Optional port to listen on. If omitted, a random port is assigned." }
    },
    "required": ["authorization_token"]
  }
}
```

**`start_proxy` output:**
```json
{
  "local_address": "127.0.0.1:45678",
  "session_id": "s_1234567890",
  "target_id": "ttcp_1234567890",
  "target_address": "10.0.1.50:5432",
  "worker_address": "worker.example.com:9202",
  "session_expiration": "2026-07-09T16:30:00Z",
  "connections_left": -1,
  "close_reason": null
}
```

### 4.3 Session Cancel: Phase Boundary Question

The PRD Phase 1 example prompts include: "Cancel all the active target A sessions that is active for more than 5 hours." This is technically a write operation (POST `sessions/{id}:cancel`).

**Decision:** Include `cancel_session` in Phase 1. The PRD places this example under Phase 1's Requirement 2 (read and list operations), and the acceptance criteria for Phase 1 explicitly list "session authorize" as a supported operation. Cancel is the natural counterpart to authorize — if the MCP server can start sessions, it should be able to stop them. Without cancel, the PRD's own example prompt cannot be fulfilled.

This is flagged as an **architectural question** in §8 for PRD owner confirmation.

---

## 5. Pagination Strategy

### 5.1 The Problem

Boundary's list API uses cursor-based pagination. When the result set is large, the API returns:
- `response_type: "partial"` (vs `"complete"`)
- `list_token` (cursor for the next page)
- `est_item_count` (estimated total)

The existing Go API client (`api/targets/target.gen.go`, `api/sessions/session.gen.go`, etc.) has two modes:
1. **Auto-pagination** (default): The client automatically fetches all pages and returns the complete result set.
2. **Client-directed pagination** (`WithClientDirectedPagination(true)`): Returns only the first page, with the `list_token` for the caller to request subsequent pages via `ListNextPage()`.

### 5.2 MCP Strategy

The PRD specifies (page 11):
> Native MCP cursor-based pagination with user prompts and decisions to fetch the next set of records. e.g. If Targets configured >= 'N' (e.g. N=100) show a warning message...

**Implementation:**

1. **Default: client-directed pagination with page_size=100.** All `list_*` tools use `WithClientDirectedPagination(true)` and `WithPageSize(100)` by default. This prevents the server from fetching thousands of records and flooding the LLM context.

2. **Tool returns page metadata.** The output includes `est_item_count`, `response_type`, and `list_token`. If `response_type == "partial"`, the output includes a message: `"More results available. Call this tool again with list_token='<token>' to fetch the next 100 items, or increase page_size to fetch more at once."`

3. **User-controlled page_size.** The user (via the LLM) can set `page_size` to any value 1-1000. If they set a large page_size, the server respects it but the LLM context may be overwhelmed. The tool description warns about this.

4. **No interactive y/n prompts from the server.** The PRD describes an interactive y/n flow ("do you want to fetch next 100 targets y/n"). In the MCP model, the server cannot interactively prompt the user — it returns a tool result and the LLM decides what to do next. The LLM will present the choice to the user in natural language. This is the correct MCP pattern and aligns with the PRD's intent.

### 5.3 Filter-First Approach

When `est_item_count` is large, the tool result includes a suggestion:
```
"hint": "Large result set (est. 500 items). Consider adding a filter to narrow results, or call again with list_token to page through. Example filters: '\"type\" == \"ssh\"', '\"created_time\" > \"2025-06-01T00:00:00Z\"'."
```

This implements the PRD's "Filtering — User shall be prompted to provide additional context" behavior.

---

## 6. Filtering

### 6.1 bexpr Filter Syntax

Boundary uses `hashicorp/go-bexpr` for API filtering. The filter is passed as a query parameter (`?filter=...`). The syntax supports:

- Comparison: `"field" == "value"`, `"field" != "value"`, `"field" > 123`
- String matching: `"name" matches "web-.*"`, `"name" contains "prod"`
- Logical: `expr1 && expr2`, `expr1 || expr2`, `!(expr)`
- Nested fields: `"scope.id" == "o_12345"`
- Collection size: `len(host_source_ids) > 0`

The `WithFilter()` option in each API package (e.g. `targets.WithFilter()`) passes this directly as the `filter` query parameter.

### 6.2 MCP Exposure

Tools expose a `filter` parameter that accepts raw bexpr syntax. The tool descriptions include examples. The LLM is responsible for constructing valid filter expressions from the user's natural language.

**Rationale:** Abstracting bexpr behind higher-level parameters (e.g. separate `target_type`, `created_after`, `has_credentials` params) would:
- Require the server to understand every possible filter dimension
- Create a maintenance burden as new fields are added
- Lose the composability of bexpr (AND/OR/NOT)
- Still require the LLM to map natural language to parameters

Instead, the LLM maps natural language directly to bexpr. This is what the LLM is good at, and the bexpr syntax is simple enough to be documented in tool descriptions.

**Example mappings (natural language → bexpr):**

| Natural Language | bexpr Filter |
|-----------------|--------------|
| "SSH targets" | `"type" == "ssh"` |
| "created in last 24 hours" | `"created_time" > "2026-07-08T14:30:00Z"` |
| "with credential injection" | `len(injected_application_credential_source_ids) > 0` |
| "no egress filters" | `egress_worker_filter == ""` |
| "name contains 'web-session-recording-prod'" | `"name" contains "web-session-recording-prod"` |
| "active sessions for user john.doe" | `"user_id" == "u_12345" && "status" == "active"` |

### 6.3 Date/Time Handling

bexpr date comparisons require RFC3339 formatted timestamps as string comparisons. The tool descriptions document the expected format: `"2026-07-09T14:30:00Z"`.

For relative time expressions ("last 24 hours", "last 7 days"), the LLM must compute the absolute timestamp. This is straightforward for an LLM with knowledge of the current time (passed in the MCP context or computed at tool execution time).

**Enhancement:** The server injects `current_time` into every tool result, so the LLM always knows the current timestamp for relative date calculations:
```json
{
  "_meta": { "current_time": "2026-07-09T14:30:00Z" },
  "items": [...]
}
```

---

## 7. Security

### 7.1 Authentication Model (Phase 1)

Phase 1 uses pre-authenticated tokens only. The server does not perform authentication — it expects a valid `BOUNDARY_TOKEN` environment variable.

**Token types:**
- **App tokens** (`apt_...`): Preferred. Can be scoped to specific read/list/authorize-session capabilities.
- **User tokens** (`at_...`): Fallback if app tokens are not GA. Have the same permissions as the user.

The server calls `api.NewClient(config)` which reads `BOUNDARY_TOKEN` from the environment and sets it on the client. All subsequent API calls include this token in the `Authorization` header.

**Token validation on startup:** The server performs a lightweight validation call on startup — `scopes.Client.Read(ctx, "global")` — to verify the token is valid and the controller is reachable. If this fails, the server logs an error to stderr but still starts (the client may fix the token and retry).

### 7.2 Permission Enforcement

The PRD states: "Users can perform the operations as per the permission they have on the boundary cluster." and "The user shall be able to perform only the operations as allowed by the permissions on the token."

The server does not enforce permissions itself — Boundary's API does this. If the token lacks permission for an operation, the API returns 403, and the server translates this into a clear error message (see §3.3).

**Even if the user accidentally uses a token with write/update/delete scope, the user shall be able to perform those operations with MCP server.** (PRD page 9). Phase 1 tools are read-only + authorize-session + cancel-session. If the token has broader permissions, those permissions are not exposed until Phase 2 tools are implemented. The server does not artificially restrict what the token can do.

### 7.3 MCP Security Guidelines

The PRD requires alignment with "Authenticated MCP server security guidelines" and "Anthropic outlined security good practices." This means:

1. **No unauthenticated access:** The server requires a token to perform any operation. Without a token, all tool calls return an authentication error.

2. **No sensitive data in tool descriptions:** Tool descriptions do not include the token or any cluster-specific sensitive information.

3. **Token not logged:** The server never logs the full token value. Log messages use a truncated form: `at_1234...7890`.

4. **TLS enforcement:** The server respects `BOUNDARY_CACERT`, `BOUNDARY_CAPATH`, and `BOUNDARY_TLS_INSECURE` from the existing API client. If `BOUNDARY_ADDR` uses `https://`, TLS is used. `BOUNDARY_TLS_INSECURE=true` is supported for development but the server logs a warning.

5. **Local execution only:** Phase 1 is stdio transport, running locally on the user's device. The token never leaves the user's machine. No network listener is opened by the MCP server itself (the proxy listener is on 127.0.0.1 only).

### 7.4 Session Authorization Token Storage

**Open question from PRD (page 12):** "Do we need MCP server to store the session authz token locally in a secure manner (e.g. on keychain on mac)?"

**Recommendation:** For Phase 1, do not persist session authorization tokens. The `authorize_session` tool returns the token in the tool result, and the LLM passes it to `start_proxy` in the same conversation. The proxy holds the token in memory for the session's lifetime. When the MCP server process exits, all tokens are discarded.

If the user needs to reconnect after restarting the server, they re-authorize. Session authorization tokens are short-lived (target TTL) and there is no value in persisting them.

Phase 2+ could add OS keychain integration if the agentic use case (PRD Requirement 8) requires unattended reconnection. This is deferred.

---

## 8. Architectural Questions

### Q1: Session cancel in Phase 1 — confirmed in scope

**Decision: Include `cancel_session` in Phase 1.** Confirmed by PRD owner. The PRD lists cancel under Phase 1 example prompts, and session authorize (also not strictly read-only) is explicitly in Phase 1 scope. Cancel is the natural counterpart to authorize.

### Q2: MCP Go SDK — build from scratch

**Decision: Build from scratch.** The MCP protocol is JSON-RPC 2.0 over stdio. The surface area for Phase 1 is small: `initialize` handshake, `tools/list`, `tools/call`, and logging to stderr. Implementing this directly avoids a third-party dependency in a security-sensitive product, gives full control over the JSON-RPC dispatch loop, and keeps the binary minimal. The protocol spec is well-documented at `https://modelcontextprotocol.io/specification/2025-06-18/`.

The implementation needs:
- JSON-RPC 2.0 message framing (newline-delimited, read from stdin, write to stdout)
- `initialize` / `initialized` handshake with capability negotiation (tools + logging)
- `tools/list` — returns tool definitions with JSON schemas
- `tools/call` — dispatches to registered tool handler, returns result or error
- `notifications/cancelled` — handle client cancellation (optional for beta)
- stderr logging (structured, never stdout)

Estimated ~500-800 lines of Go for the protocol layer. No external dependencies beyond the standard library and `encoding/json`.

### Q3: Repository structure

**Decision: Separate repo `hashicorp/boundary-mcp`.** Depend on `github.com/hashicorp/boundary/api` as a published Go module (currently `v0.0.60`). The API package is already designed as a separate module with a clean public interface — the MCP server only needs the client-facing API, not internal controller code. This satisfies the PRD's open-source requirement while keeping boundary-enterprise private.

### Q4: Tool granularity

**Decision: Granular — one tool per API operation.** The MCP protocol is designed for many granular tools. Modern LLMs handle 30+ tools well. Each tool's schema is the documentation — the LLM reads schemas to decide what to call. Composite tools would require the server to implement a query parser, which is the LLM's job.

### Q5: How does the proxy lifecycle work in the MCP tool-call model?

`proxy.ClientProxy.Start()` is a blocking call that runs until the session expires or the listener is closed. In the MCP model, a tool call must return a result — it cannot block indefinitely.

**Proposed design:**
1. `start_proxy` tool creates the `ClientProxy` with `proxy.New()`
2. Starts `proxy.Start()` in a goroutine
3. Waits for the listener address to be available (polls `ListenerAddress()` with a short timeout)
4. Returns the local address, session metadata
5. The proxy continues running in the background for the session's lifetime
6. `close_proxy` tool calls `cancel()` on the proxy context and waits for `Start()` to return
7. On server shutdown, all active proxies are cancelled

The server maintains a registry of active proxies (session ID → `*ClientProxy`), keyed by session ID. This supports the PRD's "Close my postgres database session" prompt — the LLM looks up the session by target name and calls `close_proxy`.

**Constraint:** One active proxy per MCP server instance (simplification for beta). Multiple proxies would require port management and session tracking that adds complexity. The PRD does not require concurrent sessions.

**Verified against MCP spec (2025-06-18):** Under stdio transport, the client launches the server as a long-lived subprocess and keeps it alive for the duration of the connection. Shutdown is initiated by the client closing stdin to the child process, then sending SIGTERM/SIGKILL if the server doesn't exit within a reasonable time. The server is not spawned/killed per tool call — it persists across all tool calls in a session. This confirms the proxy goroutine model works: `start_proxy` launches the background listener, it survives until `close_proxy` or client disconnect, and the server has a cleanup window on shutdown to cancel active proxies.

### Q6: Liveness and health check tool

**Decision: Yes.** A `check_connection` tool reads the global scope and returns the controller version, current user (from token), and token expiration. Useful for the LLM to diagnose connectivity issues and for the user to verify their setup.

### Q7: Output field selection

**Decision: Full objects, strip binary fields.** The LLM context can handle a few hundred kilobytes. If pagination is used, each page is bounded. For session recordings specifically, metadata is returned but the actual recording content (video, asciicast) is not — that requires a separate download mechanism outside MCP scope.

---

## 9. Phase 2/3 Architecture Outline

### Phase 2: Write Operations + Additional Auth

**Write tools (Create/Update/Delete):**
- Each resource type gets `create_*`, `update_*`, `delete_*` tools mirroring the API client methods
- Update tools require `version` for optimistic concurrency; the server uses `WithAutomaticVersioning(true)` to handle version lookup automatically
- Composite operations from the PRD (e.g. "Create a static host catalog, host set, and host, then expose as a target") are achieved by the LLM chaining individual create tools — no need for composite server-side tools

**OIDC auth (PRD Requirement 4):**
- New `authenticate_oidc` tool that launches a browser-based OAuth flow
- The server starts a local HTTP listener for the OAuth callback
- On success, the token is set on the `api.Client` and subsequent tools use it
- Configuration: `auth_mode: "oauth"` in the MCP config

**Password auth (PRD Requirement 5):**
- New `authenticate_password` tool that calls `authmethods.Client.Authenticate()` with `command: "login"` and password attributes
- Configuration: `USERNAME` and `PASSWORD` env vars, or passed in the tool call

**Target connection (PRD Requirement 3):**
- The `authorize_session` + `start_proxy` + `close_proxy` tools from Phase 1 handle this
- Phase 2 adds: user-provided credentials in the prompt (passed to `authorize_session` as session credentials), target type detection (TCP → prompt for endpoint type)

### Phase 3: Full CRUD + Agentic Use Cases

**Additional resource types:**
- Host catalogs, host sets, hosts (create/update/delete)
- Credential stores, credential libraries (create/update/delete)
- Roles, principals, grants (create/update/delete)
- Managed groups (create/update/delete)
- Users, accounts (create/update/delete, password provisioning)
- Policies (create/update/delete)

**Agentic use case (PRD Requirement 8):**
- Silent token auth via `BOUNDARY_TOKEN` env var (already supported in Phase 1)
- Sequenced lookup: target name → target ID → authorize → connect
- This is a natural LLM workflow using existing tools — no new server-side capability needed

---

## 10. Build and Distribution

### 10.1 Binary

```bash
# Build
go build -o boundary-mcp ./cmd/boundary-mcp/

# Run (stdio mode)
BOUNDARY_ADDR=https://boundary.example.com:9200 \
BOUNDARY_TOKEN=at_... \
./boundary-mcp
```

### 10.2 Docker Image

```dockerfile
FROM alpine:latest
COPY boundary-mcp /usr/local/bin/
ENTRYPOINT ["boundary-mcp"]
```

```bash
docker run -i --rm \
  -e BOUNDARY_ADDR=https://boundary.example.com:9200 \
  -e BOUNDARY_TOKEN=at_... \
  hashicorp/boundary-mcp:latest
```

### 10.3 One-Click Install Configurations

**VS Code (`.vscode/mcp.json`):**
```json
{
  "servers": {
    "boundary": {
      "command": "docker",
      "args": ["run", "-i", "--rm",
               "-e", "BOUNDARY_ADDR",
               "-e", "BOUNDARY_TOKEN",
               "-e", "BOUNDARY_CACERT"],
      "env": {
        "BOUNDARY_ADDR": "https://boundary.example.com:9200",
        "BOUNDARY_TOKEN": "at_1234567890_...",
        "BOUNDARY_CACERT": "/path/to/ca.pem"
      }
    }
  }
}
```

**Claude Desktop (`claude_desktop_config.json`):**
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

**Cursor:**
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

### 10.4 GitHub Repository

- Public repo: `github.com/hashicorp/boundary-mcp`
- License: MPL-2.0 (consistent with Boundary)
- Go module: `github.com/hashicorp/boundary-mcp`
- Dependency: `github.com/hashicorp/boundary/api v0.0.60` (or latest published)
- CI: GitHub Actions (build, test, lint, release binaries via goreleaser)
- Releases: GitHub Releases with binaries for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- Docker image published to Docker Hub: `hashicorp/boundary-mcp`

---

## 11. Telemetry

### 11.1 Phase 1 Telemetry

The PRD requires telemetry for:
- Number of downloads by deployment channel
- Total number of orgs using MCP server
- Users who set up MCP server
- Usage metrics by client type (VS Code vs. Claude vs. Cursor)
- GitHub forks and stars
- Tools being used actively (subject to feasibility and legal clearance)

**Download/usage tracking:**
- GitHub releases download counts (via GitHub API)
- Docker Hub pull counts (via Docker Hub API)
- These are channel-level metrics, not user-level

**Client type detection:**
- The MCP protocol includes an `initialize` request with client info (`clientInfo.name`, `clientInfo.version`)
- The server logs this on connection and can report it in a `server_info` tool
- Client names: `vscode`, `claude-desktop`, `cursor` (or similar)

**Tool usage tracking:**
- The server can track tool call counts in-memory and expose via a `server_stats` tool
- For anonymous telemetry, the server could send usage pings to a HashiCorp endpoint (opt-in, configurable)
- **Legal clearance required** before implementing any outbound telemetry

### 11.2 Implementation

Phase 1 telemetry is **in-memory only** — no outbound network calls from the server except to the Boundary controller. A `server_info` tool returns:
- Connected client name and version
- Boundary controller address and version
- Token type (app vs user, truncated)
- Tool call counts (since server start)
- Active proxy sessions

This satisfies the PRD's "tools being used actively" requirement without requiring legal clearance for outbound telemetry.

---

## 12. Testing Strategy

### 12.1 Unit Tests

- Tool schema validation: every tool has a valid JSON schema
- Filter construction: bexpr filters are correctly passed to the API client
- Pagination: `list_token` is correctly passed and returned
- Error translation: API errors map to correct MCP error messages
- Proxy lifecycle: `start_proxy` → `close_proxy` correctly manages the proxy goroutine

### 12.2 Integration Tests

- Against a local Boundary dev server (docker-compose with boundary controller + worker)
- End-to-end: authenticate → list targets → read target → authorize session → start proxy → verify local listener → close proxy
- Error scenarios: invalid token, insufficient permissions, nonexistent resource, rate limiting

### 12.3 MCP Protocol Tests

- Verify `initialize` handshake
- Verify `tools/list` returns all registered tools with valid schemas
- Verify `tools/call` returns correct results
- Verify error envelopes conform to MCP spec

---

## 13. Implementation Plan (Phase 1)

### Task 1: Project scaffold
- Create `github.com/hashicorp/boundary-mcp` repo
- `go.mod` with dependency on `github.com/hashicorp/boundary/api`
- `cmd/boundary-mcp/main.go` entry point
- MCP server initialization (stdio transport)

### Task 2: Configuration and client setup
- Read `BOUNDARY_ADDR`, `BOUNDARY_TOKEN`, TLS config from environment
- Create and validate `api.Client`
- Startup liveness check (`scopes.Client.Read(ctx, "global")`)
- Graceful shutdown handler

### Task 3: Tool registry framework
- Tool registration interface (name, description, schema, handler)
- JSON schema generation from Go structs
- Tool listing endpoint (`tools/list`)
- Tool call dispatch (`tools/call`)

### Task 4: Read/list tools — scopes, targets
- `list_scopes`, `read_scope`
- `list_targets`, `read_target`
- Pagination support (client-directed, page_size, list_token)
- Filter passthrough (bexpr)

### Task 5: Read/list tools — hosts, workers, sessions
- `list_host_catalogs`, `read_host_catalog`
- `list_host_sets`, `read_host_set`
- `list_hosts`, `read_host`
- `list_workers`, `read_worker`
- `list_sessions`, `read_session`

### Task 6: Read/list tools — users, groups, roles, auth
- `list_users`, `read_user`
- `list_groups`, `read_group`
- `list_roles`, `read_role`
- `list_auth_methods`, `read_auth_method`
- `list_auth_tokens`

### Task 7: Read/list tools — credentials, recordings
- `list_credential_stores`, `read_credential_store`
- `list_credential_libraries`, `read_credential_library`
- `list_session_recordings`, `read_session_recording`

### Task 8: Session tools — authorize, proxy, cancel
- `authorize_session` (calls `targets.Client.AuthorizeSession()`)
- `start_proxy` (calls `proxy.New()` + goroutine for `Start()`)
- `close_proxy` (cancels proxy context, calls `CloseSession()`)
- `cancel_session` (calls `sessions.Client.Cancel()`)
- Proxy registry (session ID → `*ClientProxy`)

### Task 9: Error handling and security
- API error → MCP error translation
- Token masking in logs
- TLS warning for insecure mode
- `check_connection` tool

### Task 10: Telemetry and server info
- `server_info` tool (client type, controller version, token type, tool call counts)
- In-memory tool call counter

### Task 11: Docker and distribution
- Dockerfile
- Multi-arch build (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64)
- One-click install configs for VS Code, Claude Desktop, Cursor
- GitHub Actions CI/CD with goreleaser

### Task 12: Documentation
- README with setup guide
- Example prompts (from PRD)
- Architecture documentation
- Contributing guide

---

## 14. References

- [PRD ICU-060] Boundary MCP Server (source document)
- Boundary API Overview: https://developer.hashicorp.com/boundary/docs/api
- Boundary API client (Go): `github.com/hashicorp/boundary/api` (v0.0.60)
- Boundary filtering (bexpr): `github.com/hashicorp/go-bexpr`
- MCP Specification: https://spec.modelcontextprotocol.io/
- Anthropic MCP security guidelines: https://modelcontextprotocol.io/docs/concepts/security
- Terraform MCP Server (reference implementation): https://github.com/hashicorp/terraform-mcp-server

---

## Appendix A: Complete Tool Catalog (Phase 1)

| # | Tool | API Package | API Method | Operations |
|---|------|------------|-----------|------------|
| 1 | `check_connection` | `scopes` | `Read("global")` | Read |
| 2 | `list_scopes` | `scopes` | `List()` | List |
| 3 | `read_scope` | `scopes` | `Read()` | Read |
| 4 | `list_targets` | `targets` | `List()` | List |
| 5 | `read_target` | `targets` | `Read()` | Read |
| 6 | `list_host_catalogs` | `hostcatalogs` | `List()` | List |
| 7 | `read_host_catalog` | `hostcatalogs` | `Read()` | Read |
| 8 | `list_host_sets` | `hostsets` | `List()` | List |
| 9 | `read_host_set` | `hostsets` | `Read()` | Read |
| 10 | `list_hosts` | `hosts` | `List()` | List |
| 11 | `read_host` | `hosts` | `Read()` | Read |
| 12 | `list_sessions` | `sessions` | `List()` | List |
| 13 | `read_session` | `sessions` | `Read()` | Read |
| 14 | `cancel_session` | `sessions` | `Cancel()` | Write (session lifecycle) |
| 15 | `list_workers` | `workers` | `List()` | List |
| 16 | `read_worker` | `workers` | `Read()` | Read |
| 17 | `list_users` | `users` | `List()` | List |
| 18 | `read_user` | `users` | `Read()` | Read |
| 19 | `list_groups` | `groups` | `List()` | List |
| 20 | `read_group` | `groups` | `Read()` | Read |
| 21 | `list_roles` | `roles` | `List()` | List |
| 22 | `read_role` | `roles` | `Read()` | Read |
| 23 | `list_auth_methods` | `authmethods` | `List()` | List |
| 24 | `read_auth_method` | `authmethods` | `Read()` | Read |
| 25 | `list_auth_tokens` | `authtokens` | `List()` | List |
| 26 | `list_credential_stores` | `credentialstores` | `List()` | List |
| 27 | `read_credential_store` | `credentialstores` | `Read()` | Read |
| 28 | `list_credential_libraries` | `credentiallibraries` | `List()` | List |
| 29 | `read_credential_library` | `credentiallibraries` | `Read()` | Read |
| 30 | `list_session_recordings` | `sessionrecordings` | `List()` (custom) | List |
| 31 | `read_session_recording` | `sessionrecordings` | `Read()` (custom) | Read |
| 32 | `authorize_session` | `targets` | `AuthorizeSession()` | Write (session lifecycle) |
| 33 | `start_proxy` | `proxy` | `New()` + `Start()` | Side effect (local listener) |
| 34 | `close_proxy` | `proxy` | `CloseSession()` / cancel | Write (session lifecycle) |
| 35 | `server_info` | n/a | n/a (in-memory) | Read (local state) |

**Total: 35 tools (33 Boundary API operations + 2 server-level tools)**

---

## Appendix B: PRD Requirement Coverage Matrix

| PRD Requirement | RFC Section | Phase | Coverage |
|----------------|------------|-------|----------|
| Req 1: Publicly available server image | §10 (Build/Distribution) | 1 | Full |
| Req 2: Read and list operations + filtering | §4 (Tool Catalog), §5 (Pagination), §6 (Filtering) | 1 | Full |
| Req 3: Session initiation and close | §4.2.11 (authorize/proxy tools) | 1 | Full |
| Req 4: OIDC auth method | §9 (Phase 2 outline) | 2 | Outline |
| Req 5: Password auth method | §9 (Phase 2 outline) | 2 | Outline |
| Req 2 (P3): Static catalog and target | §9 (Phase 3 outline) | 3 | Outline |
| Req 3 (P3): Dynamic credentials via Vault | §9 (Phase 3 outline) | 3 | Outline |
| Req 4 (P3): Federated OIDC managed groups | §9 (Phase 3 outline) | 3 | Outline |
| Req 5 (P3): Role principal auditing | §9 (Phase 3 outline) | 3 | Outline |
| Req 6 (P3): Static credential provisioning | §9 (Phase 3 outline) | 3 | Outline |
| Req 7 (P3): Password-backed user provisioning | §9 (Phase 3 outline) | 3 | Outline |
| Req 8 (P3): AI agentic token auth | §9 (Phase 3 outline) | 3 | Outline |
| Req 10 (P3): AI agentic password auth | §9 (Phase 3 outline) | 3 | Outline |
| Security: Authenticated MCP guidelines | §7 (Security) | All | Full |
| Security: Token permissions | §7.2 | All | Full |
| NFR: Docker image | §10.2 | All | Full |
| NFR: Client integration (VS Code, Claude, Cursor) | §10.3 | All | Full |
| NFR: GitHub Marketplace | §10.4 | Post-beta | Noted |
| NFR: AWS Marketplace | §10.4 | Post-beta | Noted |
| NFR: Open-source repo | §10.4 | All | Full |
| Telemetry: Downloads, orgs, client types | §11 | All | Full (in-memory) |
| Documentation: Setup guide, tutorials | §13, Task 12 | All | Full |

---

## Appendix C: Key API Client Patterns Used

All patterns verified against `boundary-enterprise-main/api/` source:

### Client creation
```go
config, _ := api.DefaultConfig()  // reads BOUNDARY_ADDR, BOUNDARY_TOKEN, etc.
client, _ := api.NewClient(config)
```

### Resource list with pagination and filter
```go
targetClient := targets.NewClient(apiClient)
result, err := targetClient.List(ctx, "o_12345",
    targets.WithFilter(`"type" == "ssh"`),
    targets.WithClientDirectedPagination(true),
    targets.WithPageSize(100),
)
// result.Items, result.EstItemCount, result.ListToken, result.ResponseType
// Next page: targetClient.ListNextPage(ctx, result, ...)
```

### Resource read
```go
result, err := targetClient.Read(ctx, "ttcp_12345")
// result.Item (full *targets.Target), result.Response
```

### Session authorize
```go
result, err := targetClient.AuthorizeSession(ctx, "ttcp_12345")
// result.Item.AuthorizationToken (base58-encoded protobuf)
// Can be decoded: result.Item.GetSessionAuthorizationData()
```

### Proxy
```go
proxy, _ := proxy.New(ctx, authzToken)
// proxy.Start() — blocking, runs until session ends
// proxy.ListenerAddress(ctx) — returns "127.0.0.1:PORT"
// proxy.CloseSession(timeout) — sends teardown to worker
```

### Session cancel
```go
sessionClient := sessions.NewClient(apiClient)
result, err := sessionClient.Cancel(ctx, "s_12345", 0,
    sessions.WithAutomaticVersioning(true),
)
```

### Auth (Phase 2)
```go
authClient := authmethods.NewClient(apiClient)
result, err := authClient.Authenticate(ctx, "ampw_12345", "login",
    map[string]any{"login_name": "john", "password": "..."},
)
// result.GetAuthToken() → *authtokens.AuthToken
// apiClient.SetToken(authToken.Token)
```