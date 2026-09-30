package secronna

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	c := New(Config{Token: "sk_live_test"})
	if c == nil {
		t.Fatal("client is nil")
	}
}

func TestCreateProjectUnwrapsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/projects" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk_live_123" {
			t.Errorf("auth header = %q", got)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "Web" {
			t.Errorf("name = %q", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  map[string]any{"id": "proj_1", "name": "Web", "slug": "web", "createdAt": "now", "createdBy": "u1"},
			"error": nil,
			"meta":  map[string]any{"requestId": "req_p", "timestamp": "now"},
		})
	}))
	defer srv.Close()

	c := New(Config{Token: "sk_live_123", BaseURL: srv.URL})
	out, err := c.CreateProject(context.Background(), "Web")
	if err != nil {
		t.Fatal(err)
	}
	if out.Slug != "web" || out.CreatedBy != "u1" {
		t.Errorf("unexpected project: %+v", out)
	}
}

func TestRevealSecretSendsVersionQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/secrets/sec_1/reveal" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("version"); got != "2" {
			t.Errorf("version = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  map[string]any{"key": "API_KEY", "version": 2, "value": "sk_live_x"},
			"error": nil,
			"meta":  map[string]any{"requestId": "req_r", "timestamp": "now"},
		})
	}))
	defer srv.Close()

	c := New(Config{Token: "tok", BaseURL: srv.URL})
	out, err := c.RevealSecret(context.Background(), "sec_1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "sk_live_x" {
		t.Errorf("value = %q", out.Value)
	}
}

func TestFetchSecretsBuildsMap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/environments/env_1/secrets":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []any{
					map[string]any{"id": "sec_a", "key": "DATABASE_URL", "currentVersion": 4, "createdAt": "now"},
					map[string]any{"id": "sec_b", "key": "STRIPE_KEY", "currentVersion": 1, "createdAt": "now"},
				},
				"error": nil,
				"meta":  map[string]any{"requestId": "req_l", "timestamp": "now"},
			})
		case "/api/v1/secrets/sec_a/reveal":
			if r.URL.Query().Get("version") != "4" {
				t.Errorf("sec_a version = %q", r.URL.Query().Get("version"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"key": "DATABASE_URL", "version": 4, "value": "postgres://x"}, "error": nil,
				"meta": map[string]any{"requestId": "r", "timestamp": "now"},
			})
		case "/api/v1/secrets/sec_b/reveal":
			if r.URL.Query().Get("version") != "1" {
				t.Errorf("sec_b version = %q", r.URL.Query().Get("version"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"key": "STRIPE_KEY", "version": 1, "value": "sk_live_y"}, "error": nil,
				"meta": map[string]any{"requestId": "r", "timestamp": "now"},
			})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := New(Config{Token: "tok", BaseURL: srv.URL})
	out, err := c.FetchSecrets(context.Background(), "env_1")
	if err != nil {
		t.Fatal(err)
	}
	if out["DATABASE_URL"] != "postgres://x" || out["STRIPE_KEY"] != "sk_live_y" {
		t.Errorf("unexpected map: %+v", out)
	}
}

func TestEnvelopeErrorBecomesTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":  nil,
			"error": map[string]any{"code": "NOT_FOUND", "message": "secret not found"},
			"meta":  map[string]any{"requestId": "req_x", "timestamp": "now"},
		})
	}))
	defer srv.Close()

	c := New(Config{Token: "tok", BaseURL: srv.URL})
	_, err := c.RevealSecret(context.Background(), "sec_missing", 0)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" || apiErr.Status != 404 || apiErr.RequestID != "req_x" {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestMissingTokenFailsFast(t *testing.T) {
	t.Setenv("SECRONNA_TOKEN", "")
	c := New(Config{BaseURL: "http://127.0.0.1:0"})
	_, err := c.ListProjects(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Code != "AUTH_REQUIRED" {
		t.Fatalf("expected AUTH_REQUIRED, got %v", err)
	}
}
