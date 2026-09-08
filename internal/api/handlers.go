package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/relentlessworks/regexkit/internal/auth"
	"github.com/relentlessworks/regexkit/internal/model"
	"github.com/relentlessworks/regexkit/internal/store"
)

// Server is the main API server.
type Server struct {
	auth  *auth.Auth
	store *store.Store
}

// New creates a new API server.
func New(a *auth.Auth, s *store.Store) *Server {
	return &Server{auth: a, store: s}
}

// Routes returns the HTTP handler with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Help / self-documentation
	mux.HandleFunc("/help", s.handleHelp)
	mux.HandleFunc("/.well-known/agent.md", s.handleHelp)

	// Auth
	mux.HandleFunc("/auth/request", s.handleAuthRequest)
	mux.HandleFunc("/auth/verify", s.handleAuthVerify)

	// Regex operations (auth required)
	mux.HandleFunc("/test", s.authMiddleware(s.handleTest))
	mux.HandleFunc("/match", s.authMiddleware(s.handleMatch))
	mux.HandleFunc("/replace", s.authMiddleware(s.handleReplace))
	mux.HandleFunc("/split", s.authMiddleware(s.handleSplit))
	mux.HandleFunc("/validate", s.authMiddleware(s.handleValidate))

	// Pattern CRUD (auth required)
	mux.HandleFunc("/patterns", s.authMiddleware(s.handlePatterns))

	// MCP
	mux.HandleFunc("/mcp", s.handleMCP)

	return mux
}

// handleHelp returns the operating manual.
func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	help := `# regexkit — Agentic Regex Service

## Auth
POST /auth/request  {"email":"you@example.com"}  → status=otp_sent code=XXXXXX
POST /auth/verify   {"email":"you@example.com","code":"XXXXXX"}  → token=... workspace=ws_...

All endpoints below require: Authorization: Bearer <token>

## Regex Operations
POST /test      pattern=\d+&input=hello123  → matched=true count=N matches...
POST /match     pattern=(\w+)&input=hello   → first match with groups
POST /replace   pattern=\d+&input=a1b2&replacement=X  → result=aXbX count=2
POST /split     pattern=[,;]&input=a,b;c   → count=3 [0]=a [1]=b [2]=c
POST /validate  pattern=\d+\w+               → valid=true

## Pattern CRUD
POST   /patterns  {"name":"email","pattern":"\\w+@\\w+\\.\\w+","flags":"i"}  → handle=pat_xxx
GET    /patterns                                               → list all
GET    /patterns/{handle}                                      → get one
DELETE /patterns/{handle}                                      → delete

## Flags
i=case-insensitive  m=multiline  s=dotall (dot matches newline)

## Response Format
Plain text by default. Add ?format=json or Accept: application/json for JSON.
Errors include a hint field for self-correction.
`
	writeText(w, http.StatusOK, help)
}

// handleAuthRequest generates an OTP for the given email.
func (s *Server) handleAuthRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// Try form data
		body.Email = r.FormValue("email")
	}
	if body.Email == "" {
		writeError(w, r, http.StatusBadRequest, "email is required", "provide an email field in JSON or form data")
		return
	}

	code := s.auth.RequestOTP(body.Email)
	writeRecord(w, r, "status", "otp_sent", "code", code)
}

// handleAuthVerify verifies an OTP and returns a bearer token.
func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.Email = r.FormValue("email")
		body.Code = r.FormValue("code")
	}
	if body.Email == "" || body.Code == "" {
		writeError(w, r, http.StatusBadRequest, "email and code are required", "provide both email and code fields")
		return
	}

	token, err := s.auth.VerifyOTP(body.Email, body.Code)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, err.Error(), "request a new OTP via POST /auth/request")
		return
	}

	ws := s.auth.GetWorkspace(token)
	writeRecord(w, r, "token", token, "workspace", ws)
}

// getWorkspace extracts the workspace from request headers (set by auth middleware).
func getWorkspace(r *http.Request) string {
	return r.Header.Get("X-Workspace")
}

// handleTest tests a regex pattern against input text, finding all matches.
func (s *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	pattern, flags, input := readPatternInput(r)
	if pattern == "" {
		writeError(w, r, http.StatusBadRequest, "pattern is required", "provide a pattern field")
		return
	}
	if input == "" {
		writeError(w, r, http.StatusBadRequest, "input is required", "provide an input field to test the pattern against")
		return
	}

	result, err := model.TestPattern(pattern, flags, input, true)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, fmt.Sprintf("invalid pattern: %v", err), "fix the regex syntax and try again")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, result)
		return
	}

	if !result.Matched {
		writeRecord(w, r, "matched", "false", "count", "0")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "matched=true count=%d", result.Count)
	for i, m := range result.Matches {
		fmt.Fprintf(&sb, " match[%d]=%s start=%d end=%d", i, m.Full, m.Start, m.End)
		for name, val := range m.Groups {
			fmt.Fprintf(&sb, " %s=%s", name, val)
		}
	}
	writeText(w, http.StatusOK, sb.String())
}

// handleMatch finds the first match of a pattern in input.
func (s *Server) handleMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	pattern, flags, input := readPatternInput(r)
	if pattern == "" {
		writeError(w, r, http.StatusBadRequest, "pattern is required", "provide a pattern field")
		return
	}
	if input == "" {
		writeError(w, r, http.StatusBadRequest, "input is required", "provide an input field to match against")
		return
	}

	result, err := model.TestPattern(pattern, flags, input, false)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, fmt.Sprintf("invalid pattern: %v", err), "fix the regex syntax and try again")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, result)
		return
	}

	if !result.Matched {
		writeRecord(w, r, "matched", "false")
		return
	}

	m := result.Matches[0]
	var sb strings.Builder
	fmt.Fprintf(&sb, "matched=true full=%s start=%d end=%d", m.Full, m.Start, m.End)
	for name, val := range m.Groups {
		fmt.Fprintf(&sb, " %s=%s", name, val)
	}
	writeText(w, http.StatusOK, sb.String())
}

// handleReplace replaces all matches in input with a replacement string.
func (s *Server) handleReplace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	pattern, flags, input, replacement := readReplaceInput(r)
	if pattern == "" {
		writeError(w, r, http.StatusBadRequest, "pattern is required", "provide a pattern field")
		return
	}
	if input == "" {
		writeError(w, r, http.StatusBadRequest, "input is required", "provide an input field")
		return
	}

	result, count, err := model.ReplaceAll(pattern, flags, input, replacement)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, fmt.Sprintf("invalid pattern: %v", err), "fix the regex syntax and try again")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"result": result,
			"count":  count,
		})
		return
	}

	writeRecord(w, r, "result", result, "count", fmt.Sprintf("%d", count))
}

// handleSplit splits input on pattern matches.
func (s *Server) handleSplit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	pattern, flags, input := readPatternInput(r)
	if pattern == "" {
		writeError(w, r, http.StatusBadRequest, "pattern is required", "provide a pattern field")
		return
	}
	if input == "" {
		writeError(w, r, http.StatusBadRequest, "input is required", "provide an input field to split")
		return
	}

	parts, err := model.Split(pattern, flags, input, -1)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, fmt.Sprintf("invalid pattern: %v", err), "fix the regex syntax and try again")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"count": len(parts),
			"parts": parts,
		})
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "count=%d", len(parts))
	for i, p := range parts {
		fmt.Fprintf(&sb, " [%d]=%s", i, p)
	}
	writeText(w, http.StatusOK, sb.String())
}

// handleValidate checks if a pattern compiles.
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST")
		return
	}

	pattern, flags, _ := readPatternInput(r)
	if pattern == "" {
		writeError(w, r, http.StatusBadRequest, "pattern is required", "provide a pattern field to validate")
		return
	}

	err := model.ValidatePattern(pattern, flags)
	if err != nil {
		if wantsJSON(r) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"valid": false,
				"error": err.Error(),
			})
			return
		}
		writeRecord(w, r, "valid", "false", "error", err.Error())
		return
	}

	writeRecord(w, r, "valid", "true")
}

// handlePatterns handles CRUD for saved patterns.
func (s *Server) handlePatterns(w http.ResponseWriter, r *http.Request) {
	ws := getWorkspace(r)

	// Extract handle from path if present
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/patterns/"), "/")
	handle := ""
	if len(pathParts) > 0 && pathParts[0] != "" {
		handle = pathParts[0]
	}

	switch {
	case r.Method == http.MethodPost && handle == "":
		s.createPattern(w, r, ws)
	case r.Method == http.MethodGet && handle == "":
		s.listPatterns(w, r, ws)
	case r.Method == http.MethodGet && handle != "":
		s.getPattern(w, r, ws, handle)
	case r.Method == http.MethodDelete && handle != "":
		s.deletePattern(w, r, ws, handle)
	default:
		writeError(w, r, http.StatusMethodNotAllowed, "method not allowed", "use POST to create, GET to list/get, DELETE to remove")
	}
}

func (s *Server) createPattern(w http.ResponseWriter, r *http.Request, ws string) {
	var body struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
		Flags   string `json:"flags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.Name = r.FormValue("name")
		body.Pattern = r.FormValue("pattern")
		body.Flags = r.FormValue("flags")
	}
	if body.Name == "" {
		writeError(w, r, http.StatusBadRequest, "name is required", "provide a name for the pattern")
		return
	}
	if body.Pattern == "" {
		writeError(w, r, http.StatusBadRequest, "pattern is required", "provide a regex pattern to save")
		return
	}

	// Validate the pattern compiles
	if err := model.ValidatePattern(body.Pattern, body.Flags); err != nil {
		writeError(w, r, http.StatusBadRequest, fmt.Sprintf("invalid pattern: %v", err), "fix the regex syntax and try again")
		return
	}

	p := &model.Pattern{
		Handle:    model.GenerateHandle("pat"),
		Name:      body.Name,
		Pattern:   body.Pattern,
		Flags:     body.Flags,
		Workspace: ws,
		CreatedAt: time.Now(),
	}

	if err := s.store.SavePattern(p); err != nil {
		writeError(w, r, http.StatusInternalServerError, "failed to save pattern", "try again or check the data file path")
		return
	}

	writeRecord(w, r, "handle", p.Handle, "name", p.Name, "pattern", p.Pattern, "flags", p.Flags)
}

func (s *Server) listPatterns(w http.ResponseWriter, r *http.Request, ws string) {
	patterns := s.store.ListPatterns(ws)
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, patterns)
		return
	}

	if len(patterns) == 0 {
		writeText(w, http.StatusOK, "count=0")
		return
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "count=%d", len(patterns))
	for i, p := range patterns {
		fmt.Fprintf(&sb, " [%d]=%s name=%s pattern=%s flags=%s", i, p.Handle, p.Name, p.Pattern, p.Flags)
	}
	writeText(w, http.StatusOK, sb.String())
}

func (s *Server) getPattern(w http.ResponseWriter, r *http.Request, ws, handle string) {
	p, ok := s.store.GetPattern(handle, ws)
	if !ok {
		writeError(w, r, http.StatusNotFound, "pattern not found", "check the handle or call GET /patterns to list all patterns")
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, p)
		return
	}

	writeRecord(w, r, "handle", p.Handle, "name", p.Name, "pattern", p.Pattern, "flags", p.Flags)
}

func (s *Server) deletePattern(w http.ResponseWriter, r *http.Request, ws, handle string) {
	if !s.store.DeletePattern(handle, ws) {
		writeError(w, r, http.StatusNotFound, "pattern not found", "check the handle or call GET /patterns to list all patterns")
		return
	}

	writeRecord(w, r, "status", "deleted", "handle", handle)
}

// readPatternInput extracts pattern, flags, and input from the request.
// Returns the raw body bytes so callers can read additional fields.
func readPatternInput(r *http.Request) (pattern, flags, input string) {
	bodyBytes, _ := io.ReadAll(r.Body)
	// Try JSON first
	var body struct {
		Pattern string `json:"pattern"`
		Flags   string `json:"flags"`
		Input   string `json:"input"`
	}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &body); err == nil {
			return body.Pattern, body.Flags, body.Input
		}
	}
	// Fall back to form data — re-parse body
	r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	r.ParseForm()
	return r.FormValue("pattern"), r.FormValue("flags"), r.FormValue("input")
}

// readReplaceInput extracts pattern, flags, input, and replacement from the request.
func readReplaceInput(r *http.Request) (pattern, flags, input, replacement string) {
	bodyBytes, _ := io.ReadAll(r.Body)
	var body struct {
		Pattern     string `json:"pattern"`
		Flags       string `json:"flags"`
		Input       string `json:"input"`
		Replacement string `json:"replacement"`
	}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &body); err == nil {
			return body.Pattern, body.Flags, body.Input, body.Replacement
		}
	}
	r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
	r.ParseForm()
	return r.FormValue("pattern"), r.FormValue("flags"), r.FormValue("input"), r.FormValue("replacement")
}
