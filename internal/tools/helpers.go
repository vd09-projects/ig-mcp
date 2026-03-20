package tools

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// errorResult builds an MCP error response from a Go error.
func errorResult[Out any](err error) *mcp.CallToolResultFor[Out] {
	return &mcp.CallToolResultFor[Out]{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
		},
		IsError: true,
	}
}

// okResult builds a successful MCP response with both human-readable text
// content and machine-readable structured content so the client always has data.
func okResult[Out any](v Out) *mcp.CallToolResultFor[Out] {
	return &mcp.CallToolResultFor[Out]{
		Content: []mcp.Content{
			&mcp.TextContent{Text: toJSON(v)},
		},
		StructuredContent: v,
	}
}

// toJSON marshals v to indented JSON, returning an error object on failure.
func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "marshal failed: %v"}`, err)
	}
	return string(b)
}
