package source

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultAikidoProxyURL = "http://cody-tools.kelos-system.svc.cluster.local:8080/aikido"

	aikidoOpenIssueGroupsPerPage = 20
	aikidoMaxOpenIssueGroupPages = 10
	aikidoMaxBodyBytes           = 64 * 1024
)

const (
	AikidoMetadataIssueGroupID = "aikido.kelos.dev/issue-group-id"
	AikidoMetadataSeverity     = "aikido.kelos.dev/severity"
	AikidoMetadataStatus       = "aikido.kelos.dev/status"
	AikidoMetadataIssueType    = "aikido.kelos.dev/issue-type"
	AikidoMetadataRepositories = "aikido.kelos.dev/repositories"
	AikidoMetadataURL          = "aikido.kelos.dev/url"
)

// AikidoSource discovers Aikido issue groups through the cody-tools Aikido proxy.
type AikidoSource struct {
	ProxyBaseURL string
	Repositories []string
	Statuses     []string
	Severities   []string
	Client       *http.Client
}

func (s *AikidoSource) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func (s *AikidoSource) proxyBaseURL() string {
	if strings.TrimSpace(s.ProxyBaseURL) != "" {
		return strings.TrimSpace(s.ProxyBaseURL)
	}
	return DefaultAikidoProxyURL
}

// Discover fetches matching Aikido issue groups and returns them as WorkItems.
func (s *AikidoSource) Discover(ctx context.Context) ([]WorkItem, error) {
	baseURL, err := parseAikidoProxyBaseURL(s.proxyBaseURL())
	if err != nil {
		return nil, err
	}

	if err := s.validateRepositories(ctx, baseURL); err != nil {
		return nil, err
	}

	statuses := s.resolvedStatuses()
	repositories := append([]string(nil), s.Repositories...)
	if len(repositories) == 0 {
		repositories = []string{""}
	}

	seen := map[string]struct{}{}
	var items []WorkItem
	for _, repo := range repositories {
		for _, status := range statuses {
			groups, err := s.fetchIssueGroups(ctx, baseURL, repo, status)
			if err != nil {
				return nil, err
			}
			for _, group := range groups {
				if !s.matchesSeverity(group) {
					continue
				}
				id := aikidoStringFromAny(firstAikidoValue(group, "id", "issue_group_id", "issueGroupId", "group_id"))
				if id == "" {
					return nil, fmt.Errorf("Aikido issue group response is missing issue group ID")
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}

				item, err := aikidoGroupToWorkItem(group, id)
				if err != nil {
					return nil, err
				}
				items = append(items, item)
			}
		}
	}

	return items, nil
}

func parseAikidoProxyBaseURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(rawURL), "/"))
	if err != nil {
		return nil, fmt.Errorf("parsing Aikido proxy URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("Aikido proxy URL must use http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("Aikido proxy URL must include a host")
	}
	return u, nil
}

func (s *AikidoSource) resolvedStatuses() []string {
	if len(s.Statuses) == 0 {
		return []string{"open"}
	}
	return append([]string(nil), s.Statuses...)
}

func (s *AikidoSource) validateRepositories(ctx context.Context, baseURL *url.URL) error {
	for _, repo := range s.Repositories {
		repo = strings.TrimSpace(repo)
		if repo == "" {
			return fmt.Errorf("Aikido repository filters must not be empty")
		}

		params := url.Values{}
		params.Set("filter_name", repo)
		params.Set("per_page", "20")
		groups, err := s.getAikidoObjects(ctx, baseURL, "/repositories/code", params)
		if err != nil {
			return fmt.Errorf("validating Aikido repository %q: %w", repo, err)
		}

		if !aikidoRepositoryExactActiveMatch(groups, repo) {
			return fmt.Errorf("Aikido repository %q was not found as an exact active code repository match", repo)
		}
	}
	return nil
}

func aikidoRepositoryExactActiveMatch(repos []map[string]any, expected string) bool {
	for _, repo := range repos {
		name := aikidoStringFromAny(firstAikidoValue(repo, "name", "repository_name", "repositoryName", "full_name", "fullName"))
		if name != expected {
			continue
		}
		activeValue := firstAikidoValue(repo, "active", "is_active", "isActive")
		active, known := aikidoBoolFromAny(activeValue)
		if !known || active {
			return true
		}
	}
	return false
}

func (s *AikidoSource) fetchIssueGroups(ctx context.Context, baseURL *url.URL, repository, status string) ([]map[string]any, error) {
	var all []map[string]any
	for page := 0; page < aikidoMaxOpenIssueGroupPages; page++ {
		params := url.Values{}
		params.Set("per_page", strconv.Itoa(aikidoOpenIssueGroupsPerPage))
		params.Set("page", strconv.Itoa(page))
		if strings.TrimSpace(repository) != "" {
			params.Set("filter_code_repo_name", repository)
		}
		if strings.TrimSpace(status) != "" {
			params.Set("filter_status", status)
		}

		groups, err := s.getAikidoObjects(ctx, baseURL, "/open-issue-groups", params)
		if err != nil {
			return nil, fmt.Errorf("fetching Aikido issue groups: %w", err)
		}
		all = append(all, groups...)
		if len(groups) < aikidoOpenIssueGroupsPerPage {
			return all, nil
		}
	}
	return nil, fmt.Errorf("Aikido issue group page cap reached with a full page; narrow filters")
}

func (s *AikidoSource) getAikidoObjects(ctx context.Context, baseURL *url.URL, path string, params url.Values) ([]map[string]any, error) {
	u := *baseURL
	u.Path = strings.TrimRight(baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating Aikido request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Aikido proxy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("Aikido proxy returned status %d: %s", resp.StatusCode, string(body))
	}

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding Aikido response: %w", err)
	}

	objects, err := aikidoObjectsFromPayload(payload)
	if err != nil {
		return nil, err
	}
	return objects, nil
}

func aikidoObjectsFromPayload(payload any) ([]map[string]any, error) {
	switch v := payload.(type) {
	case []any:
		return aikidoObjectSlice(v), nil
	case map[string]any:
		for _, key := range []string{"data", "items", "results", "repositories", "issue_groups", "issueGroups"} {
			if raw, ok := v[key]; ok {
				if arr, ok := raw.([]any); ok {
					return aikidoObjectSlice(arr), nil
				}
			}
		}
		return []map[string]any{v}, nil
	default:
		return nil, fmt.Errorf("Aikido response did not contain an object list")
	}
}

func aikidoObjectSlice(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func (s *AikidoSource) matchesSeverity(group map[string]any) bool {
	if len(s.Severities) == 0 {
		return true
	}
	severity := normalizeAikidoValue(aikidoSeverity(group))
	if severity == "" {
		return false
	}
	for _, allowed := range s.Severities {
		if severity == normalizeAikidoValue(allowed) {
			return true
		}
	}
	return false
}

func aikidoGroupToWorkItem(group map[string]any, id string) (WorkItem, error) {
	title := aikidoStringFromAny(firstAikidoValue(group, "title", "name", "summary"))
	if title == "" {
		title = fmt.Sprintf("Aikido issue group %s", id)
	}
	severity := valueOrUnknown(aikidoSeverity(group))
	status := valueOrUnknown(aikidoStringFromAny(firstAikidoValue(group, "status", "state")))
	issueType := valueOrUnknown(aikidoStringFromAny(firstAikidoValue(group, "issue_type", "issueType", "type", "category")))
	repos := aikidoRepositoryNames(group)
	reposValue := strings.Join(repos, ",")
	groupURL := aikidoStringFromAny(firstAikidoValue(group, "url", "app_url", "appUrl", "html_url", "htmlUrl", "link"))
	body := aikidoWorkItemBody(group, id, title, severity, status, issueType, repos, groupURL)

	number := 0
	if n, err := strconv.Atoi(id); err == nil {
		number = n
	}

	labels := []string{"aikido", "severity:" + severity, "status:" + status, "type:" + issueType}
	for _, repo := range repos {
		labels = append(labels, "repo:"+repo)
	}

	return WorkItem{
		ID:     aikidoWorkItemID(id),
		Number: number,
		Title:  title,
		URL:    groupURL,
		Labels: labels,
		Body:   body,
		Kind:   "AikidoIssueGroup",
		Metadata: map[string]string{
			AikidoMetadataIssueGroupID: id,
			AikidoMetadataSeverity:     severity,
			AikidoMetadataStatus:       status,
			AikidoMetadataIssueType:    issueType,
			AikidoMetadataRepositories: reposValue,
			AikidoMetadataURL:          groupURL,
		},
	}, nil
}

func aikidoWorkItemID(id string) string {
	const prefix = "aikido-group-"
	safeID := aikidoInvalidIDPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(id)), "-")
	safeID = strings.Trim(safeID, "-")
	if safeID == "" {
		safeID = "unknown"
	}
	if len(safeID) <= 14 {
		return prefix + safeID
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(id))
	head := strings.Trim(safeID[:8], "-")
	if head == "" {
		head = "group"
	}
	return fmt.Sprintf("%s%s-%08x", prefix, head, hash.Sum32())
}

func aikidoWorkItemBody(group map[string]any, id, title, severity, status, issueType string, repos []string, groupURL string) string {
	leakedSecret := normalizeAikidoValue(issueType) == "leaked_secret" || strings.Contains(strings.ToLower(title), "secret")
	description := aikidoStringFromAny(firstAikidoValue(group, "description", "details", "message"))
	if leakedSecret {
		description = redactPotentialSecret(description)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Aikido issue group ID: %s\n", id)
	fmt.Fprintf(&b, "Title: %s\n", sanitizeAikidoText(title, leakedSecret))
	fmt.Fprintf(&b, "Severity: %s\n", severity)
	fmt.Fprintf(&b, "Status: %s\n", status)
	fmt.Fprintf(&b, "Issue type: %s\n", issueType)
	if len(repos) > 0 {
		fmt.Fprintf(&b, "Code repositories: %s\n", strings.Join(repos, ", "))
	}
	if groupURL != "" {
		fmt.Fprintf(&b, "Aikido URL: %s\n", groupURL)
	}
	if description != "" {
		fmt.Fprintf(&b, "\nDescription:\n%s\n", description)
	}

	hints := aikidoHints(group, leakedSecret)
	if len(hints) > 0 {
		b.WriteString("\nAvailable remediation/context hints:\n")
		for _, hint := range hints {
			fmt.Fprintf(&b, "- %s\n", hint)
		}
	}

	b.WriteString("\nUse the internal read-only Aikido proxy for deeper context if needed: ")
	b.WriteString(DefaultAikidoProxyURL)
	b.WriteString("\n")

	body := b.String()
	if len(body) > aikidoMaxBodyBytes {
		body = body[:aikidoMaxBodyBytes] + "\n[truncated]\n"
	}
	return body
}

func aikidoHints(group map[string]any, leakedSecret bool) []string {
	var hints []string
	fields := []struct {
		label string
		keys  []string
	}{
		{label: "Package", keys: []string{"package", "package_name", "packageName", "dependency"}},
		{label: "Version", keys: []string{"version", "current_version", "currentVersion"}},
		{label: "Fixed version", keys: []string{"fixed_version", "fixedVersion", "fix_version", "fixVersion"}},
		{label: "CVE", keys: []string{"cve", "cves", "vulnerability_id", "vulnerabilityId"}},
		{label: "File", keys: []string{"file", "file_path", "filePath", "path"}},
		{label: "Line", keys: []string{"line", "line_number", "lineNumber"}},
		{label: "Fix", keys: []string{"fix", "fix_suggestion", "fixSuggestion", "recommendation"}},
	}
	for _, field := range fields {
		value := aikidoStringFromAny(firstAikidoValue(group, field.keys...))
		value = sanitizeAikidoText(value, leakedSecret)
		if value != "" {
			hints = append(hints, field.label+": "+value)
		}
	}
	return hints
}

func aikidoSeverity(group map[string]any) string {
	raw := firstAikidoValue(group, "severity", "severity_level", "severityLevel", "severity_label", "severityLabel")
	if obj, ok := raw.(map[string]any); ok {
		raw = firstAikidoValue(obj, "name", "label", "level", "value")
	}
	return aikidoStringFromAny(raw)
}

func aikidoRepositoryNames(group map[string]any) []string {
	names := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			names[value] = struct{}{}
		}
	}

	add(aikidoStringFromAny(firstAikidoValue(group, "code_repository_name", "codeRepositoryName", "repository_name", "repositoryName", "repo", "repo_name", "repoName")))

	for _, key := range []string{"repositories", "code_repositories", "codeRepositories"} {
		if raw, ok := group[key]; ok {
			for _, name := range aikidoNamesFromAny(raw) {
				add(name)
			}
		}
	}

	if raw, ok := group["locations"]; ok {
		if locations, ok := raw.([]any); ok {
			for _, location := range locations {
				obj, ok := location.(map[string]any)
				if !ok {
					continue
				}
				add(aikidoStringFromAny(firstAikidoValue(obj, "code_repository_name", "codeRepositoryName", "repository_name", "repositoryName", "repo", "repo_name", "repoName")))
				for _, key := range []string{"code_repository", "codeRepository", "repository"} {
					if nested, ok := obj[key].(map[string]any); ok {
						add(aikidoStringFromAny(firstAikidoValue(nested, "name", "repository_name", "repositoryName", "full_name", "fullName")))
					}
				}
			}
		}
	}

	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func aikidoNamesFromAny(raw any) []string {
	switch v := raw.(type) {
	case []any:
		var names []string
		for _, item := range v {
			if name := aikidoStringFromAny(item); name != "" {
				names = append(names, name)
				continue
			}
			if obj, ok := item.(map[string]any); ok {
				name := aikidoStringFromAny(firstAikidoValue(obj, "name", "repository_name", "repositoryName", "full_name", "fullName"))
				if name != "" {
					names = append(names, name)
				}
			}
		}
		return names
	default:
		if name := aikidoStringFromAny(raw); name != "" {
			return []string{name}
		}
		return nil
	}
}

func firstAikidoValue(obj map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := obj[key]; ok {
			return value
		}
	}
	return nil
}

func aikidoStringFromAny(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		f := float64(v)
		if math.Trunc(f) == f {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		return strconv.FormatBool(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if text := aikidoStringFromAny(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func aikidoBoolFromAny(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return parsed, err == nil
	default:
		return false, false
	}
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func normalizeAikidoValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sanitizeAikidoText(value string, leakedSecret bool) string {
	if !leakedSecret {
		return value
	}
	return redactPotentialSecret(value)
}

var (
	aikidoInvalidIDPattern = regexp.MustCompile(`[^a-z0-9-]+`)
	likelySecretPattern    = regexp.MustCompile(`(?i)(secret|token|key|password|credential)(\s*[:=]\s*)([^\s,;]+)|\b(AKIA[0-9A-Z]{12,}|gh[pousr]_[A-Za-z0-9_]{20,}|[A-Za-z0-9+/]{32,}={0,2})\b`)
)

func redactPotentialSecret(value string) string {
	return likelySecretPattern.ReplaceAllStringFunc(value, func(match string) string {
		if strings.Contains(match, ":") || strings.Contains(match, "=") {
			idx := strings.IndexAny(match, ":=")
			return match[:idx+1] + " [redacted]"
		}
		return "[redacted]"
	})
}
