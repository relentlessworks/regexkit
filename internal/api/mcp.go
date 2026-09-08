package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/relentlessworks/regexkit/internal/model"
)

// MCPRequest represents a Model Context Protocol request.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents a Model Context Protocol response.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPCError  `json:"error,omitempty"`
}

// MCPCError is a JSON-RPC error object.
type MCPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPTool represents a tool available to MCP clients.
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema struct {
		Type       string         `json:"type"`
		Properties map[string]MCPProp `json:"properties"`
		Required   []string       `json:"required"`
	} `json:"inputSchema"`
}

// MCPProp describes a tool input property.
type MCPProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// handleMCP serves the Model Context Protocol endpoint.
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST for MCP requests")
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, MCPResponse{
			JSONRPC: "2.0",
			Error:   &MCPCError{Code: -32700, Message: "parse error"},
		})
		return
	}

	resp := s.handleMCPMethod(req)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMCPMethod(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]string{
					"name":    "regexkit",
					"version": "0.1.0",
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]bool{"listChanged": true},
				},
			},
		}

	case "tools/list":
		tools := s.mcpTools()
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{"tools": tools},
		}

	case "tools/call":
		return s.handleMCPToolCall(req)

	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPCError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

func (s *Server) mcpTools() []MCPTool {
	var tools []MCPTool

	// test
	t := MCPTool{}
	t.Name = "test"
	t.Description = "Test a regex pattern against input text, finding all matches"
	t.InputSchema.Type = "object"
	t.InputSchema.Properties = map[string]MCPProp{
		"pattern": {Type: "string", Description: "The regex pattern to test"},
		"flags":   {Type: "string", Description: "Regex flags: i (case-insensitive), m (multiline), s (dotall)"},
		"input":   {Type: "string", Description: "The input text to test against"},
	}
	t.InputSchema.Required = []string{"pattern", "input"}
	tools = append(tools, t)

	// match
	t = MCPTool{}
	t.Name = "match"
	t.Description = "Find the first match of a regex pattern in input text"
	t.InputSchema.Type = "object"
	t.InputSchema.Properties = map[string]MCPProp{
		"pattern": {Type: "string", Description: "The regex pattern to match"},
		"flags":   {Type: "string", Description: "Regex flags: i, m, s"},
		"input":   {Type: "string", Description: "The input text to match against"},
	}
	t.InputSchema.Required = []string{"pattern", "input"}
	tools = append(tools, t)

	// replace
	t = MCPTool{}
	t.Name = "replace"
	t.Description = "Replace all matches of a regex pattern in input text"
	t.InputSchema.Type = "object"
	t.InputSchema.Properties = map[string]MCPProp{
		"pattern":     {Type: "string", Description: "The regex pattern"},
		"flags":       {Type: "string", Description: "Regex flags: i, m, s"},
		"input":       {Type: "string", Description: "The input text"},
		"replacement": {Type: "string", Description: "The replacement string"},
	}
	t.InputSchema.Required = []string{"pattern", "input", "replacement"}
	tools = append(tools, t)

	// split
	t = MCPTool{}
	t.Name = "split"
	t.Description = "Split input text on regex pattern matches"
	t.InputSchema.Type = "object"
	t.InputSchema.Properties = map[string]MCPProp{
		"pattern": {Type: "string", Description: "The regex pattern to split on"},
		"flags":   {Type: "string", Description: "Regex flags: i, m, s"},
		"input":   {Type: "string", Description: "The input text to split"},
	}
	t.InputSchema.Required = []string{"pattern", "input"}
	tools = append(tools, t)

	// validate
	t = MCPTool{}
	t.Name = "validate"
	t.Description = "Validate that a regex pattern compiles successfully"
	t.InputSchema.Type = "object"
	t.InputSchema.Properties = map[string]MCPProp{
		"pattern": {Type: "string", Description: "The regex pattern to validate"},
		"flags":   {Type: "string", Description: "Regex flags: i, m, s"},
	}
	t.InputSchema.Required = []string{"pattern"}
	tools = append(tools, t)

	return tools
}

func (s *Server) handleMCPToolCall(req MCPRequest) MCPResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPCError{Code: -32602, Message: "invalid params"},
		}
	}

	var args struct {
		Pattern     string `json:"pattern"`
		Flags       string `json:"flags"`
		Input       string `json:"input"`
		Replacement string `json:"replacement"`
	}
	if len(params.Arguments) > 0 {
		json.Unmarshal(params.Arguments, &args)
	}

	switch params.Name {
	case "test":
		result, err := s.mcpTest(args.Pattern, args.Flags, args.Input)
		if err != nil {
			return mcpError(req.ID, err)
		}
		return mcpResult(req.ID, result)

	case "match":
		result, err := s.mcpMatch(args.Pattern, args.Flags, args.Input)
		if err != nil {
			return mcpError(req.ID, err)
		}
		return mcpResult(req.ID, result)

	case "replace":
		result, err := s.mcpReplace(args.Pattern, args.Flags, args.Input, args.Replacement)
		if err != nil {
			return mcpError(req.ID, err)
		}
		return mcpResult(req.ID, result)

	case "split":
		result, err := s.mcpSplit(args.Pattern, args.Flags, args.Input)
		if err != nil {
			return mcpError(req.ID, err)
		}
		return mcpResult(req.ID, result)

	case "validate":
		result, err := s.mcpValidate(args.Pattern, args.Flags)
		if err != nil {
			return mcpError(req.ID, err)
		}
		return mcpResult(req.ID, result)

	default:
		return MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &MCPCError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}
}

func (s *Server) mcpTest(pattern, flags, input string) (interface{}, error) {
	return model.TestPattern(pattern, flags, input, true)
}

func (s *Server) mcpMatch(pattern, flags, input string) (interface{}, error) {
	return model.TestPattern(pattern, flags, input, false)
}

func (s *Server) mcpReplace(pattern, flags, input, replacement string) (interface{}, error) {
	result, count, err := model.ReplaceAll(pattern, flags, input, replacement)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"result": result, "count": count}, nil
}

func (s *Server) mcpSplit(pattern, flags, input string) (interface{}, error) {
	parts, err := model.Split(pattern, flags, input, -1)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"count": len(parts), "parts": parts}, nil
}

func (s *Server) mcpValidate(pattern, flags string) (interface{}, error) {
	err := model.ValidatePattern(pattern, flags)
	if err != nil {
		return map[string]interface{}{"valid": false, "error": err.Error()}, nil
	}
	return map[string]interface{}{"valid": true}, nil
}

func mcpResult(id interface{}, result interface{}) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": toJSONString(result)},
			},
		},
	}
}

func mcpError(id interface{}, err error) MCPResponse {
	return MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &MCPCError{Code: -32603, Message: err.Error()},
	}
}

func toJSONString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
