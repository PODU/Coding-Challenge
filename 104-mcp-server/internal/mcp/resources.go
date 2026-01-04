package mcp

import (
	"context"
	"encoding/json"
)

const (
	readmeURI = "ccshared://readme"
	infoURI   = "mcp://server/info"
)

// Resource describes a resource in resources/list.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// ResourceContent is a single content item in resources/read.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

func (s *Server) handleResourcesList(req *Request) *Response {
	if r := s.requireInitialised(req); r != nil {
		return r
	}
	resources := []Resource{
		{
			URI:         readmeURI,
			Name:        "Coding Challenges Shared Solutions README",
			Description: "The shared README listing community solutions to the Coding Challenges.",
			MimeType:    "text/markdown",
		},
		{
			URI:         infoURI,
			Name:        "Server info",
			Description: "Metadata about this MCP server.",
			MimeType:    "application/json",
		},
	}
	return resultResponse(req.ID, map[string]any{"resources": resources})
}

func (s *Server) handleResourcesRead(req *Request) *Response {
	if r := s.requireInitialised(req); r != nil {
		return r
	}
	if len(req.Params) == 0 {
		return errorResponse(req.ID, CodeInvalidParams, "invalid params: missing params")
	}
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return errorResponse(req.ID, CodeInvalidParams, "invalid params: "+err.Error())
	}

	switch p.URI {
	case readmeURI:
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		text, err := s.fetcher.Fetch(ctx)
		if err != nil {
			s.logf("resources/read fetch failed: %v", err)
			return errorResponse(req.ID, CodeInternalError, "failed to fetch README: "+err.Error())
		}
		return resultResponse(req.ID, map[string]any{
			"contents": []ResourceContent{{URI: readmeURI, MimeType: "text/markdown", Text: text}},
		})

	case infoURI:
		info := map[string]any{
			"name":            serverName,
			"version":         serverVersion,
			"protocolVersion": protocolVersion,
			"readmeURL":       DefaultReadmeURL,
		}
		b, _ := json.MarshalIndent(info, "", "  ")
		return resultResponse(req.ID, map[string]any{
			"contents": []ResourceContent{{URI: infoURI, MimeType: "application/json", Text: string(b)}},
		})

	default:
		return errorResponse(req.ID, CodeInvalidParams, "unknown resource URI: "+p.URI)
	}
}
