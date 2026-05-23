package usage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kelosv1alpha1 "github.com/kelos-dev/kelos/api/v1alpha1"
	"github.com/kelos-dev/kelos/internal/reporting"
)

const (
	labelTaskSpawner    = "kelos.dev/taskspawner"
	labelPersona        = "kelos.dev/persona"
	labelAlpheyaPersona = "cody.alpheya.com/persona"

	sourceSlack      = "slack"
	sourceKubernetes = "kubernetes"
	sourceLoki       = "loki"
)

// SessionRecord is the durable, dashboard-friendly shape for a logical Cody session.
type SessionRecord struct {
	SessionID        string
	Cluster          string
	Namespace        string
	TaskSpawnerName  string
	Persona          string
	Source           string
	SlackTeamID      *string
	SlackChannelID   *string
	SlackRootTS      *string
	FirstUserID      *string
	FirstSeenAt      time.Time
	LastActivityAt   time.Time
	FirstTaskName    *string
	AgentSessionName *string
	Status           string
}

// TurnRecord is the durable, dashboard-friendly shape for one explicit Cody request.
type TurnRecord struct {
	TurnID          string
	SessionID       string
	Cluster         string
	Namespace       string
	TaskSpawnerName string
	Persona         string
	Source          string
	SlackUserID     *string
	SlackChannelID  *string
	SlackThreadTS   *string
	SlackMessageTS  *string
	TaskName        *string
	TaskUID         *string
	AgentTurnName   *string
	AgentTurnUID    *string
	AgentType       *string
	Model           *string
	Phase           string
	StartedAt       *time.Time
	CompletedAt     *time.Time
	DurationSeconds *float64
	InputTokens     *int64
	OutputTokens    *int64
	TotalTokens     *int64
	CostUSD         *string
	PRURL           *string
	ErrorMessage    *string
}

// TaskSpawnerMeta carries optional TaskSpawner metadata that improves record labels.
type TaskSpawnerMeta struct {
	Name   string
	Labels map[string]string
}

// RecordsFromTask maps a one-shot Task into a logical session and one turn.
func RecordsFromTask(task *kelosv1alpha1.Task, cluster string, spawnerMeta *TaskSpawnerMeta) (SessionRecord, TurnRecord) {
	spawner := taskSpawnerName(task.Labels, task.OwnerReferences)
	if spawnerMeta != nil && spawnerMeta.Name != "" {
		spawner = spawnerMeta.Name
	}
	persona := personaFor(spawner, labelsFrom(task.Labels, spawnerMeta))
	slackChannel := valuePtr(task.Annotations[reporting.AnnotationSlackChannel])
	slackThread := valuePtr(task.Annotations[reporting.AnnotationSlackThreadTS])
	slackUser := valuePtr(task.Annotations[reporting.AnnotationSlackUserID])

	started := metav1TimePtr(task.Status.StartTime)
	completed := metav1TimePtr(task.Status.CompletionTime)
	firstSeen := task.CreationTimestamp.Time
	if started != nil {
		firstSeen = *started
	}
	lastActivity := firstSeen
	if completed != nil {
		lastActivity = *completed
	}

	sessionID := slackSessionID(cluster, task.Namespace, spawner, slackChannel, slackThread)
	if sessionID == "" {
		sessionID = fallbackSessionID("task", string(task.UID), task.Namespace, task.Name)
	}

	taskName := task.Name
	taskUID := string(task.UID)
	phase := string(task.Status.Phase)
	if phase == "" {
		phase = string(kelosv1alpha1.TaskPhasePending)
	}

	inputTokens := parseIntMetric(task.Status.Results, "input-tokens", "input_tokens", "inputTokens")
	outputTokens := parseIntMetric(task.Status.Results, "output-tokens", "output_tokens", "outputTokens")
	totalTokens := parseIntMetric(task.Status.Results, "total-tokens", "total_tokens", "totalTokens")
	if totalTokens == nil && inputTokens != nil && outputTokens != nil {
		total := *inputTokens + *outputTokens
		totalTokens = &total
	}

	session := SessionRecord{
		SessionID:       sessionID,
		Cluster:         cluster,
		Namespace:       task.Namespace,
		TaskSpawnerName: spawner,
		Persona:         persona,
		Source:          sourceSlack,
		SlackChannelID:  slackChannel,
		SlackRootTS:     slackThread,
		FirstUserID:     slackUser,
		FirstSeenAt:     firstSeen,
		LastActivityAt:  lastActivity,
		FirstTaskName:   &taskName,
		Status:          sessionStatusFromTaskPhase(task.Status.Phase),
	}
	turn := TurnRecord{
		TurnID:          taskTurnID(taskUID, task.Namespace, task.Name),
		SessionID:       sessionID,
		Cluster:         cluster,
		Namespace:       task.Namespace,
		TaskSpawnerName: spawner,
		Persona:         persona,
		Source:          sourceSlack,
		SlackUserID:     slackUser,
		SlackChannelID:  slackChannel,
		SlackThreadTS:   slackThread,
		TaskName:        &taskName,
		TaskUID:         valuePtr(taskUID),
		AgentType:       valuePtr(task.Spec.Type),
		Model:           valuePtr(task.Spec.Model),
		Phase:           phase,
		StartedAt:       started,
		CompletedAt:     completed,
		DurationSeconds: durationSeconds(started, completed),
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		TotalTokens:     totalTokens,
		CostUSD:         parseDecimalMetric(task.Status.Results, "cost-usd", "cost_usd", "costUSD"),
		PRURL:           parseStringMetric(task.Status.Results, "pr", "pr-url", "pr_url", "pull_request"),
		ErrorMessage:    valuePtr(task.Status.Message),
	}
	return session, turn
}

// SessionRecordFromAgentSession maps a thread-scoped AgentSession into a session row.
func SessionRecordFromAgentSession(session *kelosv1alpha1.AgentSession, cluster string, spawnerMeta *TaskSpawnerMeta) SessionRecord {
	spawner := session.Spec.TaskSpawnerRef.Name
	if spawnerMeta != nil && spawnerMeta.Name != "" {
		spawner = spawnerMeta.Name
	}
	persona := personaFor(spawner, labelsFrom(session.Labels, spawnerMeta))
	firstSeen := session.CreationTimestamp.Time
	lastActivity := firstSeen
	if session.Status.LastActivityAt != nil {
		lastActivity = session.Status.LastActivityAt.Time
	}
	sessionID := slackSessionID(cluster, session.Namespace, spawner, valuePtr(session.Spec.Source.ChannelID), valuePtr(session.Spec.Source.RootTS))
	if sessionID == "" {
		sessionID = fallbackSessionID("agentsession", string(session.UID), session.Namespace, session.Name)
	}
	name := session.Name
	return SessionRecord{
		SessionID:        sessionID,
		Cluster:          cluster,
		Namespace:        session.Namespace,
		TaskSpawnerName:  spawner,
		Persona:          persona,
		Source:           sourceSlack,
		SlackTeamID:      valuePtr(session.Spec.Source.TeamID),
		SlackChannelID:   valuePtr(session.Spec.Source.ChannelID),
		SlackRootTS:      valuePtr(session.Spec.Source.RootTS),
		FirstSeenAt:      firstSeen,
		LastActivityAt:   lastActivity,
		AgentSessionName: &name,
		Status:           sessionStatusFromAgentSessionPhase(session.Status.Phase),
	}
}

// RecordsFromAgentTurn maps an AgentTurn into a session and turn row.
func RecordsFromAgentTurn(turn *kelosv1alpha1.AgentTurn, session *kelosv1alpha1.AgentSession, cluster string, spawnerMeta *TaskSpawnerMeta) (SessionRecord, TurnRecord) {
	spawner := turn.Labels[labelTaskSpawner]
	if session != nil && session.Spec.TaskSpawnerRef.Name != "" {
		spawner = session.Spec.TaskSpawnerRef.Name
	}
	if spawnerMeta != nil && spawnerMeta.Name != "" {
		spawner = spawnerMeta.Name
	}
	persona := personaFor(spawner, labelsFrom(turn.Labels, spawnerMeta))

	channel := firstNonEmpty(turn.Spec.Source.ChannelID, turn.Annotations[reporting.AnnotationSlackChannel])
	rootTS := firstNonEmpty(turn.Spec.Source.RootTS, turn.Annotations[reporting.AnnotationSlackThreadTS])
	userID := firstNonEmpty(turn.Spec.Source.UserID, turn.Annotations[reporting.AnnotationSlackUserID])
	teamID := turn.Spec.Source.TeamID

	started := metav1TimePtr(turn.Status.StartedAt)
	completed := metav1TimePtr(turn.Status.CompletedAt)
	firstSeen := turn.CreationTimestamp.Time
	if started != nil {
		firstSeen = *started
	}
	lastActivity := firstSeen
	if completed != nil {
		lastActivity = *completed
	}

	sessionID := slackSessionID(cluster, turn.Namespace, spawner, valuePtr(channel), valuePtr(rootTS))
	if sessionID == "" && session != nil {
		sessionID = fallbackSessionID("agentsession", string(session.UID), session.Namespace, session.Name)
	}
	if sessionID == "" {
		sessionID = fallbackSessionID("agentturn", string(turn.UID), turn.Namespace, turn.Name)
	}
	sessionName := turn.Spec.SessionRef.Name
	agentTurnName := turn.Name
	agentTurnUID := string(turn.UID)
	phase := string(turn.Status.Phase)
	if phase == "" {
		phase = string(kelosv1alpha1.AgentTurnPhaseQueued)
	}

	sessionRecord := SessionRecord{
		SessionID:        sessionID,
		Cluster:          cluster,
		Namespace:        turn.Namespace,
		TaskSpawnerName:  spawner,
		Persona:          persona,
		Source:           sourceSlack,
		SlackTeamID:      valuePtr(teamID),
		SlackChannelID:   valuePtr(channel),
		SlackRootTS:      valuePtr(rootTS),
		FirstUserID:      valuePtr(userID),
		FirstSeenAt:      firstSeen,
		LastActivityAt:   lastActivity,
		AgentSessionName: valuePtr(sessionName),
		Status:           "active",
	}
	if session != nil {
		sessionRecord.Status = sessionStatusFromAgentSessionPhase(session.Status.Phase)
	}

	turnRecord := TurnRecord{
		TurnID:          agentTurnID(agentTurnUID, turn.Namespace, turn.Name),
		SessionID:       sessionID,
		Cluster:         cluster,
		Namespace:       turn.Namespace,
		TaskSpawnerName: spawner,
		Persona:         persona,
		Source:          sourceSlack,
		SlackUserID:     valuePtr(userID),
		SlackChannelID:  valuePtr(channel),
		SlackThreadTS:   valuePtr(rootTS),
		SlackMessageTS:  valuePtr(turn.Spec.Source.MessageTS),
		AgentTurnName:   &agentTurnName,
		AgentTurnUID:    valuePtr(agentTurnUID),
		Phase:           phase,
		StartedAt:       started,
		CompletedAt:     completed,
		DurationSeconds: durationSeconds(started, completed),
		ErrorMessage:    valuePtr(turn.Status.Message),
	}
	return sessionRecord, turnRecord
}

func taskTurnID(uid, namespace, name string) string {
	if uid != "" {
		return "task-" + uid
	}
	return "task-" + namespace + "-" + name
}

func agentTurnID(uid, namespace, name string) string {
	if uid != "" {
		return "agentturn-" + uid
	}
	return "agentturn-" + namespace + "-" + name
}

func LokiTaskTurnID(namespace, taskName string) string {
	return "loki-task-" + namespace + "-" + taskName
}

func LokiTurnID(namespace, turnName string) string {
	return "loki-agentturn-" + namespace + "-" + turnName
}

func LokiFallbackSessionID(namespace, name string) string {
	return "loki-" + namespace + "-" + name
}

func slackSessionID(cluster, namespace, spawner string, channelID, rootTS *string) string {
	if cluster == "" || namespace == "" || spawner == "" || channelID == nil || rootTS == nil || *channelID == "" || *rootTS == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{cluster, namespace, spawner, *channelID, *rootTS}, "\n")))
	return "slack-" + hex.EncodeToString(sum[:])[:24]
}

func fallbackSessionID(prefix, uid, namespace, name string) string {
	if uid != "" {
		return prefix + "-" + uid
	}
	return prefix + "-" + namespace + "-" + name
}

func taskSpawnerName(labels map[string]string, owners []metav1.OwnerReference) string {
	if labels != nil && labels[labelTaskSpawner] != "" {
		return labels[labelTaskSpawner]
	}
	for _, owner := range owners {
		if owner.Kind == "TaskSpawner" && owner.Name != "" {
			return owner.Name
		}
	}
	return "unknown"
}

func labelsFrom(resourceLabels map[string]string, spawnerMeta *TaskSpawnerMeta) map[string]string {
	labels := map[string]string{}
	for k, v := range resourceLabels {
		labels[k] = v
	}
	if spawnerMeta != nil {
		for k, v := range spawnerMeta.Labels {
			if _, ok := labels[k]; !ok {
				labels[k] = v
			}
		}
	}
	return labels
}

func personaFor(spawner string, labels map[string]string) string {
	if labels != nil && labels[labelPersona] != "" {
		return labels[labelPersona]
	}
	if labels != nil && labels[labelAlpheyaPersona] != "" {
		return labels[labelAlpheyaPersona]
	}
	name := strings.ToLower(spawner)
	switch {
	case strings.Contains(name, "debug-alpha"):
		return "debug-alpha"
	case strings.Contains(name, "debug"):
		return "debug"
	case strings.Contains(name, "dev"):
		return "dev"
	case strings.Contains(name, "review"):
		return "review"
	case strings.Contains(name, "ticket"):
		return "ticket"
	default:
		return "unknown"
	}
}

func sessionStatusFromTaskPhase(phase kelosv1alpha1.TaskPhase) string {
	switch phase {
	case kelosv1alpha1.TaskPhaseSucceeded, kelosv1alpha1.TaskPhaseFailed:
		return "closed"
	default:
		return "active"
	}
}

func sessionStatusFromAgentSessionPhase(phase kelosv1alpha1.AgentSessionPhase) string {
	switch phase {
	case kelosv1alpha1.AgentSessionPhaseClosed, kelosv1alpha1.AgentSessionPhaseError:
		return "closed"
	default:
		return "active"
	}
}

func parseIntMetric(results map[string]string, keys ...string) *int64 {
	value := metricValue(results, keys...)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		parseErrorsTotal.WithLabelValues("int").Inc()
		return nil
	}
	return &parsed
}

func parseDecimalMetric(results map[string]string, keys ...string) *string {
	value := metricValue(results, keys...)
	if value == "" {
		return nil
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err != nil {
		parseErrorsTotal.WithLabelValues("decimal").Inc()
		return nil
	}
	clean := strings.TrimSpace(value)
	return &clean
}

func parseStringMetric(results map[string]string, keys ...string) *string {
	value := metricValue(results, keys...)
	return valuePtr(value)
}

func metricValue(results map[string]string, keys ...string) string {
	if len(results) == 0 {
		return ""
	}
	for _, key := range keys {
		if v := strings.TrimSpace(results[key]); v != "" {
			return v
		}
	}
	return ""
}

func durationSeconds(started, completed *time.Time) *float64 {
	if started == nil || completed == nil {
		return nil
	}
	seconds := completed.Sub(*started).Seconds()
	if seconds < 0 {
		return nil
	}
	return &seconds
}

func valuePtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := value
	return &v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func metav1TimePtr(t *metav1.Time) *time.Time {
	if t == nil {
		return nil
	}
	value := t.Time
	if value.IsZero() {
		return nil
	}
	return &value
}

func ptrStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func requireRecordBasics(session SessionRecord, turn TurnRecord) error {
	if session.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if turn.TurnID == "" {
		return fmt.Errorf("turn_id is required")
	}
	if turn.SessionID == "" {
		return fmt.Errorf("turn session_id is required")
	}
	return nil
}
