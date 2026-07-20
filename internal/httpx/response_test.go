package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRespondJSON_OK(t *testing.T) {
	w := httptest.NewRecorder()
	RespondJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d want %d", w.Code, http.StatusOK)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q want application/json", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if body["hello"] != "world" {
		t.Errorf("hello: got %q want world", body["hello"])
	}
}

func TestRespondJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	RespondJSON(w, http.StatusNoContent, nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d want %d", w.Code, http.StatusNoContent)
	}

	if w.Body.Len() != 0 {
		t.Errorf("expected empty body, got %d bytes", w.Body.Len())
	}
}

func TestRespondError_Envelope(t *testing.T) {
	w := httptest.NewRecorder()
	RespondError(w, http.StatusBadRequest, "invalid_body", "request body is not valid JSON")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d want %d", w.Code, http.StatusBadRequest)
	}

	var env map[string]map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	errObj, ok := env["error"]
	if !ok {
		t.Fatal("missing 'error' key in envelope")
	}

	if errObj["code"] != "invalid_body" {
		t.Errorf("code: got %q want invalid_body", errObj["code"])
	}

	if errObj["message"] != "request body is not valid JSON" {
		t.Errorf("message: got %q want expected message", errObj["message"])
	}
}

func TestRespondNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	RespondNoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d want %d", w.Code, http.StatusNoContent)
	}
}

func TestDecodeJSON_OK(t *testing.T) {
	body := `{"email":"test@example.com","password":"secret"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	var dst struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := DecodeJSON(r, &dst); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}

	if dst.Email != "test@example.com" {
		t.Errorf("email: got %q want test@example.com", dst.Email)
	}

	if dst.Password != "secret" {
		t.Errorf("password: got %q want secret", dst.Password)
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{not json"))

	var dst map[string]string
	if err := DecodeJSON(r, &dst); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestDecodeJSON_BodySizeLimit(t *testing.T) {
	large := strings.Repeat("a", MaxRequestBodyBytes+1)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(large))

	var dst map[string]string

	err := DecodeJSON(r, &dst)
	if err == nil {
		t.Fatal("expected error for body exceeding size limit, got nil")
	}
}
