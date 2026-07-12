package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"gostartv2/internal/config"
	"gostartv2/internal/database"
)

type mockDB struct {
	healthy bool
}

func (m *mockDB) Health() map[string]string {
	if m.healthy {
		return map[string]string{"status": "up", "message": "It's healthy"}
	}
	return map[string]string{"status": "down", "error": "db down"}
}

func (m *mockDB) Ping(ctx context.Context) error {
	return nil
}

func (m *mockDB) Close() error {
	return nil
}

func newTestServer(db database.Service) *Server {
	return &Server{
		cfg: &config.Config{
			CORS: config.CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
				AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
				AllowCredentials: false,
				MaxAge:           300,
			},
		},
		logger: slog.Default(),
		db:     db,
	}
}

func TestHelloHandler(t *testing.T) {
	s := newTestServer(&mockDB{healthy: true})
	server := httptest.NewServer(http.HandlerFunc(s.helloHandler))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("error making request to server. Err: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", resp.Status)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json; got %s", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body. Err: %v", err)
	}

	expected := `{"message":"Hello World"}`
	if strings.TrimSpace(string(body)) != expected {
		t.Errorf("expected response body to be %s; got %s", expected, string(body))
	}
}

func TestReadyHandlerUp(t *testing.T) {
	s := newTestServer(&mockDB{healthy: true})
	server := httptest.NewServer(http.HandlerFunc(s.readyHandler))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("error making request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", resp.Status)
	}
}

func TestReadyHandlerDown(t *testing.T) {
	s := newTestServer(&mockDB{healthy: false})
	server := httptest.NewServer(http.HandlerFunc(s.readyHandler))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("error making request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status ServiceUnavailable; got %v", resp.Status)
	}
}

func TestHealthHandler(t *testing.T) {
	s := newTestServer(&mockDB{healthy: true})
	server := httptest.NewServer(http.HandlerFunc(s.healthHandler))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("error making request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", resp.Status)
	}
}
