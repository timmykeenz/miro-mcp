package prompts

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterPrompts adds the Miro MCP built-in prompt templates.
func RegisterPrompts(s *server.MCPServer) {
	registerCodeCreateFromBoard(s)
	registerCodeExplainOnBoard(s)
}

func registerCodeCreateFromBoard(s *server.MCPServer) {
	prompt := mcp.NewPrompt("code_create_from_board",
		mcp.WithPromptDescription("Analyze a Miro board to understand project/feature requirements, then generate comprehensive code or documentation. This uses a two-phase approach: first explore the board structure, then generate implementation based on the content found."),
		mcp.WithArgument("board_url",
			mcp.ArgumentDescription("The full URL of the Miro board to analyze (e.g. https://miro.com/app/board/uXjVJh90oMA=/)."),
			mcp.RequiredArgument(),
		),
		mcp.WithArgument("output_type",
			mcp.ArgumentDescription("What to generate: code, documentation, tests, api_spec, or architecture. Defaults to code."),
		),
		mcp.WithArgument("language",
			mcp.ArgumentDescription("Programming language or framework for code generation (e.g. Go, Python, TypeScript, React). Only needed when output_type is code."),
		),
	)

	s.AddPrompt(prompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		boardURL := req.Params.Arguments["board_url"]
		outputType := req.Params.Arguments["output_type"]
		language := req.Params.Arguments["language"]

		if outputType == "" {
			outputType = "code"
		}

		boardID := extractBoardID(boardURL)

		systemContent := `You are an expert software engineer and architect. Your task is to analyze a Miro board and generate high-quality ` + outputType + ` based on its content.

Follow this two-phase approach:

## Phase 1: Board Analysis
1. Call context_explore with the board_id to discover all high-level items (frames, documents, diagrams)
2. For each relevant item found, call context_get to read its detailed content
3. Build a complete understanding of the requirements, architecture, and design decisions on the board
4. Identify: user stories, technical requirements, data models, API contracts, UI/UX flows

## Phase 2: Generation
Based on your analysis, generate ` + outputType + `:
` + generationInstructions(outputType, language) + `

## Important rules:
- Always reference specific board content in your output (quote titles, IDs of items you used)
- If the board content is ambiguous, make reasonable assumptions and document them
- Structure output with clear headings and sections
- For code: include comments referencing the board requirements met by each section`

		userContent := `Analyze the Miro board and generate ` + outputType

		if language != "" {
			userContent += " in " + language
		}
		userContent += ".\n\n"

		if boardID != "" {
			userContent += "Board ID: " + boardID + "\n"
		}
		if boardURL != "" {
			userContent += "Board URL: " + boardURL + "\n"
		}
		userContent += "\nStart by calling context_explore with board_id=" + boardID + " to discover the board structure."

		return &mcp.GetPromptResult{
			Description: "Analyze Miro board and generate " + outputType,
			Messages: []mcp.PromptMessage{
				{
					Role:    mcp.RoleUser,
					Content: mcp.NewTextContent(systemContent + "\n\n" + userContent),
				},
			},
		}, nil
	})
}

func registerCodeExplainOnBoard(s *server.MCPServer) {
	prompt := mcp.NewPrompt("code_explain_on_board",
		mcp.WithPromptDescription("Explain code implementation by creating visual diagrams and documentation on a Miro board. Transforms code into clear visual explanations including architecture diagrams, data flows, and component relationships."),
		mcp.WithArgument("board_url",
			mcp.ArgumentDescription("The full URL of the Miro board where diagrams should be created (e.g. https://miro.com/app/board/uXjVJh90oMA=/)."),
			mcp.RequiredArgument(),
		),
		mcp.WithArgument("code_context",
			mcp.ArgumentDescription("The code, file path, or repository path to explain. Can be pasted code or a description of what to document."),
			mcp.RequiredArgument(),
		),
		mcp.WithArgument("diagram_types",
			mcp.ArgumentDescription("Comma-separated list of diagram types to create: flowchart, uml_sequence, uml_class, erd. Defaults to flowchart."),
		),
	)

	s.AddPrompt(prompt, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		boardURL := req.Params.Arguments["board_url"]
		codeContext := req.Params.Arguments["code_context"]
		diagramTypes := req.Params.Arguments["diagram_types"]

		if diagramTypes == "" {
			diagramTypes = "flowchart"
		}

		boardID := extractBoardID(boardURL)

		systemContent := `You are an expert software architect and technical writer. Your task is to analyze code and create clear visual explanations on a Miro board.

Follow this approach:

## Step 1: Understand the code
Analyze the provided code/context to identify:
- Entry points, main flows, and key components
- Data structures and their relationships
- Function call chains and dependencies
- External dependencies and integrations

## Step 2: Plan diagrams
For each requested diagram type (` + diagramTypes + `):
1. Call diagram_get_dsl with the appropriate diagram_type to understand the syntax
2. Compose the DSL for that diagram based on your code analysis

## Step 3: Create diagrams on the board
For each planned diagram:
1. Call diagram_create with the board_id and your composed DSL
2. Use appropriate x,y offsets to space diagrams so they don't overlap (offset by ~600 per diagram)

## Step 4: Add documentation
After creating diagrams, call doc_create to add a summary document on the board that:
- Describes the overall architecture
- Explains each diagram created
- Notes key design decisions visible in the code

## DSL quality guidelines:
- Keep node labels concise (< 30 chars)
- Use descriptive edge labels for complex flows
- Group related nodes spatially when possible`

		userContent := "Explain the following code by creating visual diagrams on the Miro board.\n\n"
		if boardID != "" {
			userContent += "Board ID: " + boardID + "\n"
		}
		if boardURL != "" {
			userContent += "Board URL: " + boardURL + "\n"
		}
		userContent += "\nCode/context to explain:\n```\n" + codeContext + "\n```\n\n"
		userContent += "Create diagrams of types: " + diagramTypes + "\n"
		userContent += "Start by calling diagram_get_dsl for each diagram type you plan to create."

		return &mcp.GetPromptResult{
			Description: "Create visual code explanation on Miro board",
			Messages: []mcp.PromptMessage{
				{
					Role:    mcp.RoleUser,
					Content: mcp.NewTextContent(systemContent + "\n\n" + userContent),
				},
			},
		}, nil
	})
}

// extractBoardID parses the board ID from a Miro board URL.
// Handles: https://miro.com/app/board/uXjVJh90oMA=/
func extractBoardID(boardURL string) string {
	if boardURL == "" {
		return ""
	}
	const prefix = "/board/"
	idx := len(boardURL)
	for i := range boardURL {
		if i+len(prefix) <= len(boardURL) && boardURL[i:i+len(prefix)] == prefix {
			rest := boardURL[i+len(prefix):]
			// Trim trailing slash and query params
			for j, c := range rest {
				if c == '/' || c == '?' || c == '#' {
					return rest[:j]
				}
			}
			return rest
		}
	}
	_ = idx
	return boardURL
}

func generationInstructions(outputType, language string) string {
	switch outputType {
	case "code":
		lang := language
		if lang == "" {
			lang = "the most appropriate language based on the board content"
		}
		return `Generate production-ready ` + lang + ` code:
- Include all necessary files/modules identified in the board
- Follow idiomatic conventions for ` + lang + `
- Add error handling and input validation
- Include unit test stubs for key functions
- Add a README.md with setup instructions`

	case "documentation":
		return `Generate comprehensive technical documentation:
- Architecture overview with component descriptions
- API reference for any interfaces/endpoints described
- Data flow documentation
- Setup and deployment guide
- Decision log for architectural choices`

	case "tests":
		lang := language
		if lang == "" {
			lang = "the language used in the codebase"
		}
		return `Generate a complete test suite in ` + lang + `:
- Unit tests for each component/function identified
- Integration tests for key flows shown in diagrams
- Edge cases and error scenarios
- Test data fixtures based on data models on the board`

	case "api_spec":
		return `Generate an OpenAPI 3.0 specification:
- Include all endpoints identified in the board
- Define request/response schemas from data models
- Add authentication/authorization requirements
- Include example requests and responses
- Add descriptions from the board documentation`

	case "architecture":
		return `Generate an architecture decision record (ADR) document:
- Executive summary of the system
- Component breakdown and responsibilities
- Technology choices and rationale
- Data flow and integration points
- Security considerations
- Scalability notes`

	default:
		return `Generate comprehensive output based on the board content.`
	}
}
