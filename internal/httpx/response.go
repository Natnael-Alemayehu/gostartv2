// Package httpx provides JSON response and request-decoding helpers that
// enforce a consistent error envelope ({"error":{"code","message"}}) across
// every HTTP handler. Handlers should use these instead of writing responses
// directly so Content-Type and the error shape stay uniform.
package httpx

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// RespondJSON writes data as JSON with the given HTTP status, setting
// Content-Type to application/json. When data is nil only the status code is
// written, so callers can reuse this for empty success responses.
func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// RespondError writes the standard error envelope
// {"error":{"code","message"}} with the given status. Use it for every error
// response so clients can rely on a stable shape and a machine-readable code.
func RespondError(w http.ResponseWriter, status int, code, message string) {
	RespondJSON(w, status, errorEnvelope{
		Error: errorBody{
			Code:    code,
			Message: message,
		},
	})
}

// RespondNoContent writes a 204 status with no body, for operations such as
// DELETE or PUT that have nothing to return.
func RespondNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// MaxRequestBodyBytes is the default upper bound on JSON request bodies. 1 MB
// is generous for the current DTOs (user registration, login) while preventing
// unbounded memory consumption from malicious or buggy clients.
const MaxRequestBodyBytes = 1 << 20 // 1 MB

// DecodeJSON parses the request body into dst with a size limit of
// MaxRequestBodyBytes. Handlers should translate syntax and type errors into
// a 400 via RespondError rather than surfacing the raw error to clients.
func DecodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, MaxRequestBodyBytes)
	return json.NewDecoder(r.Body).Decode(dst)
}
