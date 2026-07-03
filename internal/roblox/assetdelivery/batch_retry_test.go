package assetdelivery

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newEchoServer(t *testing.T, bodies *[]string, cookieCounts *[]int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(b))
		*cookieCounts = append(*cookieCounts, len(r.Cookies()))
		w.Write([]byte(`[]`))
	}))
}

// The old handler pattern reused one *http.Request across retry attempts: the
// body reader is consumed by the first attempt and cookies accumulate. This
// documents why NewBatchHandler now builds a fresh request per call.
func TestSharedRequestBreaksOnSecondAttempt(t *testing.T) {
	var bodies []string
	var cookieCounts []int
	srv := newEchoServer(t, &bodies, &cookieCounts)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL, bytes.NewReader([]byte(`[{"assetId":123}]`)))
	req.AddCookie(&http.Cookie{Name: ".ROBLOSECURITY", Value: "x"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Second attempt with the same request: body already consumed.
	req.AddCookie(&http.Cookie{Name: ".ROBLOSECURITY", Value: "x"})
	resp, err = http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
		if len(bodies) == 2 && bodies[1] == bodies[0] {
			t.Fatal("expected shared request reuse to lose the body or error; retry fix may be unnecessary")
		}
	}
}

// Fresh request per attempt (what NewBatchHandler does now): every attempt
// sends the full JSON body and exactly one cookie.
func TestFreshRequestPerAttempt(t *testing.T) {
	var bodies []string
	var cookieCounts []int
	srv := newEchoServer(t, &bodies, &cookieCounts)
	defer srv.Close()

	jsonBody := []byte(`[{"assetId":123}]`)
	for i := 0; i < 3; i++ {
		req, err := newBatchRequest(jsonBody, 456)
		if err != nil {
			t.Fatal(err)
		}
		req.URL, _ = req.URL.Parse(srv.URL)
		req.Host = ""
		req.AddCookie(&http.Cookie{Name: ".ROBLOSECURITY", Value: "x"})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
		resp.Body.Close()
	}

	if len(bodies) != 3 {
		t.Fatalf("got %d requests, want 3", len(bodies))
	}
	for i := range bodies {
		if bodies[i] != string(jsonBody) {
			t.Errorf("attempt %d: body = %q, want %q", i+1, bodies[i], jsonBody)
		}
		if cookieCounts[i] != 1 {
			t.Errorf("attempt %d: %d cookies, want 1", i+1, cookieCounts[i])
		}
	}
}
