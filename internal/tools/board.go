package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	miroclient "github.com/your-org/miro-mcp/internal/miro"
)

// BoardListItemsResponse mirrors the Miro v2 items list response shape.
type boardItemsResponse struct {
	Data   []json.RawMessage `json:"data"`
	Cursor string            `json:"cursor"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
	Size   int               `json:"size"`
}

// RegisterBoardTools adds the board_list_items tool to the server.
func RegisterBoardTools(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("board_list_items",
		mcp.WithDescription("List items on a Miro board with cursor-based pagination. Supports filtering by item type and parent container."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID (found in the board URL after /board/)."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results per page (10–50). Defaults to 10."),
		),
		mcp.WithString("cursor",
			mcp.Description("Pagination cursor from a previous response to fetch the next page."),
		),
		mcp.WithString("type",
			mcp.Description("Filter by item type: app_card, card, connector, data_table_format, document, embed, frame, image, shape, sticky_note, text, doc_format."),
			mcp.Enum(
				"app_card", "card", "connector", "data_table_format", "document",
				"embed", "frame", "image", "shape", "sticky_note", "text", "doc_format",
			),
		),
		mcp.WithString("parent_item_id",
			mcp.Description("Restrict results to children of this parent item ID (e.g. a frame ID)."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID, err := req.RequireString("board_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		params := url.Values{}

		args := req.GetArguments()

		if limitVal, ok := args["limit"]; ok && limitVal != nil {
			lf, ok := limitVal.(float64)
			if !ok {
				return mcp.NewToolResultError("limit must be a number"), nil
			}
			li := int(lf)
			if li < 10 {
				li = 10
			}
			if li > 50 {
				li = 50
			}
			params.Set("limit", fmt.Sprintf("%d", li))
		}

		if cursor, ok := args["cursor"].(string); ok && cursor != "" {
			params.Set("cursor", cursor)
		}

		itemType, _ := args["type"].(string)

		if itemType != "" && itemType != "connector" {
			params.Set("type", itemType)
		}

		if parentID, ok := args["parent_item_id"].(string); ok && parentID != "" {
			params.Set("parent_item_id", parentID)
		}

		apiPath := fmt.Sprintf("/boards/%s/items", boardID)
		if itemType == "connector" {
			apiPath = fmt.Sprintf("/boards/%s/connectors", boardID)
		}

		data, _, err := c.Get(ctx, apiPath, params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var resp boardItemsResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse response: %v", err)), nil
		}

		out := map[string]any{
			"items":  resp.Data,
			"cursor": resp.Cursor,
			"total":  resp.Total,
			"size":   resp.Size,
		}
		outBytes, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}
