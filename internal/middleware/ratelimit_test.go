package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// countHandler records the number of times the downstream handler ran so a
// test can assert that the rate limiter blocked requests rather than passing
// them through after the burst was exhausted.
type countHandler struct {
	calls atomic.Int32
}

func (c *countHandler) ServeHTTP(http.ResponseWriter, *http.Request) {
	c.calls.Add(1)
}

// assertTooManyRequests verifies a 429 response carries the project's
// standard error envelope with the rate_limited code.
func assertTooManyRequests(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	if w.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status: got %d want %d", w.Result().StatusCode, http.StatusTooManyRequests)
	}

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(w.Result().Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}

	if env.Error.Code != "rate_limited" {
		t.Errorf("error code: got %q want rate_limited", env.Error.Code)
	}
}

// TestRateLimit_AllowsUnderBurst confirms that every request within the burst
// capacity reaches the downstream handler and returns 200.
func TestRateLimit_AllowsUnderBurst(t *testing.T) {
	const (
		rps   = 1.0
		burst = 3
	)

	h := &countHandler{}
	wrapped := RateLimit(rps, burst)(h)

	for range burst {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:5678"
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)

		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for request within burst, got %d", rec.Result().StatusCode)
		}
	}

	if got := int(h.calls.Load()); got != burst {
		t.Errorf("downstream call count: got %d want %d", got, burst)
	}
}

// TestRateLimit_BlocksOverBurst verifies that the (burst+1)-th immediate
// request is rejected with 429 rate_limited and never reaches the handler.
func TestRateLimit_BlocksOverBurst(t *testing.T) {
	const (
		rps   = 1.0
		burst = 2
	)

	h := &countHandler{}
	wrapped := RateLimit(rps, burst)(h)

	for range burst {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1000"
		wrapped.ServeHTTP(httptest.NewRecorder(), req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1000"
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	assertTooManyRequests(t, rec)

	if got := int(h.calls.Load()); got != burst {
		t.Errorf("downstream call count after rejection: got %d want %d", got, burst)
	}
}

// TestRateLimit_PerIPIndependentBuckets verifies that exceeding one client's
// burst does not affect a different client's bucket.
func TestRateLimit_PerIPIndependentBuckets(t *testing.T) {
	const (
		rps   = 1.0
		burst = 1
	)

	h := &countHandler{}
	wrapped := RateLimit(rps, burst)(h)

	// Exhaust the bucket for the first client.
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "192.168.0.1:5000"
	wrapped.ServeHTTP(httptest.NewRecorder(), req1)
	wrapped.ServeHTTP(httptest.NewRecorder(), req1)

	// A second client should still get its own fresh bucket.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "192.168.0.2:5000"
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req2)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("second IP should not be rate-limited, got %d", rec.Result().StatusCode)
	}

	if got := int(h.calls.Load()); got != 2 {
		t.Errorf("downstream call count: got %d want 2 (one per IP allowed)", got)
	}
}

// TestRateLimit_EmptyRemoteAddrUsesUnknownBucket confirms that a request with
// no RemoteAddr is bucketed under the "unknown" key rather than panicking.
func TestRateLimit_EmptyRemoteAddrUsesUnknownBucket(t *testing.T) {
	const (
		rps   = 10.0
		burst = 1
	)

	h := &countHandler{}
	wrapped := RateLimit(rps, burst)(h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ""
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("first request from unknown IP should pass, got %d", rec.Result().StatusCode)
	}

	if got := int(h.calls.Load()); got != 1 {
		t.Errorf("downstream call count: got %d want 1", got)
	}
}

// TestRateLimit_RecoversAfterRefill verifies the token bucket refills over
// time: after a blocked request, waiting long enough for at least one token
// to refill allows a subsequent request to pass.
func TestRateLimit_RecoversAfterRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("rate-limit refill test sleeps ~150ms; skipped in -short mode")
	}

	const (
		rps   = 20.0 // ~1 token per 50ms
		burst = 1
	)

	h := &countHandler{}
	wrapped := RateLimit(rps, burst)(h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.16.0.1:8000"
	wrapped.ServeHTTP(httptest.NewRecorder(), req)

	// Second immediate request must be blocked.
	blockedRec := httptest.NewRecorder()
	wrapped.ServeHTTP(blockedRec, req)

	if blockedRec.Result().StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 immediately after burst, got %d", blockedRec.Result().StatusCode)
	}

	// Allow enough time for the token bucket to accrue >=1 token.
	time.Sleep(120 * time.Millisecond)

	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after refill, got %d", rec.Result().StatusCode)
	}
}

// TestRateLimit_GetsByRemoteAddrPortAndIP confirms that clients are
// differentiated by the full RemoteAddr string (including the port), so
// requests from the same IP on distinct ports are bucketed separately. This
// documents the current RemoteAddr-based behavior rather than any
// X-Forwarded-For handling, which the middleware intentionally does not do.
func TestRateLimit_GetsByRemoteAddrPortAndIP(t *testing.T) {
	const (
		rps   = 1.0
		burst = 1
	)

	h := &countHandler{}
	wrapped := RateLimit(rps, burst)(h)

	// Same IP, different ports — each should get its own bucket.
	ips := []string{"203.0.113.7:1001", "203.0.113.7:1002"}

	for _, addr := range ips {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)

		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", addr, rec.Result().StatusCode)
		}
	}

	if got := int(h.calls.Load()); got != len(ips) {
		t.Errorf("downstream call count: got %d want %d", got, len(ips))
	}
}
