package capture

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/kelos-dev/kelos/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	markerStart     = "---KELOS_OUTPUTS_START---"
	markerEnd       = "---KELOS_OUTPUTS_END---"
	agentOutputFile = "/tmp/agent-output.jsonl"
	commandTimeout  = 30 * time.Second
)

// Run captures deterministic outputs (branch, commit, PRs, token usage) from
// the workspace and emits them between markers to stdout. Returns 0 on success.
func Run() int {
	ctx := observability.ExtractContextFromEnv(context.Background())
	agentType := os.Getenv("KELOS_AGENT_TYPE")
	exitCode := agentExitCode()
	startOptions := []trace.SpanStartOption{
		trace.WithAttributes(agentRunAttributes(agentType, exitCode)...),
	}
	if startedAt, ok := agentStartedAt(); ok {
		startOptions = append(startOptions, trace.WithTimestamp(startedAt))
	}
	_, span := observability.Tracer("github.com/kelos-dev/kelos/internal/capture").Start(
		ctx,
		"cody.agent.run",
		startOptions...,
	)
	defer span.End()

	outputs := captureOutputs(realRunner{}, agentOutputFile)
	summary := summarizeAgentRun(agentType, agentOutputFile)
	span.SetAttributes(summary.attributes()...)
	for family, count := range summary.CommandFamilies {
		span.AddEvent("cody.agent.command_family", trace.WithAttributes(
			attribute.String("command.family", family),
			attribute.Int("command.count", count),
		))
	}
	if exitCode != 0 {
		span.SetStatus(codes.Error, fmt.Sprintf("agent exited with code %d", exitCode))
	}

	if len(outputs) == 0 {
		return 0
	}
	fmt.Println(markerStart)
	for _, line := range outputs {
		fmt.Println(line)
	}
	fmt.Println(markerEnd)
	return 0
}

type agentRunSummary struct {
	InputTokens     int64
	OutputTokens    int64
	TurnCount       int
	CommandCount    int
	CommandFamilies map[string]int
}

func agentExitCode() int {
	raw := strings.TrimSpace(os.Getenv("KELOS_AGENT_EXIT_CODE"))
	if raw == "" {
		return 0
	}
	exitCode, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return exitCode
}

func agentStartedAt() (time.Time, bool) {
	raw := strings.TrimSpace(os.Getenv("KELOS_AGENT_STARTED_AT"))
	if raw == "" {
		return time.Time{}, false
	}
	startedAt, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return startedAt, true
}

func agentRunAttributes(agentType string, exitCode int) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Bool("main", true),
		attribute.String("kelos.agent.type", agentType),
		attribute.Int("kelos.agent.exit_code", exitCode),
	}
	if taskName := os.Getenv("KELOS_TASK_NAME"); taskName != "" {
		attrs = append(attrs, attribute.String("kelos.task.name", taskName))
	}
	if taskNamespace := os.Getenv("KELOS_TASK_NAMESPACE"); taskNamespace != "" {
		attrs = append(attrs, attribute.String("kelos.task.namespace", taskNamespace))
	}
	if taskSpawner := os.Getenv("KELOS_TASKSPAWNER"); taskSpawner != "" {
		attrs = append(attrs, attribute.String("kelos.taskspawner.name", taskSpawner))
	}
	if model := os.Getenv("KELOS_MODEL"); model != "" {
		attrs = append(attrs, attribute.String("kelos.agent.model", model))
	}
	return attrs
}

func summarizeAgentRun(agentType, filePath string) agentRunSummary {
	summary := agentRunSummary{CommandFamilies: map[string]int{}}
	if usage := ParseUsage(agentType, filePath); usage != nil {
		summary.InputTokens = parseInt64(usage["input-tokens"])
		summary.OutputTokens = parseInt64(usage["output-tokens"])
	}

	if agentType != "codex" {
		return summary
	}

	f, err := os.Open(filePath)
	if err != nil {
		return summary
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		m := parseLine(scanner.Bytes())
		if m == nil {
			continue
		}
		switch m["type"] {
		case "turn.completed":
			summary.TurnCount++
		case "item.started":
			item, ok := m["item"].(map[string]any)
			if !ok || item["type"] != "command_execution" {
				continue
			}
			summary.CommandCount++
			family := commandFamily(item["command"])
			summary.CommandFamilies[family]++
		}
	}
	return summary
}

func (s agentRunSummary) attributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.Int64("kelos.agent.input_tokens", s.InputTokens),
		attribute.Int64("kelos.agent.output_tokens", s.OutputTokens),
		attribute.Int("kelos.agent.turn_count", s.TurnCount),
		attribute.Int("kelos.agent.command_count", s.CommandCount),
	}
	return attrs
}

func parseInt64(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func commandFamily(raw any) string {
	command, ok := raw.(string)
	if !ok {
		return "unknown"
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "unknown"
	}
	name := path.Base(fields[0])
	if name == "bash" || name == "sh" || name == "zsh" {
		return "shell"
	}
	switch name {
	case "cat", "curl", "flux", "gh", "git", "go", "kubectl", "ls", "make", "node", "npm", "pnpm", "psql", "redis-cli", "rg", "sed", "temporal", "yarn":
		return name
	default:
		return "other"
	}
}

// runner abstracts command execution for testing.
type runner interface {
	run(name string, args ...string) (string, error)
}

type realRunner struct{}

func (realRunner) run(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), err
}

func captureOutputs(r runner, usageFile string) []string {
	var outputs []string

	inGitRepo := isGitRepo(r)

	if inGitRepo {
		branch, err := r.run("git", "branch", "--show-current")
		if err == nil && branch != "" {
			outputs = append(outputs, "branch: "+branch)
			outputs = append(outputs, capturePRs(r, branch)...)
		}

		commit, err := r.run("git", "rev-parse", "HEAD")
		if err == nil && commit != "" {
			outputs = append(outputs, "commit: "+commit)
		}
	}

	if base := os.Getenv("KELOS_BASE_BRANCH"); base != "" {
		outputs = append(outputs, "base-branch: "+base)
	} else if inGitRepo {
		ref, err := r.run("git", "symbolic-ref", "refs/remotes/origin/HEAD")
		if err == nil && ref != "" {
			branch := strings.TrimPrefix(ref, "refs/remotes/origin/")
			if branch != "" {
				outputs = append(outputs, "base-branch: "+branch)
			}
		}
	}

	agentType := os.Getenv("KELOS_AGENT_TYPE")
	usage := ParseUsage(agentType, usageFile)
	for _, key := range []string{"cost-usd", "input-tokens", "output-tokens"} {
		if v, ok := usage[key]; ok {
			outputs = append(outputs, key+": "+v)
		}
	}

	// Emit the agent's visible response text so reporters (Slack thread
	// replies, GitHub PR comments) can surface the answer to the user
	// instead of only the task title and token counts. Base64-encoded
	// because the text may span multiple lines, which would otherwise
	// break the line-delimited KELOS_OUTPUTS contract.
	if response := ParseResponse(agentType, usageFile); response != "" {
		outputs = append(outputs, "response: "+base64.StdEncoding.EncodeToString([]byte(response)))
	}

	return outputs
}

func isGitRepo(r runner) bool {
	_, err := r.run("git", "rev-parse", "--is-inside-work-tree")
	return err == nil
}

func capturePRs(r runner, branch string) []string {
	// Check origin repo (current behavior)
	lines := queryPRs(r, branch, "")

	// Also check upstream repo if set (fork workflow)
	if upstreamRepo := os.Getenv("KELOS_UPSTREAM_REPO"); upstreamRepo != "" {
		lines = append(lines, queryPRs(r, branch, upstreamRepo)...)
	}

	return lines
}

func queryPRs(r runner, branch, repo string) []string {
	args := []string{"pr", "list", "--head", branch, "--json", "url"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	output, err := r.run("gh", args...)
	if err != nil || output == "" {
		return nil
	}
	var prs []struct {
		URL string `json:"url"`
	}
	if json.Unmarshal([]byte(output), &prs) != nil {
		return nil
	}
	var lines []string
	for _, pr := range prs {
		if pr.URL != "" {
			lines = append(lines, "pr: "+pr.URL)
		}
	}
	return lines
}
