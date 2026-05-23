package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kelos-dev/kelos/internal/controller"
)

const (
	DefaultAgentQuery      = `{namespace="kelos-system", container=~"codex|claude-code|gemini|opencode|cursor"}`
	DefaultSlackQuery      = `{namespace="kelos-system", container="slack-server"}`
	DefaultControllerQuery = `{namespace="kelos-system", container="manager"}`
)

type LokiBackfillOptions struct {
	DatabaseURL     string
	LokiURL         string
	LokiTenantID    string
	Cluster         string
	Instance        string
	Namespace       string
	From            time.Time
	To              time.Time
	AgentQuery      string
	SlackQuery      string
	ControllerQuery string
	Limit           int
	Parallelism     int
	CheckpointKey   string
	DryRun          bool
}

type BackfillSummary struct {
	AgentEntries      int
	SlackEntries      int
	ControllerEntries int
	Sessions          int
	Turns             int
	PartialRows       int
}

type LokiClient struct {
	baseURL  string
	tenantID string
	client   *http.Client
}

func NewLokiClient(baseURL, tenantID string) (*LokiClient, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("loki URL is required")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &LokiClient{
		baseURL:  baseURL,
		tenantID: tenantID,
		client:   &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *LokiClient) QueryRange(ctx context.Context, query string, from, to time.Time, limit int) ([]lokiEntry, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5000
	}
	var out []lokiEntry
	start := from
	for !start.After(to) {
		entries, err := c.queryRangePage(ctx, query, start, to, limit)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		out = append(out, entries...)
		last := entries[len(entries)-1].Timestamp
		next := last.Add(time.Nanosecond)
		if !next.After(start) {
			break
		}
		if len(entries) < limit {
			break
		}
		start = next
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

func (c *LokiClient) queryRangePage(ctx context.Context, query string, from, to time.Time, limit int) ([]lokiEntry, error) {
	endpoint, err := url.Parse(c.baseURL + "/loki/api/v1/query_range")
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(from.UnixNano(), 10))
	q.Set("end", strconv.FormatInt(to.UnixNano(), 10))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("direction", "forward")
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.tenantID != "" {
		req.Header.Set("X-Scope-OrgID", c.tenantID)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("loki query_range returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var parsed lokiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decoding loki response: %w", err)
	}
	var out []lokiEntry
	for _, stream := range parsed.Data.Result {
		for _, pair := range stream.Values {
			if len(pair) != 2 {
				continue
			}
			ts, err := parseLokiTimestamp(pair[0])
			if err != nil {
				parseErrorsTotal.WithLabelValues("loki_timestamp").Inc()
				continue
			}
			out = append(out, lokiEntry{
				Timestamp: ts,
				Labels:    stream.Stream,
				Line:      pair[1],
			})
		}
	}
	return out, nil
}

type lokiResponse struct {
	Data struct {
		Result []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

type lokiEntry struct {
	Timestamp time.Time
	Labels    map[string]string
	Line      string
}

func RunLokiBackfill(ctx context.Context, store *Store, opts LokiBackfillOptions) (BackfillSummary, error) {
	if opts.AgentQuery == "" {
		opts.AgentQuery = DefaultAgentQuery
	}
	if opts.SlackQuery == "" {
		opts.SlackQuery = DefaultSlackQuery
	}
	if opts.ControllerQuery == "" {
		opts.ControllerQuery = DefaultControllerQuery
	}
	if opts.Cluster == "" {
		return BackfillSummary{}, fmt.Errorf("cluster is required")
	}
	if opts.Namespace == "" {
		return BackfillSummary{}, fmt.Errorf("namespace is required")
	}
	if opts.From.IsZero() || opts.To.IsZero() || !opts.From.Before(opts.To) {
		return BackfillSummary{}, fmt.Errorf("valid from/to range is required")
	}

	loki, err := NewLokiClient(opts.LokiURL, opts.LokiTenantID)
	if err != nil {
		return BackfillSummary{}, err
	}
	agentEntries, err := loki.QueryRange(ctx, opts.AgentQuery, opts.From, opts.To, opts.Limit)
	if err != nil {
		return BackfillSummary{}, fmt.Errorf("querying agent logs: %w", err)
	}
	slackEntries, err := loki.QueryRange(ctx, opts.SlackQuery, opts.From, opts.To, opts.Limit)
	if err != nil {
		return BackfillSummary{}, fmt.Errorf("querying Slack logs: %w", err)
	}
	controllerEntries, err := loki.QueryRange(ctx, opts.ControllerQuery, opts.From, opts.To, opts.Limit)
	if err != nil {
		return BackfillSummary{}, fmt.Errorf("querying controller logs: %w", err)
	}

	bundles := parseLokiAgentEntries(agentEntries, opts.Cluster, opts.Namespace)
	mergeSlackEntries(bundles, slackEntries, opts.Cluster, opts.Namespace)

	summary := BackfillSummary{
		AgentEntries:      len(agentEntries),
		SlackEntries:      len(slackEntries),
		ControllerEntries: len(controllerEntries),
		Sessions:          len(uniqueLokiSessions(bundles)),
		Turns:             len(bundles),
	}
	for _, bundle := range bundles {
		if bundle.Turn.SlackUserID == nil || bundle.Turn.SlackChannelID == nil || bundle.Turn.SlackThreadTS == nil {
			summary.PartialRows++
		}
	}
	if opts.DryRun {
		return summary, nil
	}
	if store == nil {
		return BackfillSummary{}, fmt.Errorf("store is required when dry-run is false")
	}
	for _, bundle := range bundles {
		if err := store.UpsertSessionAndTurn(ctx, bundle.Session, bundle.Turn); err != nil {
			return summary, err
		}
	}
	if opts.CheckpointKey != "" {
		if err := store.SetOffset(ctx, opts.CheckpointKey, opts.To.Format(time.RFC3339Nano)); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

type lokiBundle struct {
	Session SessionRecord
	Turn    TurnRecord
}

func parseLokiAgentEntries(entries []lokiEntry, cluster, namespace string) map[string]*lokiBundle {
	grouped := map[string][]lokiEntry{}
	for _, entry := range entries {
		if entry.Labels["namespace"] != namespace {
			continue
		}
		key := streamKey(entry.Labels)
		grouped[key] = append(grouped[key], entry)
	}
	out := map[string]*lokiBundle{}
	for _, group := range grouped {
		sort.SliceStable(group, func(i, j int) bool { return group[i].Timestamp.Before(group[j].Timestamp) })
		first := group[0]
		taskName := taskNameFromLabels(first.Labels)
		if taskName == "" {
			continue
		}
		logText := strings.Builder{}
		for _, entry := range group {
			logText.WriteString(entry.Line)
			logText.WriteByte('\n')
		}
		outputs := controller.ParseOutputs(logText.String())
		results := controller.ResultsFromOutputs(outputs)
		started := first.Timestamp
		completed := group[len(group)-1].Timestamp
		phase := "Pending"
		if len(outputs) > 0 {
			phase = "Succeeded"
		}
		sessionID := LokiFallbackSessionID(namespace, taskName)
		turnID := LokiTaskTurnID(namespace, taskName)
		task := taskName
		agentType := first.Labels["container"]
		session := SessionRecord{
			SessionID:       sessionID,
			Cluster:         cluster,
			Namespace:       namespace,
			TaskSpawnerName: "unknown",
			Persona:         "unknown",
			Source:          sourceLoki,
			FirstSeenAt:     started,
			LastActivityAt:  completed,
			FirstTaskName:   &task,
			Status:          "closed",
		}
		turn := TurnRecord{
			TurnID:          turnID,
			SessionID:       sessionID,
			Cluster:         cluster,
			Namespace:       namespace,
			TaskSpawnerName: "unknown",
			Persona:         "unknown",
			Source:          sourceLoki,
			TaskName:        &task,
			AgentType:       valuePtr(agentType),
			Phase:           phase,
			StartedAt:       &started,
			CompletedAt:     &completed,
			DurationSeconds: durationSeconds(&started, &completed),
			InputTokens:     parseIntMetric(results, "input-tokens", "input_tokens", "inputTokens"),
			OutputTokens:    parseIntMetric(results, "output-tokens", "output_tokens", "outputTokens"),
			TotalTokens:     parseIntMetric(results, "total-tokens", "total_tokens", "totalTokens"),
			CostUSD:         parseDecimalMetric(results, "cost-usd", "cost_usd", "costUSD"),
			PRURL:           parseStringMetric(results, "pr", "pr-url", "pr_url", "pull_request"),
		}
		if turn.TotalTokens == nil && turn.InputTokens != nil && turn.OutputTokens != nil {
			total := *turn.InputTokens + *turn.OutputTokens
			turn.TotalTokens = &total
		}
		out[turnID] = &lokiBundle{Session: session, Turn: turn}
	}
	return out
}

func mergeSlackEntries(bundles map[string]*lokiBundle, entries []lokiEntry, cluster, namespace string) {
	var routes []slackRouteEvent
	var created []slackCreatedTaskEvent
	var turnEvents []slackTurnEvent

	for _, entry := range entries {
		if entry.Labels["namespace"] != namespace {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(entry.Line), &event); err != nil {
			continue
		}
		msg, _ := event["msg"].(string)
		pod := entry.Labels["pod"]
		spawner, _ := event["spawner"].(string)
		channel, _ := event["channel"].(string)
		user, _ := event["user"].(string)
		switch {
		case strings.HasPrefix(msg, "Message matches TaskSpawner"):
			routes = append(routes, slackRouteEvent{
				Timestamp: entry.Timestamp,
				Pod:       pod,
				Spawner:   spawner,
				Channel:   channel,
				User:      user,
			})
		case strings.HasPrefix(msg, "Message matches session-enabled TaskSpawner"):
			routes = append(routes, slackRouteEvent{
				Timestamp: entry.Timestamp,
				Pod:       pod,
				Spawner:   spawner,
				Channel:   channel,
				User:      user,
				Session:   true,
			})
		case msg == "Created task from Slack message":
			task, _ := event["task"].(string)
			created = append(created, slackCreatedTaskEvent{
				Timestamp: entry.Timestamp,
				Pod:       pod,
				Spawner:   spawner,
				Task:      task,
			})
		case strings.Contains(msg, "AgentTurn"):
			turn, _ := event["turn"].(string)
			turnEvents = append(turnEvents, slackTurnEvent{
				Timestamp: entry.Timestamp,
				Pod:       pod,
				Turn:      turn,
				Channel:   channel,
				Terminal: strings.Contains(msg, "terminal") ||
					strings.Contains(msg, "terminal result"),
			})
		}
	}

	for _, task := range created {
		route := nearestRoute(routes, task.Pod, task.Spawner, task.Timestamp, false)
		turnID := LokiTaskTurnID(namespace, task.Task)
		bundle := bundles[turnID]
		if bundle == nil {
			started := task.Timestamp
			sessionID := LokiFallbackSessionID(namespace, task.Task)
			name := task.Task
			bundle = &lokiBundle{
				Session: SessionRecord{
					SessionID:       sessionID,
					Cluster:         cluster,
					Namespace:       namespace,
					TaskSpawnerName: task.Spawner,
					Persona:         personaFor(task.Spawner, nil),
					Source:          sourceLoki,
					FirstSeenAt:     started,
					LastActivityAt:  started,
					FirstTaskName:   &name,
					Status:          "active",
				},
				Turn: TurnRecord{
					TurnID:          turnID,
					SessionID:       sessionID,
					Cluster:         cluster,
					Namespace:       namespace,
					TaskSpawnerName: task.Spawner,
					Persona:         personaFor(task.Spawner, nil),
					Source:          sourceLoki,
					TaskName:        &name,
					Phase:           "Pending",
					StartedAt:       &started,
				},
			}
			bundles[turnID] = bundle
		}
		applySpawner(bundle, task.Spawner)
		if route != nil {
			applySlackRoute(bundle, route)
		}
	}

	sort.SliceStable(turnEvents, func(i, j int) bool { return turnEvents[i].Timestamp.Before(turnEvents[j].Timestamp) })
	for _, event := range turnEvents {
		if event.Turn == "" {
			continue
		}
		sessionName := sessionNameFromTurn(event.Turn)
		turnID := LokiTurnID(namespace, event.Turn)
		bundle := bundles[turnID]
		if bundle == nil {
			sessionID := LokiFallbackSessionID(namespace, sessionName)
			started := event.Timestamp
			turnName := event.Turn
			bundle = &lokiBundle{
				Session: SessionRecord{
					SessionID:        sessionID,
					Cluster:          cluster,
					Namespace:        namespace,
					TaskSpawnerName:  spawnerFromSessionName(sessionName),
					Persona:          personaFor(spawnerFromSessionName(sessionName), nil),
					Source:           sourceLoki,
					SlackChannelID:   valuePtr(event.Channel),
					FirstSeenAt:      started,
					LastActivityAt:   started,
					AgentSessionName: valuePtr(sessionName),
					Status:           "active",
				},
				Turn: TurnRecord{
					TurnID:          turnID,
					SessionID:       sessionID,
					Cluster:         cluster,
					Namespace:       namespace,
					TaskSpawnerName: spawnerFromSessionName(sessionName),
					Persona:         personaFor(spawnerFromSessionName(sessionName), nil),
					Source:          sourceLoki,
					SlackChannelID:  valuePtr(event.Channel),
					AgentTurnName:   &turnName,
					Phase:           "Running",
					StartedAt:       &started,
				},
			}
			bundles[turnID] = bundle
		}
		if event.Channel != "" {
			bundle.Session.SlackChannelID = valuePtr(event.Channel)
			bundle.Turn.SlackChannelID = valuePtr(event.Channel)
		}
		if event.Terminal {
			completed := event.Timestamp
			bundle.Turn.CompletedAt = &completed
			bundle.Turn.DurationSeconds = durationSeconds(bundle.Turn.StartedAt, bundle.Turn.CompletedAt)
			bundle.Turn.Phase = "Succeeded"
			bundle.Session.LastActivityAt = completed
			bundle.Session.Status = "active"
		}
		if route := nearestRoute(routes, event.Pod, "", event.Timestamp, true); route != nil {
			applySlackRoute(bundle, route)
		}
	}
}

type slackRouteEvent struct {
	Timestamp time.Time
	Pod       string
	Spawner   string
	Channel   string
	User      string
	Session   bool
}

type slackCreatedTaskEvent struct {
	Timestamp time.Time
	Pod       string
	Spawner   string
	Task      string
}

type slackTurnEvent struct {
	Timestamp time.Time
	Pod       string
	Turn      string
	Channel   string
	Terminal  bool
}

func nearestRoute(routes []slackRouteEvent, pod, spawner string, at time.Time, session bool) *slackRouteEvent {
	var best *slackRouteEvent
	var bestDelta time.Duration
	for i := range routes {
		route := &routes[i]
		if route.Session != session {
			continue
		}
		if pod != "" && route.Pod != "" && route.Pod != pod {
			continue
		}
		if spawner != "" && route.Spawner != spawner {
			continue
		}
		if route.Timestamp.After(at) {
			continue
		}
		delta := at.Sub(route.Timestamp)
		if delta > 5*time.Second {
			continue
		}
		if best == nil || delta < bestDelta {
			best = route
			bestDelta = delta
		}
	}
	return best
}

func applySlackRoute(bundle *lokiBundle, route *slackRouteEvent) {
	if route == nil {
		return
	}
	applySpawner(bundle, route.Spawner)
	if route.Channel != "" {
		bundle.Session.SlackChannelID = valuePtr(route.Channel)
		bundle.Turn.SlackChannelID = valuePtr(route.Channel)
	}
	if route.User != "" {
		bundle.Session.FirstUserID = valuePtr(route.User)
		bundle.Turn.SlackUserID = valuePtr(route.User)
	}
	if bundle.Session.SlackRootTS != nil {
		bundle.Turn.SlackThreadTS = bundle.Session.SlackRootTS
	}
}

func applySpawner(bundle *lokiBundle, spawner string) {
	if spawner == "" {
		return
	}
	persona := personaFor(spawner, nil)
	bundle.Session.TaskSpawnerName = spawner
	bundle.Session.Persona = persona
	bundle.Turn.TaskSpawnerName = spawner
	bundle.Turn.Persona = persona
}

func uniqueLokiSessions(bundles map[string]*lokiBundle) map[string]struct{} {
	out := map[string]struct{}{}
	for _, bundle := range bundles {
		out[bundle.Session.SessionID] = struct{}{}
	}
	return out
}

func streamKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, "\xff")
}

var podSuffixRE = regexp.MustCompile(`-[a-z0-9]{5,}$`)

func taskNameFromLabels(labels map[string]string) string {
	for _, key := range []string{"app", "service_name"} {
		if name := strings.TrimSpace(labels[key]); strings.Contains(name, "-slack-") || strings.Contains(name, "-sess-") {
			return name
		}
	}
	if job := strings.TrimSpace(labels["job"]); job != "" {
		parts := strings.Split(job, "/")
		candidate := parts[len(parts)-1]
		if strings.Contains(candidate, "-slack-") || strings.Contains(candidate, "-sess-") {
			return candidate
		}
	}
	pod := strings.TrimSpace(labels["pod"])
	if pod == "" {
		return ""
	}
	return podSuffixRE.ReplaceAllString(pod, "")
}

func parseLokiTimestamp(value string) (time.Time, error) {
	if ns, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(0, ns).UTC(), nil
	}
	return time.Parse(time.RFC3339Nano, value)
}

func sessionNameFromTurn(turn string) string {
	idx := strings.LastIndex(turn, "-t-")
	if idx == -1 {
		return turn
	}
	return turn[:idx]
}

func spawnerFromSessionName(sessionName string) string {
	idx := strings.LastIndex(sessionName, "-sess-")
	if idx == -1 {
		return "unknown"
	}
	return sessionName[:idx]
}
