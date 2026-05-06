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

// RegisterDiagramTools adds diagram_get_dsl and diagram_create to the server.
func RegisterDiagramTools(s *server.MCPServer, c *miroclient.Client) {
	registerDiagramGetDSL(s)
	registerDiagramCreate(s, c)
}

func registerDiagramGetDSL(s *server.MCPServer) {
	tool := mcp.NewTool("diagram_get_dsl",
		mcp.WithDescription("Get the DSL format specification (syntax rules, color guidelines, and examples) for a diagram type. Call this before diagram_create to understand the required DSL format."),
		mcp.WithString("diagram_type",
			mcp.Required(),
			mcp.Description("The type of diagram: flowchart, uml_class, uml_sequence, or erd."),
			mcp.Enum("flowchart", "uml_class", "uml_sequence", "erd"),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagType, err := req.RequireString("diagram_type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		specs := map[string]string{
			"flowchart": `Flowchart DSL (Mermaid-compatible subset)
Syntax rules:
  - Start with: flowchart TD  (TD=top-down, LR=left-right, BT=bottom-top, RL=right-left)
  - Node declarations: nodeId[Label text]
  - Node shapes: [rect], (rounded), {diamond/decision}, ((circle)), ([stadium])
  - Edges: A --> B          (arrow)
           A --- B          (line)
           A -->|label| B   (labeled arrow)
  - Subgraphs: subgraph Title \n ... \n end
  - Colors: style nodeId fill:#f9f,stroke:#333

Example:
flowchart TD
    A[Start] --> B{Decision?}
    B -->|Yes| C[Process A]
    B -->|No| D[Process B]
    C --> E[End]
    D --> E`,

			"uml_class": `UML Class Diagram DSL (Mermaid-compatible subset)
Syntax rules:
  - Start with: classDiagram
  - Class declaration: class ClassName { \n  +field type \n  +method() returnType \n }
  - Visibility: + public, - private, # protected, ~ package
  - Relationships:
      ClassA --|> ClassB    (inheritance)
      ClassA --* ClassB     (composition)
      ClassA --o ClassB     (aggregation)
      ClassA --> ClassB     (association)
      ClassA ..> ClassB     (dependency)
      ClassA ..|> ClassB    (realization/interface)
  - Cardinality: ClassA "1" --> "0..*" ClassB : label

Example:
classDiagram
    class Animal {
        +String name
        +int age
        +speak() void
    }
    class Dog {
        +String breed
        +fetch() void
    }
    Animal --|> Dog`,

			"uml_sequence": `UML Sequence Diagram DSL (Mermaid-compatible subset)
Syntax rules:
  - Start with: sequenceDiagram
  - Participant: participant A as Alice
  - Messages:
      A->>B: message          (async arrow)
      A->B: message           (open arrow)
      A-->>B: message         (dashed)
      A->>+B: message         (activate B)
      B-->>-A: response       (deactivate B)
  - Notes: Note right of A: text
  - Loops: loop condition \n ... \n end
  - Alt: alt condition \n ... \n else \n ... \n end

Example:
sequenceDiagram
    participant C as Client
    participant S as Server
    participant D as Database
    C->>+S: GET /api/data
    S->>+D: SELECT * FROM items
    D-->>-S: rows
    S-->>-C: 200 JSON response`,

			"erd": `Entity Relationship Diagram DSL (Mermaid-compatible subset)
Syntax rules:
  - Start with: erDiagram
  - Entity: ENTITY { \n  type fieldName [PK|FK|UK] "description" \n }
  - Relationships:
      ENTITY1 ||--|| ENTITY2 : "relationship label"
      Cardinality: || (exactly one), |{ (one or more), }| (zero or one), }{ (zero or more)
  - Types: string, int, float, boolean, date, datetime

Example:
erDiagram
    CUSTOMER {
        int id PK
        string name
        string email UK
    }
    ORDER {
        int id PK
        int customer_id FK
        date created_at
    }
    CUSTOMER ||--o{ ORDER : "places"`,
		}

		spec, ok := specs[diagType]
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unknown diagram type: %s", diagType)), nil
		}
		return mcp.NewToolResultText(spec), nil
	})
}

// ---- diagram_create --------------------------------------------------------

type shapeCreateRequest struct {
	Data     shapeData     `json:"data"`
	Style    shapeStyle    `json:"style"`
	Position shapePosition `json:"position"`
	Geometry shapeGeometry `json:"geometry"`
}

type shapeData struct {
	Shape   string `json:"shape"`
	Content string `json:"content"`
}

type shapeStyle struct {
	FillColor   string `json:"fillColor"`
	StrokeColor string `json:"strokeColor"`
	TextColor   string `json:"textColor"`
}

type shapePosition struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Origin string  `json:"origin"`
}

type shapeGeometry struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type connectorCreateRequest struct {
	StartItem connectorEndpoint  `json:"startItem"`
	EndItem   connectorEndpoint  `json:"endItem"`
	Style     connectorStyle     `json:"style"`
	Captions  []connectorCaption `json:"captions,omitempty"`
}

type connectorEndpoint struct {
	ID string `json:"id"`
}

type connectorStyle struct {
	StrokeColor  string `json:"strokeColor"`
	EndStrokeCap string `json:"endStrokeCap"`
}

type connectorCaption struct {
	Content  string  `json:"content"`
	Position float64 `json:"position"`
}

func registerDiagramCreate(s *server.MCPServer, c *miroclient.Client) {
	tool := mcp.NewTool("diagram_create",
		mcp.WithDescription("Create a diagram on a Miro board from DSL text. Supports flowchart, uml_class, uml_sequence, and erd diagram types. Call diagram_get_dsl first to get the correct syntax."),
		mcp.WithString("board_id",
			mcp.Required(),
			mcp.Description("The Miro board ID."),
		),
		mcp.WithString("diagram_type",
			mcp.Required(),
			mcp.Description("Type of diagram to create: flowchart, uml_class, uml_sequence, or erd."),
			mcp.Enum("flowchart", "uml_class", "uml_sequence", "erd"),
		),
		mcp.WithString("dsl_text",
			mcp.Required(),
			mcp.Description("The diagram DSL text. Use diagram_get_dsl to get the correct format for your diagram type."),
		),
		mcp.WithNumber("x",
			mcp.Description("X position on the board for the top-left of the diagram. Defaults to 0."),
		),
		mcp.WithNumber("y",
			mcp.Description("Y position on the board for the top-left of the diagram. Defaults to 0."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boardID, err := req.RequireString("board_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		diagType, err := req.RequireString("diagram_type")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		dslText, err := req.RequireString("dsl_text")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		baseX := req.GetFloat("x", 0.0)
		baseY := req.GetFloat("y", 0.0)

		switch diagType {
		case "flowchart":
			return createFlowChart(ctx, c, boardID, dslText, baseX, baseY)
		default:
			return mcp.NewToolResultError(fmt.Sprintf(
				"diagram type '%s' is not yet supported for auto-rendering. Supported: flowchart. "+
					"For other types, consider creating nodes manually using board shapes.",
				diagType,
			)), nil
		}
	})
}

// createFlowChart parses a minimal Mermaid flowchart DSL and creates shapes + connectors.
func createFlowChart(ctx context.Context, c *miroclient.Client, boardID, dsl string, baseX, baseY float64) (*mcp.CallToolResult, error) {
	nodes, edges, err := parseFlowchart(dsl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("parse flowchart DSL: %v", err)), nil
	}

	nodeWidth := 200.0
	nodeHeight := 80.0
	hSpacing := 280.0
	vSpacing := 160.0
	nodesPerRow := 4

	createdIDs := map[string]string{} // DSL node id → Miro item id

	for i, n := range nodes {
		row := i / nodesPerRow
		col := i % nodesPerRow
		x := baseX + float64(col)*hSpacing
		y := baseY + float64(row)*vSpacing

		shape := "rectangle"
		fillColor := "#ffffff"
		strokeColor := "#1a1a2e"
		switch n.shape {
		case "diamond":
			shape = "rhombus"
			fillColor = "#fff3cd"
		case "rounded":
			shape = "round_rectangle"
			fillColor = "#d1ecf1"
		case "circle":
			shape = "circle"
			fillColor = "#d4edda"
		}

		body := shapeCreateRequest{
			Data:     shapeData{Shape: shape, Content: n.label},
			Style:    shapeStyle{FillColor: fillColor, StrokeColor: strokeColor, TextColor: "#1a1a2e"},
			Position: shapePosition{X: x, Y: y, Origin: "center"},
			Geometry: shapeGeometry{Width: nodeWidth, Height: nodeHeight},
		}

		data, _, err := c.Post(ctx, fmt.Sprintf("/boards/%s/shapes", boardID), body)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("create shape '%s': %v", n.id, err)), nil
		}

		var resp struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("parse shape response: %v", err)), nil
		}
		createdIDs[n.id] = resp.ID
	}

	connectorIDs := []string{}
	for _, e := range edges {
		fromMiroID, ok1 := createdIDs[e.from]
		toMiroID, ok2 := createdIDs[e.to]
		if !ok1 || !ok2 {
			continue
		}

		body := connectorCreateRequest{
			StartItem: connectorEndpoint{ID: fromMiroID},
			EndItem:   connectorEndpoint{ID: toMiroID},
			Style:     connectorStyle{StrokeColor: "#1a1a2e", EndStrokeCap: "arrow"},
		}
		if e.label != "" {
			body.Captions = []connectorCaption{{Content: e.label, Position: 0.5}}
		}

		data, _, err := c.Post(ctx, fmt.Sprintf("/boards/%s/connectors", boardID), body)
		if err != nil {
			continue // non-fatal: connector errors are best-effort
		}
		var cr struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(data, &cr); err == nil {
			connectorIDs = append(connectorIDs, cr.ID)
		}
	}

	result := map[string]any{
		"status":        "created",
		"nodes_created": len(createdIDs),
		"edges_created": len(connectorIDs),
		"board_url":     fmt.Sprintf("https://miro.com/app/board/%s/", boardID),
	}
	outBytes, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(outBytes)), nil
}

// ---- minimal Mermaid flowchart parser --------------------------------------

type flowNode struct {
	id    string
	label string
	shape string // rectangle, diamond, rounded, circle
}

type flowEdge struct {
	from  string
	to    string
	label string
}

func parseFlowchart(dsl string) ([]flowNode, []flowEdge, error) {
	lines := strings.Split(dsl, "\n")
	nodeMap := map[string]*flowNode{}
	var nodes []flowNode
	var edges []flowEdge

	edgeChars := []string{"-->", "---", "-.->", "==>"}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		// Skip directive lines
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "flowchart") ||
			strings.HasPrefix(lower, "graph") ||
			strings.HasPrefix(lower, "subgraph") ||
			line == "end" {
			continue
		}

		// Detect edge lines
		edgeFound := ""
		for _, sep := range edgeChars {
			if strings.Contains(line, sep) {
				edgeFound = sep
				break
			}
		}

		if edgeFound != "" {
			// Example: A -->|label| B  or  A --> B
			parts := strings.SplitN(line, edgeFound, 2)
			if len(parts) != 2 {
				continue
			}
			fromRaw := strings.TrimSpace(parts[0])
			rest := strings.TrimSpace(parts[1])

			// Extract optional label: |label|
			edgeLabel := ""
			if strings.HasPrefix(rest, "|") {
				end := strings.Index(rest[1:], "|")
				if end >= 0 {
					edgeLabel = rest[1 : end+1]
					rest = strings.TrimSpace(rest[end+2:])
				}
			}

			fromID, fromLabel, fromShape := parseNodeDecl(fromRaw)
			toID, toLabel, toShape := parseNodeDecl(rest)

			ensureNode(fromID, fromLabel, fromShape, nodeMap, &nodes)
			ensureNode(toID, toLabel, toShape, nodeMap, &nodes)
			edges = append(edges, flowEdge{from: fromID, to: toID, label: edgeLabel})
			continue
		}

		// Standalone node declaration
		if strings.ContainsAny(line, "[({") {
			id, label, shape := parseNodeDecl(line)
			ensureNode(id, label, shape, nodeMap, &nodes)
		}
	}

	if len(nodes) == 0 {
		return nil, nil, fmt.Errorf("no nodes found in flowchart DSL")
	}
	return nodes, edges, nil
}

func parseNodeDecl(s string) (id, label, shape string) {
	s = strings.TrimSpace(s)
	// Try common patterns: id[text], id(text), id{text}, id((text)), id([text])
	bracketPairs := []struct {
		open, close string
		shapeType   string
	}{
		{"((", "))", "circle"},
		{"([", "])", "rounded"},
		{"{", "}", "diamond"},
		{"(", ")", "rounded"},
		{"[", "]", "rectangle"},
	}

	for _, bp := range bracketPairs {
		oi := strings.Index(s, bp.open)
		ci := strings.LastIndex(s, bp.close)
		if oi >= 0 && ci > oi {
			id = strings.TrimSpace(s[:oi])
			label = strings.TrimSpace(s[oi+len(bp.open) : ci])
			shape = bp.shapeType
			if id == "" {
				id = label
			}
			return
		}
	}

	// Plain identifier
	id = s
	label = s
	shape = "rectangle"
	return
}

func ensureNode(id, label, shape string, nodeMap map[string]*flowNode, nodes *[]flowNode) {
	if id == "" {
		return
	}
	if _, exists := nodeMap[id]; !exists {
		n := &flowNode{id: id, label: label, shape: shape}
		nodeMap[id] = n
		*nodes = append(*nodes, *n)
	} else if label != id && nodeMap[id].label == id {
		// Update label if previously only had the ID
		nodeMap[id].label = label
	}
}
