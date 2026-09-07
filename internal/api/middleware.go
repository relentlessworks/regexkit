package api

import (
	"net/http"

	"github.com/relentlessworks/regexkit/internal/auth"
)

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := auth.ExtractToken(r)
		if token == "" {
			writeError(w, r, http.StatusUnauthorized, "missing auth token", "call POST /auth/request with email to get an OTP, then POST /auth/verify to get a bearer token")
			return
		}
		email, ok := s.auth.ValidateToken(token)
		if !ok {
			writeError(w, r, http.StatusUnauthorized, "invalid or expired token", "call POST /auth/request with email to get a new OTP, then POST /auth/verify")
			return
		}
		r.Header.Set("X-Auth-Email", email)
		r.Header.Set("X-Workspace", s.auth.GetWorkspace(token))
		r.Header.Set("X-Token", token)
		next(w, r)
	}
}
