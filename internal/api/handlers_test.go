package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/relentlessworks/regexkit/internal/auth"
	"github.com/relentlessworks/regexkit/internal/store"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	a := auth.New("test-secret")
	s := store.New("/tmp/regexkit-test.json")
	srv := New(a, s)

	// Get a token for testing
	code := a.RequestOTP("test@example.com")
	token, err := a.VerifyOTP("test@example.com", code)
	if err != nil {
		t.Fatalf("failed to get test token: %v", err)
	}

	return srv, token
}

func TestHelp(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	srv.handleHelp(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !contains(body, "regexkit") {
		t.Error("help should mention regexkit")
	}
	if !contains(body, "/test") {
		t.Error("help should mention /test endpoint")
	}
}

func TestAuthRequest(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"email":"newuser@example.com"}`
	req := httptest.NewRequest("POST", "/auth/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAuthRequest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !contains(resp, "otp_sent") {
		t.Error("should return otp_sent status")
	}
}

func TestAuthVerify(t *testing.T) {
	srv, _ := newTestServer(t)

	// Request OTP
	code := srv.auth.RequestOTP("verify@example.com")

	body := `{"email":"verify@example.com","code":"` + code + `"}`
	req := httptest.NewRequest("POST", "/auth/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAuthVerify(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !contains(resp, "token=") {
		t.Error("should return a token")
	}
	if !contains(resp, "workspace=") {
		t.Error("should return a workspace")
	}
}

func TestAuthVerifyBadCode(t *testing.T) {
	srv, _ := newTestServer(t)

	srv.auth.RequestOTP("badcode@example.com")

	body := `{"email":"badcode@example.com","code":"000000"}`
	req := httptest.NewRequest("POST", "/auth/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleAuthVerify(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestTestEndpoint(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"\\d+","input":"hello123world456"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleTest)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := w.Body.String()
	if !contains(resp, "matched=true") {
		t.Error("should report matched=true")
	}
	if !contains(resp, "count=2") {
		t.Error("should report count=2")
	}
}

func TestTestEndpointNoMatch(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"\\d+","input":"no digits here"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleTest)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !contains(resp, "matched=false") {
		t.Error("should report matched=false")
	}
}

func TestTestEndpointJSON(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"\\d+","input":"abc123"}`
	req := httptest.NewRequest("POST", "/test?format=json", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleTest)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON, got: %s", w.Body.String())
	}
	if result["matched"] != true {
		t.Error("JSON should have matched=true")
	}
}

func TestMatchEndpoint(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"(\\w+)@(\\w+)\\.(\\w+)","input":"user@example.com"}`
	req := httptest.NewRequest("POST", "/match", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleMatch)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := w.Body.String()
	if !contains(resp, "matched=true") {
		t.Error("should report matched=true")
	}
	if !contains(resp, "full=user@example.com") {
		t.Error("should report full match")
	}
}

func TestReplaceEndpoint(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"\\d+","input":"hello123","replacement":"NUM"}`
	req := httptest.NewRequest("POST", "/replace", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleReplace)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := w.Body.String()
	if !contains(resp, "result=helloNUM") {
		t.Error("should report result=helloNUM, got: " + resp)
	}
	if !contains(resp, "count=1") {
		t.Error("should report count=1")
	}
}

func TestSplitEndpoint(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"[,]","input":"a,b,c"}`
	req := httptest.NewRequest("POST", "/split", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleSplit)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := w.Body.String()
	if !contains(resp, "count=3") {
		t.Error("should report count=3, got: " + resp)
	}
}

func TestValidateEndpoint(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"\\d+\\w+"}`
	req := httptest.NewRequest("POST", "/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleValidate)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !contains(resp, "valid=true") {
		t.Error("should report valid=true, got: " + resp)
	}
}

func TestValidateInvalidPattern(t *testing.T) {
	srv, token := newTestServer(t)

	body := `{"pattern":"[invalid"}`
	req := httptest.NewRequest("POST", "/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleValidate)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !contains(resp, "valid=false") {
		t.Error("should report valid=false, got: " + resp)
	}
}

func TestPatternCRUD(t *testing.T) {
	srv, token := newTestServer(t)

	// Create
	body := `{"name":"email","pattern":"\\w+@\\w+\\.\\w+","flags":"i"}`
	req := httptest.NewRequest("POST", "/patterns", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handlePatterns)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("create failed: %d %s", w.Code, w.Body.String())
	}
	resp := w.Body.String()
	if !contains(resp, "handle=pat_") {
		t.Error("should return a handle, got: " + resp)
	}

	// Extract handle
	handle := extractField(resp, "handle=")

	// List
	req = httptest.NewRequest("GET", "/patterns", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.authMiddleware(srv.handlePatterns)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("list failed: %d", w.Code)
	}
	if !contains(w.Body.String(), "count=1") {
		t.Error("should list 1 pattern")
	}

	// Get
	req = httptest.NewRequest("GET", "/patterns/"+handle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.authMiddleware(srv.handlePatterns)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("get failed: %d", w.Code)
	}
	if !contains(w.Body.String(), "name=email") {
		t.Error("should return pattern with name=email")
	}

	// Delete
	req = httptest.NewRequest("DELETE", "/patterns/"+handle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.authMiddleware(srv.handlePatterns)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("delete failed: %d", w.Code)
	}
	if !contains(w.Body.String(), "status=deleted") {
		t.Error("should report status=deleted")
	}

	// Verify deleted
	req = httptest.NewRequest("GET", "/patterns/"+handle, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	srv.authMiddleware(srv.handlePatterns)(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w.Code)
	}
}

func TestAuthMiddlewareNoToken(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(`{"pattern":"\\d+","input":"123"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleTest)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareBadToken(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(`{"pattern":"\\d+","input":"123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalidtoken123")
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleTest)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMCPInitialize(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Error("should return jsonrpc 2.0")
	}
}

func TestMCPToolsList(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("missing result")
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatal("missing tools")
	}
	if len(tools) != 5 {
		t.Errorf("expected 5 tools, got %d", len(tools))
	}
}

func TestMCPToolCallTest(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"test","arguments":{"pattern":"\\d+","input":"abc123"}}}`
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %s", resp.Error.Message)
	}
}

func TestFormInput(t *testing.T) {
	srv, token := newTestServer(t)

	body := "pattern=" + urlEncode(`\d+`) + "&input=hello123world456"
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.authMiddleware(srv.handleTest)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := w.Body.String()
	if !contains(resp, "matched=true") {
		t.Error("should report matched=true, got: " + resp)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func extractField(s, prefix string) string {
	idx := indexOf(s, prefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	end := start
	for end < len(s) && s[end] != ' ' && s[end] != '\n' {
		end++
	}
	return s[start:end]
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func urlEncode(s string) string {
	return url.QueryEscape(s)
}
