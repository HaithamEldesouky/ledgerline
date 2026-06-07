package httpapi

import (
	"encoding/json"
	"net/http"
)

// maxBodyBytes caps request bodies to protect against oversized payloads.
const maxBodyBytes = 1 << 20 // 1 MiB

// writeJSON serialises v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// decodeJSON strictly decodes a JSON request body into dst, rejecting unknown
// fields and bodies larger than maxBodyBytes.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
