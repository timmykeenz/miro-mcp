package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	miroclient "github.com/your-org/miro-mcp/internal/miro"
)

// RegisterDocTools adds doc_create, doc_get, and doc_update to the server.
func RegisterDocTools(s *server.MCPServer, c *miroclient.Client) {
	registerDocCreate(s, c)
	registerDocGet(s, c)
	registerDocUpdate(s, c)
}

func registerDocCreate(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("doc_create",
		mcp.WithDescription("Create a doc format item (structured document with markdown support for headings, bold, italic, lists, links) on a Miro board."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Markdown content for the document. Supports headings (#, ##), bold (**text**), italic (*text*), lists (- item), and links ([text](url))."),
		),
		mcp.WithString("title",
			mcp.Description("Optional title for the document."),
		),
		mcp.WithNumber("x",
			mcp.Description("X position on the board. Defaults to 0."),
		),
		mcp.WithNumber("y",
			mcp.Description("Y position on the board. Defaults to 0."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID, err := req.RequireString("board_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		x := req.GetFloat("x", 0.0)
		y := req.GetFloat("y", 0.0)

		body := map[string]any{
			"data": map[string]any{
				"content": markdownToMiroDoc(content),
				"format":  "markdown",
			},
			"position": map[string]any{
				"x":      x,
				"y":      y,
				"origin": "center",
			},
			"geometry": map[string]any{
				"width": 600.0,
			},
		}

		if title := req.GetString("title", ""); title != "" {
			body["data"].(map[string]any)["title"] = title
		}

		data, _, err := c.Post(ctx, fmt.Sprintf("/boards/%s/doc_format_items", boardID), body)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var resp map[string]any
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultText(string(data)), nil
		}
		outBytes, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}

func registerDocGet(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("doc_get",
		mcp.WithDescription("Read the markdown content and version of an existing doc format item from a Miro board."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("item_id",
			mcp.Required(),
			mcp.Description("The ID of the doc format item."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID, err := req.RequireString("board_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		itemID, err := req.RequireString("item_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, _, err := c.Get(ctx, fmt.Sprintf("/boards/%s/doc_format_items/%s", boardID, itemID), nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var resp map[string]any
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultText(string(data)), nil
		}
		outBytes, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}

func registerDocUpdate(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("doc_update",
		mcp.WithDescription("Edit content in an existing doc format item using find-and-replace. Can replace the first occurrence or all occurrences of the search string."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("item_id",
			mcp.Required(),
			mcp.Description("The ID of the doc format item to update."),
		),
		mcp.WithString("find",
			mcp.Required(),
			mcp.Description("The text string to find in the document."),
		),
		mcp.WithString("replace",
			mcp.Required(),
			mcp.Description("The text string to replace the found text with."),
		),
		mcp.WithBoolean("replace_all",
			mcp.Description("If true, replace all occurrences. If false (default), replace only the first."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID, err := req.RequireString("board_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		itemID, err := req.RequireString("item_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		find, err := req.RequireString("find")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		replace, err := req.RequireString("replace")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		replaceAll := req.GetBool("replace_all", false)

		// First, fetch existing content.
		data, _, err := c.Get(ctx, fmt.Sprintf("/boards/%s/doc_format_items/%s", boardID, itemID), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("fetch doc: %v", err)), nil
		}

		var current map[string]any
		if err := json.Unmarshal(data, &current); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse doc: %v", err)), nil
		}

		// Extract and modify content.
		dataField, _ := current["data"].(map[string]any)
		if dataField == nil {
			return mcp.NewToolResultError("doc has no data field"), nil
		}
		content, _ := dataField["content"].(string)
		if content == "" {
			return mcp.NewToolResultError("doc content is empty"), nil
		}

		var newContent string
		if replaceAll {
			newContent = strings.ReplaceAll(content, find, replace)
		} else {
			newContent = strings.Replace(content, find, replace, 1)
		}

		if newContent == content {
			return mcp.NewToolResultText(`{"status":"no_change","message":"Search string not found in document"}`), nil
		}

		// Patch with updated content.
		body := map[string]any{
			"data": map[string]any{
				"content": newContent,
				"format":  "markdown",
			},
		}

		patchData, _, err := c.Patch(ctx, fmt.Sprintf("/boards/%s/doc_format_items/%s", boardID, itemID), body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("update doc: %v", err)), nil
		}

		var resp map[string]any
		if err := json.Unmarshal(patchData, &resp); err != nil {
			return mcp.NewToolResultText(string(patchData)), nil
		}
		outBytes, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}

// markdownToMiroDoc is a passthrough — the Miro API accepts markdown directly.
func markdownToMiroDoc(md string) string {
	return md
}
