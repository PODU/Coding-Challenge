package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const fetchTimeout = 15 * time.Second

// Content is a single content block in a tool result.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult is the result of tools/call. Tool failures set IsError
// instead of returning a JSON-RPC error, so the model gets to see them.
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError"`
}

// Tool is a registered tool. Handlers report tool failures in the result and
// return an error only for protocol problems (*paramError for bad arguments).
type Tool struct {
	Name        string                                                                  `json:"name"`
	Description string                                                                  `json:"description"`
	InputSchema json.RawMessage                                                         `json:"inputSchema"`
	Handler     func(ctx context.Context, args json.RawMessage) (CallToolResult, error) `json:"-"`
}

func textResult(text string, isError bool) CallToolResult {
	return CallToolResult{Content: []Content{{Type: "text", Text: text}}, IsError: isError}
}

func (s *Server) buildTools() map[string]Tool {
	return map[string]Tool{
		"hello": {
			Name:        "hello",
			Description: "Returns a friendly greeting of the form \"Hello, <name>\".",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"The name to greet."}},"required":["name"]}`),
			Handler:     helloHandler,
		},
		"CodingChallengesSolutionFinder": {
			Name:        "CodingChallengesSolutionFinder",
			Description: "Searches the Coding Challenges Shared Solutions README for challenge solutions matching a query, optionally filtered by programming language, and returns the matching links.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Challenge name or number to search for, e.g. \"wc\" or \"104\"."},"language":{"type":"string","description":"Optional programming language filter, e.g. \"Go\" or \"Python\"."}},"required":["query"]}`),
			Handler:     s.finderHandler,
		},
	}
}

// handleToolsList lists tools sorted by name.
func (s *Server) handleToolsList(req *Request) *Response {
	if r := s.requireInitialised(req); r != nil {
		return r
	}
	names := make([]string, 0, len(s.tools))
	for n := range s.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	tools := make([]Tool, 0, len(names))
	for _, n := range names {
		tools = append(tools, s.tools[n])
	}
	return resultResponse(req.ID, map[string]any{"tools": tools})
}

func (s *Server) handleToolsCall(req *Request) *Response {
	if r := s.requireInitialised(req); r != nil {
		return r
	}
	if len(req.Params) == 0 {
		return errorResponse(req.ID, CodeInvalidParams, "invalid params: missing params")
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return errorResponse(req.ID, CodeInvalidParams, "invalid params: "+err.Error())
	}
	tool, ok := s.tools[p.Name]
	if !ok {
		return errorResponse(req.ID, CodeInvalidParams, "unknown tool: "+p.Name)
	}
	args := p.Arguments
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}

	result, err := tool.Handler(context.Background(), args)
	if err != nil {
		var pe *paramError
		if errors.As(err, &pe) {
			return errorResponse(req.ID, CodeInvalidParams, pe.msg)
		}
		s.logf("tool %q internal error: %v", p.Name, err)
		return errorResponse(req.ID, CodeInternalError, err.Error())
	}
	return resultResponse(req.ID, result)
}

func helloHandler(_ context.Context, args json.RawMessage) (CallToolResult, error) {
	var a struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return CallToolResult{}, &paramError{"invalid arguments: " + err.Error()}
	}
	if strings.TrimSpace(a.Name) == "" {
		return CallToolResult{}, &paramError{"missing required argument: name"}
	}
	return textResult("Hello, "+a.Name, false), nil
}

func (s *Server) finderHandler(ctx context.Context, args json.RawMessage) (CallToolResult, error) {
	var a struct {
		Query    string `json:"query"`
		Language string `json:"language"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return CallToolResult{}, &paramError{"invalid arguments: " + err.Error()}
	}
	if strings.TrimSpace(a.Query) == "" {
		return CallToolResult{}, &paramError{"missing required argument: query"}
	}

	fctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	readme, err := s.fetcher.Fetch(fctx)
	if err != nil {
		s.logf("fetch failed: %v", err)
		return textResult("Failed to fetch the Coding Challenges README: "+err.Error(), true), nil
	}

	all := parseSolutions(readme)
	if len(all) == 0 {
		return textResult("No solution links could be parsed from the README. The upstream format may have changed.", true), nil
	}

	matches := filterSolutions(all, a.Query, a.Language)
	if len(matches) == 0 {
		msg := fmt.Sprintf("No matching challenges for %q", a.Query)
		if strings.TrimSpace(a.Language) != "" {
			msg += fmt.Sprintf(" in language %q", a.Language)
		}
		msg += "."
		return textResult(msg, false), nil
	}

	return textResult(formatSolutions(a.Query, a.Language, matches), false), nil
}

// formatSolutions renders matches as markdown, grouped by challenge in README
// order.
func formatSolutions(query, lang string, sols []Solution) string {
	var b strings.Builder
	header := fmt.Sprintf("Found %d solution(s) matching %q", len(sols), query)
	if strings.TrimSpace(lang) != "" {
		header += fmt.Sprintf(" in %s", lang)
	}
	b.WriteString(header)
	b.WriteString(":\n")

	var order []string
	groups := map[string][]Solution{}
	for _, s := range sols {
		if _, ok := groups[s.Challenge]; !ok {
			order = append(order, s.Challenge)
		}
		groups[s.Challenge] = append(groups[s.Challenge], s)
	}
	for _, ch := range order {
		b.WriteString("\n")
		b.WriteString(ch)
		b.WriteString("\n")
		for _, s := range groups[ch] {
			fmt.Fprintf(&b, "- [%s](%s)\n", s.Language, s.URL)
		}
	}
	return b.String()
}
