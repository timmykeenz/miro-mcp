# miro-mcp — Local Miro MCP Server (Go)

A locally runnable [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server written in Go that exposes Miro board capabilities to AI coding tools such as GitHub Copilot in VS Code.

---

## Tools & Prompts

| Name | Type | Description |
|---|---|---|
| `board_list_items` | Tool | List items on a board with cursor pagination and type filter |
| `context_explore` | Tool | Discover high-level items (frames, docs, diagrams) on a board |
| `context_get` | Tool | Get text content from a specific board item |
| `diagram_create` | Tool | Create a diagram on a board from DSL text (flowchart supported) |
| `diagram_get_dsl` | Tool | Get DSL syntax spec for flowchart, uml_class, uml_sequence, or erd |
| `doc_create` | Tool | Create a Miro doc (markdown) on a board |
| `doc_get` | Tool | Read content of a Miro doc item |
| `doc_update` | Tool | Find-and-replace content in a Miro doc |
| `image_get_url` | Tool | Get the download URL for an image item |
| `image_get_data` | Tool | Fetch base64-encoded binary data for an image item |
| `table_create` | Tool | Create a table on a board with named columns |
| `table_list_rows` | Tool | List rows from a table with optional column filter |
| `table_sync_rows` | Tool | Upsert rows into a table using a key column |
| `code_create_from_board` | Prompt | Analyze a board and generate code/docs from its content |
| `code_explain_on_board` | Prompt | Create visual diagrams on a board from code |

---

## Prerequisites

- **Go 1.21 or later** — [Download](https://go.dev/dl/)
- **Git**
- **VS Code** with the **GitHub Copilot** extension (v1.0+)

---

## Miro-Side Setup — Getting Your Access Token

You need a Miro Developer App with an access token scoped to your team.

### Step 1 — Create a Developer App

1. Log in to [miro.com](https://miro.com).
2. Click your profile avatar (top-right) → **Settings** → **Your apps** (left sidebar).
3. Click **Create new app**.
4. Fill in a name (e.g. `Local MCP Server`) and click **Create app**.

### Step 2 — Configure Scopes

On your new app page:

1. Under **Redirect URI for OAuth2.0**, enter `http://localhost` (required by the form even though you won't use the full OAuth flow).
2. Under **Scopes**, enable **all** of:
   - `boards:read`
   - `boards:write`
   - `identity:read`  *(needed for context_get on some item types)*
3. Click **Save**.

### Step 3 — Install the App and Get the Token

1. Scroll down to **Install app and get OAuth token**.
2. Click the button, then select the Miro **team** that contains the boards you want to work with.

   > **Important:** The token is team-scoped. You can only access boards that belong to the team you select here. If you need to switch teams, repeat this step.

3. Click **Add**.
4. Copy the **Access token** shown on the page — this is your `MIRO_ACCESS_TOKEN`.

   > Treat this token as a secret. Do **not** commit it to source control.

---

## Build

```bash
cd c:\wksp\miro-mcp

# Download dependencies
go mod tidy

# Build for Windows (produces miro-mcp.exe)
go build -o miro-mcp.exe .

# Build for Linux/macOS (produces miro-mcp)
# go build -o miro-mcp .
```

---

## VS Code Setup

### Step 1 — Set the environment variable

Set `MIRO_ACCESS_TOKEN` in your user environment **before** launching VS Code:

**Windows (Command Prompt, persistent):**
```cmd
setx MIRO_ACCESS_TOKEN "your-token-here"
```
Then restart your terminal/VS Code for it to take effect.

**Windows (current session only):**
```cmd
set MIRO_ACCESS_TOKEN=your-token-here
code .
```

**Linux / macOS:**
```bash
export MIRO_ACCESS_TOKEN=your-token-here
code .
```

### Step 2 — Open the workspace

```bash
cd c:\wksp\miro-mcp
code .
```

The `.vscode/mcp.json` file in this repo already configures VS Code to spawn `miro-mcp.exe` as a local stdio MCP server.

### Step 3 — Verify in Copilot

1. Open the **GitHub Copilot Chat** panel.
2. Switch to **Agent** mode (the dropdown next to the chat input, select `Agent`).
3. Click the **tools icon** (hammer icon) in the chat toolbar.
4. You should see all 13 `miro-mcp` tools listed.

---

## Running Locally (Manual Test)

You can verify the server is working without VS Code by piping JSON-RPC directly:

```bash
echo ^{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1"}}} | .\miro-mcp.exe
```

Expected output: an `initialize` response with `serverInfo.name = "miro-mcp"` and the list of capabilities.

### Using MCP Inspector (recommended)

[MCP Inspector](https://github.com/modelcontextprotocol/inspector) provides a visual UI to test all tools:

```bash
npx @modelcontextprotocol/inspector .\miro-mcp.exe
```

Open the URL shown in the output. You'll see all tools and can invoke them interactively.

---

## Usage Examples

### List items on a board
```
Use board_list_items with board_id="uXjVJh90oMA=" to list the first 10 items
```

### Explore a board's structure
```
Use context_explore with board_id="uXjVJh90oMA=" to see all frames and documents
```

### Create a flowchart from code
```
Use the code_explain_on_board prompt with board_url="https://miro.com/app/board/uXjVJh90oMA=/" 
and code_context="<paste your code here>"
```

### Create a sequence diagram
```
First use diagram_get_dsl with diagram_type="uml_sequence" to see the syntax,
then use diagram_create with board_id, diagram_type="flowchart", and your dsl_text
```

### Summarize a board
```
Use context_explore on board "uXjVJh90oMA=", then use context_get on each frame 
to read the content, then summarize everything
```

---

## Finding Your Board ID

The board ID is in the Miro board URL:
```
https://miro.com/app/board/uXjVJh90oMA=/
                             ^^^^^^^^^^^
                             This is your board_id
```

---

## Architecture

```
VS Code (Copilot Agent)
    │  stdin/stdout (JSON-RPC 2.0)
    ▼
miro-mcp.exe  (this binary)
    │  HTTPS + Bearer token
    ▼
api.miro.com/v2
```

The server uses the `stdio` MCP transport: VS Code spawns it as a child process on startup and communicates over stdin/stdout. No listening port is required. The process is lightweight and exits when VS Code closes.

---

## Security Notes

- The access token is never written to disk by this server — it is read once from the environment at startup.
- All requests to Miro's API are made over HTTPS.
- The server has no inbound network surface — it only connects outbound to `api.miro.com`.
- The `.gitignore` excludes the compiled binary to prevent accidentally committing it.

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `MIRO_ACCESS_TOKEN environment variable is not set` | Set the env var before launching VS Code. See VS Code Setup above. |
| `miro API error 401` | Token is invalid or expired. Re-generate it on your Miro app settings page. |
| `miro API error 403` | Your token doesn't have the required scope, or the board belongs to a different team than the one you authorized. Re-install the app on the correct team. |
| `miro API error 404` | The board ID or item ID doesn't exist, or the board belongs to a different team. |
| Tools not appearing in Copilot | Ensure the binary is built (`miro-mcp.exe` exists), `MIRO_ACCESS_TOKEN` is set, and VS Code was launched after setting the env var. Check Output > GitHub Copilot for error messages. |
| `command not found` or binary not executable | Run `go build -o miro-mcp.exe .` first. On Linux/macOS run `chmod +x miro-mcp`. |
