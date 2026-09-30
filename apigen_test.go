package secronna

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Client.API (api_generated.go) goes through apigenRequest: the same bearer
// and envelope handling as every hand-written call.

type apigenSeen struct {
	method, uri, auth, contentType string
	body                           []byte
}

func apigenServer(t *testing.T, status int, env map[string]any) (*httptest.Server, *[]apigenSeen) {
	t.Helper()
	var seen []apigenSeen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, apigenSeen{r.Method, r.URL.RequestURI(), r.Header.Get("Authorization"), r.Header.Get("Content-Type"), body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(env)
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func apigenOK(data any) map[string]any {
	return map[string]any{"data": data, "error": nil, "meta": map[string]any{"requestId": "req_ok"}}
}

func TestAPI_SetSecretSendsBody(t *testing.T) {
	srv, seen := apigenServer(t, 200, apigenOK(map[string]any{"secretId": "sec_1", "version": 2}))
	c := New(Config{Token: "st_test", BaseURL: srv.URL})
	data, err := c.API.EnvironmentsSetSecrets(context.Background(), "env/1", &EnvironmentsSetSecretsArgs{Key: "DATABASE_URL", Value: "postgres://x?a=1&b=2"})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"secretId":"sec_1","version":2}` {
		t.Fatalf("data = %s", data)
	}
	r := (*seen)[0]
	if r.method != "PUT" || r.uri != "/api/v1/environments/env%2F1/secrets" || r.auth != "Bearer st_test" || r.contentType != "application/json" {
		t.Fatalf("request = %+v", r)
	}
	var got map[string]string
	if err := json.Unmarshal(r.body, &got); err != nil || got["key"] != "DATABASE_URL" || got["value"] != "postgres://x?a=1&b=2" || len(got) != 2 {
		t.Fatalf("body = %s (%v)", r.body, err)
	}
}

func TestAPI_RevealSendsQueryWithoutBody(t *testing.T) {
	srv, seen := apigenServer(t, 200, apigenOK(map[string]any{"key": "K", "value": "v", "version": 1}))
	c := New(Config{Token: "st_test", BaseURL: srv.URL})
	if _, err := c.API.SecretsReveal(context.Background(), "sec_1", &SecretsRevealArgs{Version: Ptr(1)}); err != nil {
		t.Fatal(err)
	}
	r := (*seen)[0]
	if r.method != "POST" || r.uri != "/api/v1/secrets/sec_1/reveal?version=1" || len(r.body) != 0 || r.contentType != "" || r.auth != "Bearer st_test" {
		t.Fatalf("request = %+v", r)
	}
}

func TestAPI_ListWithoutArgs(t *testing.T) {
	srv, seen := apigenServer(t, 200, apigenOK([]any{}))
	c := New(Config{Token: "st_test", BaseURL: srv.URL})
	data, err := c.API.ProjectsList(context.Background())
	if err != nil || string(data) != `[]` {
		t.Fatalf("data = %s, err = %v", data, err)
	}
	if r := (*seen)[0]; r.method != "GET" || r.uri != "/api/v1/projects" {
		t.Fatalf("request = %+v", r)
	}
}

func TestAPI_RequiredFieldMissing(t *testing.T) {
	srv, seen := apigenServer(t, 200, apigenOK(nil))
	c := New(Config{Token: "st_test", BaseURL: srv.URL})
	_, err := c.API.EnvironmentsSetSecrets(context.Background(), "env_1", &EnvironmentsSetSecretsArgs{Key: "K"})
	if err == nil || !strings.Contains(err.Error(), "Value") {
		t.Fatalf("err = %v, want a missing Value", err)
	}
	if len(*seen) != 0 {
		t.Fatalf("sent %d requests", len(*seen))
	}
}

func TestAPI_ErrorEnvelopeAndNoToken(t *testing.T) {
	srv, seen := apigenServer(t, 404, map[string]any{"data": nil, "error": map[string]any{"code": "NOT_FOUND", "message": "nope"}, "meta": map[string]any{"requestId": "req_9"}})
	_, err := New(Config{Token: "st_test", BaseURL: srv.URL}).API.SecretsDelete(context.Background(), "sec_x")
	e, ok := err.(*Error)
	if !ok || e.Status != 404 || e.Code != "NOT_FOUND" || e.RequestID != "req_9" {
		t.Fatalf("err = %#v", err)
	}
	t.Setenv("SECRONNA_TOKEN", "")
	_, err = New(Config{BaseURL: srv.URL}).API.ProjectsList(context.Background())
	if e, ok := err.(*Error); !ok || e.Code != "AUTH_REQUIRED" || len(*seen) != 1 {
		t.Fatalf("err = %#v, requests = %d", err, len(*seen))
	}
}
