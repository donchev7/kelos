package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAtlassianHandlerInjectsServerSideAuth(t *testing.T) {
	var gotAuth string
	var gotCookie string
	var gotBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	}))
	defer upstream.Close()

	s := &server{
		cfg: config{
			upstreamURL:   upstream.URL,
			authorization: "Basic server-secret",
		},
		httpClient: upstream.Client(),
		logger:     testLogger(),
		ready:      true,
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp/atlassian", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer client-secret")
	req.Header.Set("Cookie", "session=client")
	rec := httptest.NewRecorder()

	s.handleAtlassian(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if gotAuth != "Basic server-secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotCookie != "" {
		t.Fatalf("Cookie forwarded = %q", gotCookie)
	}
	if gotBody["method"] != "tools/list" {
		t.Fatalf("unexpected body: %#v", gotBody)
	}
}

func TestAtlassianHandlerRejectsUnknownSubroute(t *testing.T) {
	s := &server{logger: testLogger()}
	req := httptest.NewRequest(http.MethodPost, "/mcp/atlassian/extra", nil)
	rec := httptest.NewRecorder()

	s.handleAtlassian(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAikidoHandlerProxiesReadOnlyRequestsWithServerSideAuth(t *testing.T) {
	var gotAuth string
	var gotCookie string
	var gotPath string
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"items":[]}`))
	}))
	defer upstream.Close()

	s := &server{
		cfg: config{
			aikidoAPIBaseURL:    upstream.URL + "/api/public/v1",
			aikidoAuthorization: "Bearer server-secret",
		},
		httpClient: upstream.Client(),
		logger:     testLogger(),
		ready:      true,
	}
	req := httptest.NewRequest(http.MethodGet, "/aikido/open-issue-groups?filter_code_repo_name=payments-api&page=0", nil)
	req.Header.Set("Authorization", "Bearer client-secret")
	req.Header.Set("Cookie", "session=client")
	rec := httptest.NewRecorder()

	s.handleAikido(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if gotAuth != "Bearer server-secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotCookie != "" {
		t.Fatalf("Cookie forwarded = %q", gotCookie)
	}
	if gotPath != "/api/public/v1/open-issue-groups" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotQuery != "filter_code_repo_name=payments-api&page=0" {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestAikidoHandlerRejectsWriteMethods(t *testing.T) {
	s := &server{
		cfg: config{
			aikidoAPIBaseURL:    "https://app.aikido.dev/api/public/v1",
			aikidoAuthorization: "Bearer server-secret",
		},
		httpClient: http.DefaultClient,
		logger:     testLogger(),
	}
	req := httptest.NewRequest(http.MethodPost, "/aikido/open-issue-groups", nil)
	rec := httptest.NewRecorder()

	s.handleAikido(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}

func TestAikidoHandlerRequiresServerSideAuth(t *testing.T) {
	s := &server{
		cfg: config{
			aikidoAPIBaseURL: "https://app.aikido.dev/api/public/v1",
		},
		httpClient: http.DefaultClient,
		logger:     testLogger(),
	}
	req := httptest.NewRequest(http.MethodGet, "/aikido/open-issue-groups", nil)
	rec := httptest.NewRecorder()

	s.handleAikido(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAikidoHandlerMintsAndCachesOAuthToken(t *testing.T) {
	var tokenRequests int
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		assertAikidoTokenRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-1",
			"token_type":   "bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	var apiRequests int
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiRequests++
		if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
			t.Fatalf("Authorization = %q", got)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"items":[]}`))
	}))
	defer apiServer.Close()

	cfg := config{
		aikidoAPIBaseURL:   apiServer.URL + "/api/public/v1",
		aikidoTokenURL:     tokenServer.URL + "/oauth/token",
		aikidoClientID:     "client-id",
		aikidoClientSecret: "client-secret",
	}
	s := &server{
		cfg:         cfg,
		httpClient:  apiServer.Client(),
		aikidoOAuth: newAikidoOAuthClientCredentials(cfg, apiServer.Client()),
		logger:      testLogger(),
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/aikido/open-issue-groups", nil)
		rec := httptest.NewRecorder()
		s.handleAikido(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d", i+1, rec.Code)
		}
	}
	if tokenRequests != 1 {
		t.Fatalf("tokenRequests = %d", tokenRequests)
	}
	if apiRequests != 2 {
		t.Fatalf("apiRequests = %d", apiRequests)
	}
}

func TestAikidoHandlerRefreshesOAuthTokenOnUnauthorized(t *testing.T) {
	var tokenRequests int
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		assertAikidoTokenRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-" + string(rune('0'+tokenRequests)),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()

	var apiRequests int
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiRequests++
		switch r.Header.Get("Authorization") {
		case "Bearer token-1":
			http.Error(w, "expired", http.StatusUnauthorized)
		case "Bearer token-2":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"items":[]}`))
		default:
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
	}))
	defer apiServer.Close()

	cfg := config{
		aikidoAPIBaseURL:   apiServer.URL + "/api/public/v1",
		aikidoTokenURL:     tokenServer.URL + "/oauth/token",
		aikidoClientID:     "client-id",
		aikidoClientSecret: "client-secret",
	}
	s := &server{
		cfg:         cfg,
		httpClient:  apiServer.Client(),
		aikidoOAuth: newAikidoOAuthClientCredentials(cfg, apiServer.Client()),
		logger:      testLogger(),
	}
	req := httptest.NewRequest(http.MethodGet, "/aikido/open-issue-groups", nil)
	rec := httptest.NewRecorder()

	s.handleAikido(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if tokenRequests != 2 {
		t.Fatalf("tokenRequests = %d", tokenRequests)
	}
	if apiRequests != 2 {
		t.Fatalf("apiRequests = %d", apiRequests)
	}
}

func TestAikidoHandlerBlocksRedirectsToUnexpectedHosts(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://evil.example/steal", http.StatusFound)
	}))
	defer upstream.Close()

	s := &server{
		cfg: config{
			aikidoAPIBaseURL:    upstream.URL + "/api/public/v1",
			aikidoAuthorization: "Bearer server-secret",
		},
		httpClient: upstream.Client(),
		logger:     testLogger(),
	}
	req := httptest.NewRequest(http.MethodGet, "/aikido/open-issue-groups", nil)
	rec := httptest.NewRecorder()

	s.handleAikido(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAikidoAuthorizationFromEnv(t *testing.T) {
	if got := aikidoAuthorizationFromEnv("Bearer exact", "ignored"); got != "Bearer exact" {
		t.Fatalf("exact authorization = %q", got)
	}
	if got := aikidoAuthorizationFromEnv("", "raw-token"); got != "Bearer raw-token" {
		t.Fatalf("raw token authorization = %q", got)
	}
	if got := aikidoAuthorizationFromEnv("", "Basic encoded"); got != "Basic encoded" {
		t.Fatalf("preformatted authorization = %q", got)
	}
}

func TestAikidoClientCredentialsFromEnv(t *testing.T) {
	id, secret, err := aikidoClientCredentialsFromEnv("client-id:client-secret", "", "")
	if err != nil {
		t.Fatalf("credentials pair failed: %v", err)
	}
	if id != "client-id" || secret != "client-secret" {
		t.Fatalf("id/secret = %q/%q", id, secret)
	}

	id, secret, err = aikidoClientCredentialsFromEnv("", "separate-id", "separate-secret")
	if err != nil {
		t.Fatalf("separate credentials failed: %v", err)
	}
	if id != "separate-id" || secret != "separate-secret" {
		t.Fatalf("id/secret = %q/%q", id, secret)
	}

	if _, _, err := aikidoClientCredentialsFromEnv("bad", "", ""); err == nil {
		t.Fatal("expected malformed credentials to fail")
	}
	if _, _, err := aikidoClientCredentialsFromEnv("", "id", ""); err == nil {
		t.Fatal("expected partial credentials to fail")
	}
}

func TestAikidoExpiresAt(t *testing.T) {
	now := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	if got := aikidoExpiresAt(now, 3600); !got.Equal(now.Add(59 * time.Minute)) {
		t.Fatalf("expiresAt = %s", got)
	}
	if got := aikidoExpiresAt(now, 0); !got.Equal(now.Add(aikidoDefaultTokenTTL)) {
		t.Fatalf("default expiresAt = %s", got)
	}
}

func TestParseRPCRequestLogFields(t *testing.T) {
	fields := parseRPCRequestLogFields([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"createJiraIssue","arguments":{}}}`))
	if fields.Method != "tools/call" {
		t.Fatalf("Method = %q", fields.Method)
	}
	if fields.Tool != "createJiraIssue" {
		t.Fatalf("Tool = %q", fields.Tool)
	}
}

func TestAtlassianHosts(t *testing.T) {
	hosts := atlassianHosts(map[string]any{
		"resources": []any{
			map[string]any{"url": "https://wgen4.atlassian.net"},
			map[string]any{"siteUrl": "alpheya.atlassian.net"},
		},
	})
	if _, ok := hosts["wgen4.atlassian.net"]; !ok {
		t.Fatalf("missing wgen4 host: %#v", hosts)
	}
	if _, ok := hosts["alpheya.atlassian.net"]; !ok {
		t.Fatalf("missing alpheya host: %#v", hosts)
	}
}

func TestRequiredToolsPresent(t *testing.T) {
	if !requiredToolsPresent([]string{"createJiraIssue", "getAccessibleAtlassianResources", "searchJiraIssuesUsingJql"}) {
		t.Fatal("expected required tools to be present")
	}
	if requiredToolsPresent([]string{"getTeamworkGraphContext", "getTeamworkGraphObject"}) {
		t.Fatal("expected required tools to be missing")
	}
}

func TestValidateAtlassianAccessFailsForMultipleSites(t *testing.T) {
	upstream := mockMCPServer(t, []any{
		map[string]any{"url": "https://wgen4.atlassian.net"},
		map[string]any{"url": "https://alpheya.atlassian.net"},
	})
	defer upstream.Close()

	err := validateAtlassianAccess(context.Background(), upstream.Client(), config{
		upstreamURL:   upstream.URL,
		authorization: "Basic token",
		expectedSite:  "https://wgen4.atlassian.net",
	}, testLogger())
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "expected only wgen4.atlassian.net") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAtlassianAccessSucceedsForWgen4Only(t *testing.T) {
	upstream := mockMCPServer(t, []any{
		map[string]any{"url": "https://wgen4.atlassian.net"},
	})
	defer upstream.Close()

	err := validateAtlassianAccess(context.Background(), upstream.Client(), config{
		upstreamURL:   upstream.URL,
		authorization: "Basic token",
		expectedSite:  "https://wgen4.atlassian.net",
	}, testLogger())
	if err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func mockMCPServer(t *testing.T, resources any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic token" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if method, _ := req["method"].(string); method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		id := req["id"]
		method, _ := req["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"protocolVersion": "2025-06-18",
				},
			})
		case "tools/list":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"tools": []any{
						map[string]any{"name": "getAccessibleAtlassianResources"},
					},
				},
			})
		case "tools/call":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"content": []any{
						map[string]any{
							"type": "text",
							"text": mustJSON(t, resources),
						},
					},
				},
			})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
}

func assertAikidoTokenRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("method = %s", r.Method)
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		t.Fatal("missing Basic auth")
	}
	if id != "client-id" || secret != "client-secret" {
		t.Fatalf("Basic auth = %q/%q", id, secret)
	}
	if got := r.Header.Get("Content-Type"); got != aikidoTokenRequestContentType {
		t.Fatalf("Content-Type = %q", got)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("parse form: %v", err)
	}
	if values.Get("grant_type") != "client_credentials" {
		t.Fatalf("grant_type = %q", values.Get("grant_type"))
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
