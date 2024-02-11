package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"datavault/internal/vault"
)

type tokenizeOptions struct {
	FormatPreserving bool `json:"format_preserving"`
	Deterministic    bool `json:"deterministic"`
}

type tokenizeRequest struct {
	ID      string            `json:"id"`
	Data    map[string]string `json:"data"`
	Options tokenizeOptions   `json:"options"`
}

type tokenizeResponse struct {
	ID   string            `json:"id"`
	Data map[string]string `json:"data"`
}

type detokenizeRequest struct {
	Tokens []string `json:"tokens"`
}

type detokResult struct {
	Found bool   `json:"found"`
	Value string `json:"value,omitempty"`
}

type detokenizeResponse struct {
	Results map[string]detokResult `json:"results"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: errorBody{
		Code:      code,
		Message:   message,
		RequestID: requestIDFrom(r.Context()),
	}})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.vault.Ping(r.Context()); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "store not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// decodeJSON decodes the body into dst, rejecting the wrong content type,
// oversized bodies and unknown fields. On failure it has already written the
// error response.
func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	ct := r.Header.Get("Content-Type")
	if mt := strings.TrimSpace(strings.Split(ct, ";")[0]); !strings.EqualFold(mt, "application/json") {
		writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return errHandled
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, r, http.StatusBadRequest, "body_too_large", "request body exceeds limit")
			return errHandled
		}
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return errHandled
	}
	if dec.More() {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "unexpected trailing data in request body")
		return errHandled
	}
	return nil
}

// errHandled means the error response has already been written.
var errHandled = errors.New("handled")

func (s *Server) handleTokenize(w http.ResponseWriter, r *http.Request) {
	var req tokenizeRequest
	if err := s.decodeJSON(w, r, &req); err != nil {
		return
	}
	if len(req.Data) == 0 {
		writeError(w, r, http.StatusBadRequest, "missing_data", "request must include a non-empty \"data\" object")
		return
	}

	opts := vault.Options{
		FormatPreserving: req.Options.FormatPreserving,
		Deterministic:    req.Options.Deterministic,
	}
	tokens, err := s.vault.Tokenize(r.Context(), req.ID, req.Data, opts, requestIDFrom(r.Context()))
	if err != nil {
		if errors.Is(err, vault.ErrValueTooLarge) {
			writeError(w, r, http.StatusBadRequest, "value_too_large", "a field value exceeds the maximum size")
			return
		}
		s.logger.Error("tokenize failed", "request_id", requestIDFrom(r.Context()), "err", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, tokenizeResponse{ID: req.ID, Data: tokens})
}

func (s *Server) handleDetokenize(w http.ResponseWriter, r *http.Request) {
	var req detokenizeRequest
	if err := s.decodeJSON(w, r, &req); err != nil {
		return
	}
	if len(req.Tokens) == 0 {
		writeError(w, r, http.StatusBadRequest, "missing_tokens", "request must include a non-empty \"tokens\" array")
		return
	}

	results, err := s.vault.Detokenize(r.Context(), req.Tokens)
	if err != nil {
		s.logger.Error("detokenize failed", "request_id", requestIDFrom(r.Context()), "err", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	out := make(map[string]detokResult, len(results))
	for tok, res := range results {
		out[tok] = detokResult{Found: res.Found, Value: res.Value}
	}
	writeJSON(w, http.StatusOK, detokenizeResponse{Results: out})
}
