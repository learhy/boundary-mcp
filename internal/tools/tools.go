// Package tools contains all MCP tool definitions and handlers for the
// Boundary MCP server. Each tool maps to one Boundary API operation.
package tools

import (
	"encoding/json"

	"github.com/learhy/boundary-mcp/internal/mcp"
)

// ToolSchema builds a JSON Schema inputSchema as json.RawMessage.
func ToolSchema(properties map[string]json.RawMessage, required []string) json.RawMessage {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	data, _ := json.Marshal(schema)
	return data
}

// Prop creates a property definition for a JSON schema.
func Prop(propType, description string, def interface{}) json.RawMessage {
	m := map[string]interface{}{
		"type":        propType,
		"description": description,
	}
	if def != nil {
		m["default"] = def
	}
	data, _ := json.Marshal(m)
	return data
}

// PropWithConstraints creates a property with min/max constraints.
func PropWithConstraints(propType, description string, min, max interface{}) json.RawMessage {
	m := map[string]interface{}{
		"type":        propType,
		"description": description,
	}
	if min != nil {
		m["minimum"] = min
	}
	if max != nil {
		m["maximum"] = max
	}
	data, _ := json.Marshal(m)
	return data
}

// ArrayProp creates an array property.
func ArrayProp(itemType, description string) json.RawMessage {
	m := map[string]interface{}{
		"type":        "array",
		"description": description,
		"items":        map[string]string{"type": itemType},
	}
	data, _ := json.Marshal(m)
	return data
}

// Registration creates a ToolRegistration from a name, description, schema, and handler.
func Registration(name, description string, schema json.RawMessage, handler mcp.ToolHandler) *mcp.ToolRegistration {
	return &mcp.ToolRegistration{
		Definition: mcp.ToolDefinition{
			Name:        name,
			Description: description,
			InputSchema: schema,
		},
		Handler: handler,
	}
}