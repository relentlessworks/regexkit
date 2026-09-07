package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Auth struct {
	mu        sync.RWMutex
	secret    string
	otps      map[string]string // email -> code
	tokens    map[string]string // token -> email
	workspaces map[string]string // token -> workspace
}

func New(secret string) *Auth {
	if secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
	}
	return &Auth{
		secret:    secret,
		otps:      make(map[string]string),
		tokens:    make(map[string]string),
		workspaces: make(map[string]string),
	}
}

// RequestOTP generates a 6-digit OTP for the given email
func (a *Auth) RequestOTP(email string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	a.otps[email] = code
	return code
}

// VerifyOTP validates the OTP and returns a bearer token
func (a *Auth) VerifyOTP(email, code string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	saved, ok := a.otps[email]
	if !ok {
		return "", fmt.Errorf("no OTP requested for this email")
	}
	if subtle.ConstantTimeCompare([]byte(saved), []byte(code)) != 1 {
		return "", fmt.Errorf("invalid OTP code")
	}
	delete(a.otps, email)

	token := generateToken()
	a.tokens[token] = email
	ws := generateWorkspace()
	a.workspaces[token] = ws
	return token, nil
}

// ValidateToken checks if a bearer token is valid
func (a *Auth) ValidateToken(token string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	email, ok := a.tokens[token]
	return email, ok
}

// GetWorkspace returns the workspace for a token
func (a *Auth) GetWorkspace(token string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.workspaces[token]
}

// ExtractToken gets the bearer token from the Authorization header
func ExtractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateWorkspace() string {
	b := make([]byte, 5)
	rand.Read(b)
	enc := "abcdefghijklmnopqrstuvwxyz0123456789"
	var sb strings.Builder
	for _, v := range b {
		sb.WriteByte(enc[int(v)%len(enc)])
	}
	return "ws_" + sb.String()
}
