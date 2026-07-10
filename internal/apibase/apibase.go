package apibase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	bclient "github.com/learhy/boundary-mcp/internal/client"
	"github.com/learhy/boundary-mcp/internal/mcp"
)

// ListParams holds common parameters for list operations.
type ListParams struct {
	ScopeID            string `json:"scope_id"`
	Recursive          bool   `json:"recursive"`
	Filter             string `json:"filter"`
	PageSize           int    `json:"page_size"`
	ListToken          string `json:"list_token"`
	HostCatalogID      string `json:"host_catalog_id"`
	CredentialStoreID  string `json:"credential_store_id"`
}

// ReadParams holds common parameters for read operations.
type ReadParams struct {
	ID string `json:"id"`
}

// ListResult is the generic structure for list tool output.
type ListResult struct {
	Items        json.RawMessage `json:"items"`
	EstItemCount int             `json:"est_item_count"`
	ResponseType string          `json:"response_type"`
	ListToken    *string         `json:"list_token"`
	RemovedIDs   json.RawMessage `json:"removed_ids"`
	Hint         string          `json:"hint,omitempty"`
}

// ReadResult is the generic structure for read tool output.
type ReadResult struct {
	Item json.RawMessage `json:"item"`
}

// ParseListParams decodes raw JSON arguments into ListParams.
func ParseListParams(args json.RawMessage) (*ListParams, error) {
	var p ListParams
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	return &p, nil
}

// ParseReadParams decodes raw JSON arguments into ReadParams.
func ParseReadParams(args json.RawMessage) (*ReadParams, error) {
	var p ReadParams
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}
	return &p, nil
}

// BuildListQuery constructs the URL query string for a Boundary list API call.
// Returns the full path with query parameters.
func BuildListQuery(basePath string, p *ListParams) string {
	q := url.Values{}
	if p.ScopeID != "" {
		q.Set("scope_id", p.ScopeID)
	}
	if p.HostCatalogID != "" {
		q.Set("host_catalog_id", p.HostCatalogID)
	}
	if p.CredentialStoreID != "" {
		q.Set("credential_store_id", p.CredentialStoreID)
	}
	if p.Filter != "" {
		q.Set("filter", p.Filter)
	}
	if p.Recursive {
		q.Set("recursive", "true")
	}
	if p.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(p.PageSize))
	}
	if p.ListToken != "" {
		q.Set("list_token", p.ListToken)
	}
	if len(q) > 0 {
		return basePath + "?" + q.Encode()
	}
	return basePath
}

// CheckToken returns an error if the client has no token.
func CheckToken(c *bclient.Client) error {
	if c.Token == "" {
		return fmt.Errorf("Authentication failed. Please configure a valid BOUNDARY_TOKEN. Current token is missing or expired.")
	}
	return nil
}

// HandleListResponse processes a list API response and returns an MCP result.
func HandleListResponse(body json.RawMessage, statusCode int, err error) (*mcp.ToolCallResult, error) {
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	if statusCode != 200 {
		return nil, fmt.Errorf("%s", translateError(statusCode, body))
	}

	var result ListResult
	result.Items = body

	var fullResp struct {
		Items        json.RawMessage `json:"items"`
		EstItemCount int             `json:"est_item_count"`
		ResponseType string          `json:"response_type"`
		ListToken    *string         `json:"list_token"`
		RemovedIDs   json.RawMessage `json:"removed_ids"`
	}
	if json.Unmarshal(body, &fullResp) == nil {
		// If items field exists (even if null), use the structured response
		if fullResp.Items != nil {
			result.Items = fullResp.Items
		} else {
			// items is null — empty result set
			result.Items = json.RawMessage("[]")
		}
		result.EstItemCount = fullResp.EstItemCount
		result.ResponseType = fullResp.ResponseType
		if result.ResponseType == "" {
			result.ResponseType = "complete"
		}
		result.ListToken = fullResp.ListToken
		result.RemovedIDs = fullResp.RemovedIDs
	} else {
		// Body might be the items array directly
		result.Items = body
		result.ResponseType = "complete"
	}

	// Hint for large result sets
	if result.EstItemCount > 100 {
		result.Hint = fmt.Sprintf(
			"Large result set (est. %d items). Consider adding a filter to narrow results, or call again with list_token to page through.",
			result.EstItemCount,
		)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.TextResult(string(out)), nil
}

// HandleReadResponse processes a read API response and returns an MCP result.
func HandleReadResponse(body json.RawMessage, statusCode int, err error) (*mcp.ToolCallResult, error) {
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	if statusCode != 200 {
		return nil, fmt.Errorf("%s", translateError(statusCode, body))
	}

	var result ReadResult
	result.Item = body

	out, _ := json.MarshalIndent(result, "", "  ")
	return mcp.TextResult(string(out)), nil
}

// HandleRawResponse processes a raw API response (for non-standard operations).
func HandleRawResponse(body json.RawMessage, statusCode int, err error) (*mcp.ToolCallResult, error) {
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	if statusCode != 200 && statusCode != 201 && statusCode != 204 {
		return nil, fmt.Errorf("%s", translateError(statusCode, body))
	}

	if len(body) == 0 {
		return mcp.TextResult("{}"), nil
	}
	return mcp.TextResult(string(body)), nil
}

// translateError converts an HTTP status code and response body into a
// human-readable MCP error message.
func translateError(statusCode int, body json.RawMessage) string {
	var apiErr struct {
		Details []struct {
			Messages []string `json:"messages"`
		} `json:"details"`
	}
	var msg string
	if json.Unmarshal(body, &apiErr) == nil && len(apiErr.Details) > 0 {
		for _, d := range apiErr.Details {
			msg += fmt.Sprintf("%v; ", d.Messages)
		}
	}

	switch statusCode {
	case 401:
		if msg != "" {
			return fmt.Sprintf("Authentication failed. Please configure a valid BOUNDARY_TOKEN. Details: %s", msg)
		}
		return "Authentication failed. Please configure a valid BOUNDARY_TOKEN. Current token is missing or expired."
	case 403:
		if msg != "" {
			return fmt.Sprintf("Your token does not have permission to perform this operation. Details: %s", msg)
		}
		return "Your token does not have permission to perform this operation."
	case 404:
		return "Resource not found. The resource does not exist or you do not have access to it."
	case 400:
		if msg != "" {
			return fmt.Sprintf("Invalid request: %s", msg)
		}
		return "Invalid request."
	case 429:
		return "Rate limited. Please retry after a few seconds."
	default:
		if statusCode >= 500 {
			if msg != "" {
				return fmt.Sprintf("Boundary controller error (HTTP %d): %s. The server may be experiencing issues.", statusCode, msg)
			}
			return fmt.Sprintf("Boundary controller error (HTTP %d). The server may be experiencing issues.", statusCode)
		}
		if msg != "" {
			return fmt.Sprintf("API error (HTTP %d): %s", statusCode, msg)
		}
		return fmt.Sprintf("API error (HTTP %d).", statusCode)
	}
}

// ExecuteList performs a standard list API call and returns the MCP result.
func ExecuteList(ctx context.Context, c *bclient.Client, basePath string, p *ListParams) (*mcp.ToolCallResult, error) {
	if err := CheckToken(c); err != nil {
		return nil, err
	}
	path := BuildListQuery(basePath, p)
	body, status, err := c.DoGet(ctx, path)
	return HandleListResponse(body, status, err)
}

// ExecuteRead performs a standard read API call and returns the MCP result.
func ExecuteRead(ctx context.Context, c *bclient.Client, basePath, id string) (*mcp.ToolCallResult, error) {
	if err := CheckToken(c); err != nil {
		return nil, err
	}
	body, status, err := c.DoGet(ctx, basePath+"/"+id)
	return HandleReadResponse(body, status, err)
}

// ExecutePost performs a POST API call and returns the MCP result.
func ExecutePost(ctx context.Context, c *bclient.Client, path string, reqBody interface{}) (*mcp.ToolCallResult, error) {
	if err := CheckToken(c); err != nil {
		return nil, err
	}
	body, status, err := c.DoPost(ctx, path, reqBody)
	return HandleRawResponse(body, status, err)
}