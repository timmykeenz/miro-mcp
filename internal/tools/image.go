package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	miroclient "github.com/your-org/miro-mcp/internal/miro"
)

// RegisterImageTools adds image_get_url and image_get_data to the server.
func RegisterImageTools(s *server.MCPServer, c *miroclient.Client) {
	registerImageGetURL(s, c)
	registerImageGetData(s, c)
}

func registerImageGetURL(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("image_get_url",
		mcp.WithDescription("Get the download URL for an image item from a Miro board. The URL can be used to access or link to the image."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("item_id",
			mcp.Required(),
			mcp.Description("The ID of the image item on the board."),
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

		data, _, err := c.Get(ctx, fmt.Sprintf("/boards/%s/images/%s", boardID, itemID), nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var resp struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Data struct {
				ImageURL string `json:"imageUrl"`
				Title    string `json:"title"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultText(string(data)), nil
		}

		out := map[string]any{
			"item_id":      resp.ID,
			"type":         resp.Type,
			"download_url": resp.Data.ImageURL,
			"title":        resp.Data.Title,
		}
		outBytes, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}

func registerImageGetData(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("image_get_data",
		mcp.WithDescription("Retrieve the actual binary image data (base64-encoded) for an image item from a Miro board. Use image_get_url first if you only need the URL."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("item_id",
			mcp.Required(),
			mcp.Description("The ID of the image item on the board."),
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

		// Step 1: get image metadata to obtain the download URL.
		metaData, _, err := c.Get(ctx, fmt.Sprintf("/boards/%s/images/%s", boardID, itemID), nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var meta struct {
			Data struct {
				ImageURL string `json:"imageUrl"`
				Title    string `json:"title"`
			} `json:"data"`
		}
		if err := json.Unmarshal(metaData, &meta); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse image metadata: %v", err)), nil
		}

		if meta.Data.ImageURL == "" {
			return mcp.NewToolResultError("image has no downloadable URL"), nil
		}

		// Step 2: fetch image binary.
		rawBytes, err := c.GetRaw(ctx, meta.Data.ImageURL)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("download image: %v", err)), nil
		}

		encoded := base64.StdEncoding.EncodeToString(rawBytes)
		out := map[string]any{
			"item_id":      itemID,
			"title":        meta.Data.Title,
			"download_url": meta.Data.ImageURL,
			"data_base64":  encoded,
			"size_bytes":   len(rawBytes),
		}
		outBytes, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}
