package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func wantsJSON(r *http.Request) bool {
	if r.URL.Query().Get("format") == "json" {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeText(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintln(w, text)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg, hint string) {
	if wantsJSON(r) {
		writeJSON(w, status, map[string]string{"error": msg, "hint": hint})
		return
	}
	writeText(w, status, fmt.Sprintf("error: %s | hint: %s", msg, hint))
}

func writeRecord(w http.ResponseWriter, r *http.Request, fields ...string) {
	if wantsJSON(r) {
		m := make(map[string]string)
		for i := 0; i+1 < len(fields); i += 2 {
			m[fields[i]] = fields[i+1]
		}
		writeJSON(w, http.StatusOK, m)
		return
	}
	var sb strings.Builder
	for i := 0; i+1 < len(fields); i += 2 {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(fields[i])
		sb.WriteByte('=')
		sb.WriteString(fields[i+1])
	}
	writeText(w, http.StatusOK, sb.String())
}
