package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	miroclient "github.com/your-org/miro-mcp/internal/miro"
)

// RegisterContextTools adds context_explore and context_get to the server.
func RegisterContextTools(s *server.MCPServer, c *miroclient.Client) {
	registerContextExplore(s, c)
	registerContextGet(s, c)
}

func registerContextExplore(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("context_explore",
		mcp.WithDescription("Discover high-level items on a Miro board (frames, documents, prototypes, tables, diagrams) with their URLs and titles. Use this first to understand the board structure before fetching specific content."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID (found in the board URL after /board/)."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID, err := req.RequireString("board_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch several high-level item types in parallel-ish by iterating.
		types := []string{"frame", "doc_format", "document", "image"}
		discovered := []map[string]any{}

		for _, itemType := range types {
			params := url.Values{}
			params.Set("type", itemType)
			params.Set("limit", "50")

			data, _, err := c.Get(ctx, fmt.Sprintf("/boards/%s/items", boardID), params)
			if err != nil {
				// Non-fatal: some types may not be available on all plans
				continue
			}

			var resp struct {
				Data []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
					Data struct {
						Title   string `json:"title"`
						Content string `json:"content"`
					} `json:"data"`
					Position struct {
						X float64 `json:"x"`
						Y float64 `json:"y"`
					} `json:"position"`
				} `json:"data"`
				Cursor string `json:"cursor"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				continue
			}

			for _, item := range resp.Data {
				title := item.Data.Title
				if title == "" {
					title = item.Data.Content
				}
				if len(title) > 80 {
					title = title[:80] + "..."
				}
				boardURL := fmt.Sprintf("https://miro.com/app/board/%s/?moveToWidget=%s", boardID, item.ID)
				discovered = append(discovered, map[string]any{
					"id":    item.ID,
					"type":  item.Type,
					"title": title,
					"url":   boardURL,
					"position": map[string]any{
						"x": item.Position.X,
						"y": item.Position.Y,
					},
				})
			}
		}

		out := map[string]any{
			"board_id": boardID,
			"items":    discovered,
			"count":    len(discovered),
		}
		outBytes, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}

func registerContextGet(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("context_get",
		mcp.WithDescription("Get text context from a specific Miro board item. Documents return HTML content, frames return a summary of child items, images return metadata. Use context_explore first to find item IDs."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("item_id",
			mcp.Required(),
			mcp.Description("The ID of the specific item to retrieve content from."),
		),
		mcp.WithString("item_type",
			mcp.Required(),
			mcp.Description("The type of the item: frame, doc_format, document, image, sticky_note, shape, text, card."),
			mcp.Enum("frame", "doc_format", "document", "image", "sticky_note", "shape", "text", "card"),
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
		itemType, err := req.RequireString("item_type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var apiPath string
		switch itemType {
		case "frame":
			apiPath = fmt.Sprintf("/boards/%s/frames/%s", boardID, itemID)
		case "doc_format":
			apiPath = fmt.Sprintf("/boards/%s/doc_format_items/%s", boardID, itemID)
		case "document":
			apiPath = fmt.Sprintf("/boards/%s/documents/%s", boardID, itemID)
		case "image":
			apiPath = fmt.Sprintf("/boards/%s/images/%s", boardID, itemID)
		case "sticky_note":
			apiPath = fmt.Sprintf("/boards/%s/sticky_notes/%s", boardID, itemID)
		case "shape":
			apiPath = fmt.Sprintf("/boards/%s/shapes/%s", boardID, itemID)
		case "text":
			apiPath = fmt.Sprintf("/boards/%s/texts/%s", boardID, itemID)
		case "card":
			apiPath = fmt.Sprintf("/boards/%s/cards/%s", boardID, itemID)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("unsupported item type: %s", itemType)), nil
		}

		data, _, err := c.Get(ctx, apiPath, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// For frames, also fetch child items to provide a text summary.
		if itemType == "frame" {
			var frameItem map[string]any
			_ = json.Unmarshal(data, &frameItem)

			childParams := url.Values{}
			childParams.Set("limit", "50")
			childData, _, childErr := c.Get(ctx, fmt.Sprintf("/boards/%s/frames/%s/items", boardID, itemID), childParams)

			if childErr == nil {
				var childResp struct {
					Data []struct {
						ID   string `json:"id"`
						Type string `json:"type"`
						Data struct {
							Content string `json:"content"`
							Title   string `json:"title"`
						} `json:"data"`
					} `json:"data"`
				}
				if err := json.Unmarshal(childData, &childResp); err == nil {
					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Frame %s contains %d items:\n\n", itemID, len(childResp.Data)))
					for _, child := range childResp.Data {
						text := child.Data.Content
						if text == "" {
							text = child.Data.Title
						}
						if text == "" {
							text = "(no text)"
						}
						if len(text) > 200 {
							text = text[:200] + "..."
						}
						sb.WriteString(fmt.Sprintf("[%s id=%s]: %s\n", child.Type, child.ID, text))
					}
					out := map[string]any{
						"frame":    frameItem,
						"children": sb.String(),
					}
					outBytes, _ := json.MarshalIndent(out, "", "  ")
					return mcp.NewToolResultText(string(outBytes)), nil
				}
			}
		}

		// For all other types, return the raw JSON response.
		var pretty map[string]any
		if err := json.Unmarshal(data, &pretty); err != nil {
			return mcp.NewToolResultText(string(data)), nil
		}
		outBytes, _ := json.MarshalIndent(pretty, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}
