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

// RegisterTableTools adds table_create, table_list_rows, and table_sync_rows to the server.
func RegisterTableTools(s *server.MCPServer, c *miroclient.Client) {
	registerTableCreate(s, c)
	registerTableListRows(s, c)
	registerTableSyncRows(s, c)
}

func registerTableCreate(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("table_create",
		mcp.WithDescription("Create a table on a Miro board with specified columns. Supports text and select column types."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("columns",
			mcp.Required(),
			mcp.Description(`JSON array of column definitions. Each column has a "name" (string) and "type" ("text" or "select"). Example: [{"name":"Status","type":"select"},{"name":"Notes","type":"text"}]`),
		),
		mcp.WithString("title",
			mcp.Description("Optional title for the table."),
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
		columnsJSON, err := req.RequireString("columns")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var columns []map[string]any
		if err := json.Unmarshal([]byte(columnsJSON), &columns); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse columns JSON: %v", err)), nil
		}

		x := req.GetFloat("x", 0.0)
		y := req.GetFloat("y", 0.0)

		body := map[string]any{
			"data": map[string]any{
				"columns": columns,
			},
			"position": map[string]any{
				"x":      x,
				"y":      y,
				"origin": "center",
			},
		}

		if title := req.GetString("title", ""); title != "" {
			body["data"].(map[string]any)["title"] = title
		}

		data, _, err := c.Post(ctx, fmt.Sprintf("/boards/%s/data_table_format_items", boardID), body)
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

func registerTableListRows(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("table_list_rows",
		mcp.WithDescription("Get rows from a Miro table with column metadata. Supports filtering by column value and cursor-based pagination."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("item_id",
			mcp.Required(),
			mcp.Description("The ID of the data table item on the board."),
		),
		mcp.WithString("cursor",
			mcp.Description("Pagination cursor from a previous response."),
		),
		mcp.WithString("filter_column",
			mcp.Description("Column name to filter by (use together with filter_value)."),
		),
		mcp.WithString("filter_value",
			mcp.Description("Value to filter rows by (requires filter_column)."),
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

		params := url.Values{}
		if cursor := req.GetString("cursor", ""); cursor != "" {
			params.Set("cursor", cursor)
		}

		data, _, err := c.Get(ctx, fmt.Sprintf("/boards/%s/data_table_format_items/%s/rows", boardID, itemID), params)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var resp map[string]any
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultText(string(data)), nil
		}

		// Apply client-side filter if requested (the Miro API may not support server-side filtering).
		filterCol := req.GetString("filter_column", "")
		filterVal := req.GetString("filter_value", "")
		hasFilterCol := filterCol != ""
		hasFilterVal := filterVal != ""
		if hasFilterCol && hasFilterVal && filterCol != "" && filterVal != "" {
			if rows, ok := resp["data"].([]any); ok {
				filtered := []any{}
				for _, row := range rows {
					rowMap, ok := row.(map[string]any)
					if !ok {
						continue
					}
					cells, _ := rowMap["cells"].([]any)
					for _, cell := range cells {
						cellMap, ok := cell.(map[string]any)
						if !ok {
							continue
						}
						col, _ := cellMap["columnName"].(string)
						val, _ := cellMap["value"].(string)
						if col == filterCol && val == filterVal {
							filtered = append(filtered, row)
							break
						}
					}
				}
				resp["data"] = filtered
				resp["filtered_count"] = len(filtered)
			}
		}

		outBytes, _ := json.MarshalIndent(resp, "", "  ")
		return mcp.NewToolResultText(string(outBytes)), nil
	})
}

func registerTableSyncRows(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("table_sync_rows",
		mcp.WithDescription("Add new rows or update existing rows in a Miro table using key-based upsert matching. Rows with a matching key column value are updated; others are added."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("item_id",
			mcp.Required(),
			mcp.Description("The ID of the data table item on the board."),
		),
		mcp.WithString("key_column",
			mcp.Required(),
			mcp.Description("Name of the column to use as the upsert key for matching existing rows."),
		),
		mcp.WithString("rows",
			mcp.Required(),
			mcp.Description(`JSON array of rows to sync. Each row is an object mapping column names to values. Example: [{"Name":"Alice","Status":"Active"},{"Name":"Bob","Status":"Inactive"}]`),
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
		keyColumn, err := req.RequireString("key_column")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		rowsJSON, err := req.RequireString("rows")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var incomingRows []map[string]any
		if err := json.Unmarshal([]byte(rowsJSON), &incomingRows); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse rows JSON: %v", err)), nil
		}

		body := map[string]any{
			"keyColumn": keyColumn,
			"rows":      incomingRows,
		}

		data, _, err := c.Patch(ctx, fmt.Sprintf("/boards/%s/data_table_format_items/%s/rows", boardID, itemID), body)
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
