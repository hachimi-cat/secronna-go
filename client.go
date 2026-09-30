// Package secronna is the Go SDK for the secronna.com secrets REST API.
// Sister to @forjio/secronna (JS) and forjio-secronna (Python).
//
// Auth = Bearer token — an sk_live_… API key from the dashboard (or a
// Huudis-minted access token). Pass Config.Token or set SECRONNA_TOKEN.
//
// Every response rides the Forjio envelope {data, error, meta}; the
// client unwraps it and returns *Error (carrying the envelope's
// error.code) on failure.
//
// The API is organized around projects → environments → secrets. Secrets
// are immutable + versioned: writing a key mints a new version, and the
// plaintext value is only ever returned by an explicit, audited reveal.
package secronna

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is the Secronna typed client.
type Client struct {
	token   string
	baseURL string
	httpc   *http.Client

	// API has every feature route, one method each (generated from the API
	// spec: api_generated.go), with the same bearer and envelope handling
	// as every other call.
	API *GeneratedAPI
}

// Config holds the credentials + endpoint overrides.
type Config struct {
	// Token is the Bearer token — an sk_live_… API key or a Huudis
	// access token. Defaults to the SECRONNA_TOKEN env var.
	Token string
	// BaseURL overrides the API base. Default: https://secronna.com.
	BaseURL string
	// HTTP overrides the http.Client. Default: 30s timeout.
	HTTP *http.Client
}

// New constructs a Secronna client.
//
// Example:
//
//	c := secronna.New(secronna.Config{Token: os.Getenv("SECRONNA_TOKEN")})
//	project, err := c.CreateProject(ctx, "Web")
func New(cfg Config) *Client {
	token := cfg.Token
	if token == "" {
		token = os.Getenv("SECRONNA_TOKEN")
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://secronna.com"
	}
	httpc := cfg.HTTP
	if httpc == nil {
		httpc = &http.Client{Timeout: 30 * time.Second}
	}
	c := &Client{
		token:   token,
		baseURL: strings.TrimRight(base, "/"),
		httpc:   httpc,
	}
	c.API = &GeneratedAPI{c: c}
	return c
}

// apigenRequest is the call behind Client.API (api_generated.go): the same
// bearer auth and envelope handling as every other call.
func (c *Client) apigenRequest(ctx context.Context, method, path string, query url.Values, body map[string]any) (json.RawMessage, error) {
	var send any
	if body != nil {
		send = body
	}
	var out json.RawMessage
	if err := c.do(ctx, strings.ToUpper(method), path, query, send, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// envelope mirrors the Forjio data/error/meta API envelope.
type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *envelopeError  `json:"error"`
	Meta  *envelopeMeta   `json:"meta"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
	DocURL  string `json:"docUrl,omitempty"`
}

type envelopeMeta struct {
	RequestID string `json:"requestId,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// do builds the request, attaches the bearer, parses the envelope, and
// decodes the data slot into out (pointer; nil to ignore the body).
func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
) error {
	if c.token == "" {
		return &Error{
			Status:  0,
			Code:    "AUTH_REQUIRED",
			Message: "no token configured: set Config.Token or SECRONNA_TOKEN",
		}
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return &Error{Status: 0, Code: "SERIALIZE_FAILED", Message: err.Error()}
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return &Error{Status: 0, Code: "REQUEST_BUILD_FAILED", Message: err.Error()}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	res, err := c.httpc.Do(req)
	if err != nil {
		return &Error{Status: 0, Code: "NETWORK_ERROR", Message: err.Error()}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return &Error{
			Status:  res.StatusCode,
			Code:    "INVALID_RESPONSE",
			Message: fmt.Sprintf("non-JSON response (HTTP %d)", res.StatusCode),
		}
	}

	requestID := ""
	if env.Meta != nil {
		requestID = env.Meta.RequestID
	}
	if res.StatusCode >= 400 || env.Error != nil {
		code := "UNKNOWN"
		message := fmt.Sprintf("HTTP %d", res.StatusCode)
		param := ""
		if env.Error != nil {
			if env.Error.Code != "" {
				code = env.Error.Code
			}
			if env.Error.Message != "" {
				message = env.Error.Message
			}
			param = env.Error.Param
		}
		return &Error{
			Status:    res.StatusCode,
			Code:      code,
			Message:   message,
			RequestID: requestID,
			Param:     param,
		}
	}

	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return &Error{
				Status:    res.StatusCode,
				Code:      "DECODE_FAILED",
				Message:   err.Error(),
				RequestID: requestID,
			}
		}
	}
	return nil
}

// ─── Projects ─────────────────────────────────────────────────────

// CreateProject calls POST /api/v1/projects.
func (c *Client) CreateProject(ctx context.Context, name string) (*Project, error) {
	var out Project
	if err := c.do(ctx, "POST", "/api/v1/projects", nil, map[string]string{"name": name}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListProjects calls GET /api/v1/projects.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var out []Project
	if err := c.do(ctx, "GET", "/api/v1/projects", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── Environments ─────────────────────────────────────────────────

// CreateEnvironment calls POST /api/v1/projects/:projectId/environments.
func (c *Client) CreateEnvironment(ctx context.Context, projectID, name string) (*Environment, error) {
	var out Environment
	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/environments"
	if err := c.do(ctx, "POST", path, nil, map[string]string{"name": name}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListEnvironments calls GET /api/v1/projects/:projectId/environments.
func (c *Client) ListEnvironments(ctx context.Context, projectID string) ([]Environment, error) {
	var out []Environment
	path := "/api/v1/projects/" + url.PathEscape(projectID) + "/environments"
	if err := c.do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── Secrets ──────────────────────────────────────────────────────

// SetSecret calls PUT /api/v1/environments/:envId/secrets, minting a new
// immutable version for key (creating it if absent).
func (c *Client) SetSecret(ctx context.Context, envID, key, value string) (*SecretVersion, error) {
	var out SecretVersion
	path := "/api/v1/environments/" + url.PathEscape(envID) + "/secrets"
	body := map[string]string{"key": key, "value": value}
	if err := c.do(ctx, "PUT", path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSecrets calls GET /api/v1/environments/:envId/secrets — keys only,
// no values.
func (c *Client) ListSecrets(ctx context.Context, envID string) ([]SecretMeta, error) {
	var out []SecretMeta
	path := "/api/v1/environments/" + url.PathEscape(envID) + "/secrets"
	if err := c.do(ctx, "GET", path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RevealSecret calls POST /api/v1/secrets/:secretId/reveal. Every reveal
// is written to the audit log. Pass version > 0 to read a historical
// version; pass 0 for the current version.
func (c *Client) RevealSecret(ctx context.Context, secretID string, version int) (*RevealedSecret, error) {
	q := url.Values{}
	if version > 0 {
		q.Set("version", strconv.Itoa(version))
	}
	var out RevealedSecret
	path := "/api/v1/secrets/" + url.PathEscape(secretID) + "/reveal"
	if err := c.do(ctx, "POST", path, q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSecret calls DELETE /api/v1/secrets/:secretId (all versions).
func (c *Client) DeleteSecret(ctx context.Context, secretID string) (*DeletedSecret, error) {
	var out DeletedSecret
	path := "/api/v1/secrets/" + url.PathEscape(secretID)
	if err := c.do(ctx, "DELETE", path, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ─── Audit ────────────────────────────────────────────────────────

// ListAudit calls GET /api/v1/audit — recent audit-log entries. limit
// caps how many are returned (pass 0 for the server default).
func (c *Client) ListAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out []AuditEntry
	if err := c.do(ctx, "GET", "/api/v1/audit", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ─── Convenience ──────────────────────────────────────────────────

// FetchSecrets reveals every secret in an environment and returns them as
// a map[KEY]value — the shape you'd inject into a process environment. It
// lists the environment's secrets then reveals each at its current
// version, so it triggers one secret.reveal audit entry per key.
func (c *Client) FetchSecrets(ctx context.Context, envID string) (map[string]string, error) {
	metas, err := c.ListSecrets(ctx, envID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(metas))
	for _, m := range metas {
		revealed, err := c.RevealSecret(ctx, m.ID, m.CurrentVersion)
		if err != nil {
			return nil, err
		}
		out[revealed.Key] = revealed.Value
	}
	return out, nil
}
