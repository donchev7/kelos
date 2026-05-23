package usage

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kelosv1alpha1 "github.com/kelos-dev/kelos/api/v1alpha1"
)

// Collector syncs Kelos Kubernetes resources into the usage store.
type Collector struct {
	Client    client.Client
	Store     *Store
	Cluster   string
	Namespace string
}

func (c *Collector) SetupWithManager(mgr ctrl.Manager) error {
	if err := builder.ControllerManagedBy(mgr).
		Named("usage-task").
		For(&kelosv1alpha1.Task{}).
		Complete(&taskReconciler{collector: c}); err != nil {
		return err
	}
	if err := builder.ControllerManagedBy(mgr).
		Named("usage-agentsession").
		For(&kelosv1alpha1.AgentSession{}).
		Complete(&agentSessionReconciler{collector: c}); err != nil {
		return err
	}
	if err := builder.ControllerManagedBy(mgr).
		Named("usage-agentturn").
		For(&kelosv1alpha1.AgentTurn{}).
		Complete(&agentTurnReconciler{collector: c}); err != nil {
		return err
	}
	return mgr.Add(&initialSyncRunnable{collector: c})
}

func (c *Collector) SyncAll(ctx context.Context) error {
	var taskList kelosv1alpha1.TaskList
	if err := c.Client.List(ctx, &taskList, c.listOptions()...); err != nil {
		return fmt.Errorf("listing Tasks: %w", err)
	}
	for i := range taskList.Items {
		if err := c.SyncTask(ctx, &taskList.Items[i]); err != nil {
			return err
		}
	}

	var sessionList kelosv1alpha1.AgentSessionList
	if err := c.Client.List(ctx, &sessionList, c.listOptions()...); err != nil {
		return fmt.Errorf("listing AgentSessions: %w", err)
	}
	for i := range sessionList.Items {
		if err := c.SyncAgentSession(ctx, &sessionList.Items[i]); err != nil {
			return err
		}
	}

	var turnList kelosv1alpha1.AgentTurnList
	if err := c.Client.List(ctx, &turnList, c.listOptions()...); err != nil {
		return fmt.Errorf("listing AgentTurns: %w", err)
	}
	for i := range turnList.Items {
		if err := c.SyncAgentTurn(ctx, &turnList.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collector) SyncTask(ctx context.Context, task *kelosv1alpha1.Task) error {
	if !c.inScope(task.Namespace) {
		return nil
	}
	meta := c.taskSpawnerMeta(ctx, task.Namespace, task.Labels[labelTaskSpawner])
	session, turn := RecordsFromTask(task, c.Cluster, meta)
	if err := c.Store.UpsertSessionAndTurn(ctx, session, turn); err != nil {
		return fmt.Errorf("upserting Task %s/%s usage: %w", task.Namespace, task.Name, err)
	}
	syncResourcesTotal.WithLabelValues("task").Inc()
	return nil
}

func (c *Collector) SyncAgentSession(ctx context.Context, session *kelosv1alpha1.AgentSession) error {
	if !c.inScope(session.Namespace) {
		return nil
	}
	meta := c.taskSpawnerMeta(ctx, session.Namespace, session.Spec.TaskSpawnerRef.Name)
	record := SessionRecordFromAgentSession(session, c.Cluster, meta)
	if err := c.Store.UpsertSession(ctx, record); err != nil {
		return fmt.Errorf("upserting AgentSession %s/%s usage: %w", session.Namespace, session.Name, err)
	}
	syncResourcesTotal.WithLabelValues("agentsession").Inc()
	return nil
}

func (c *Collector) SyncAgentTurn(ctx context.Context, turn *kelosv1alpha1.AgentTurn) error {
	if !c.inScope(turn.Namespace) {
		return nil
	}
	var session kelosv1alpha1.AgentSession
	var sessionPtr *kelosv1alpha1.AgentSession
	if turn.Spec.SessionRef.Name != "" {
		if err := c.Client.Get(ctx, client.ObjectKey{Namespace: turn.Namespace, Name: turn.Spec.SessionRef.Name}, &session); err == nil {
			sessionPtr = &session
		} else if !apierrors.IsNotFound(err) {
			return fmt.Errorf("fetching AgentSession %s/%s for turn %s: %w", turn.Namespace, turn.Spec.SessionRef.Name, turn.Name, err)
		}
	}
	spawnerName := turn.Labels[labelTaskSpawner]
	if sessionPtr != nil && sessionPtr.Spec.TaskSpawnerRef.Name != "" {
		spawnerName = sessionPtr.Spec.TaskSpawnerRef.Name
	}
	meta := c.taskSpawnerMeta(ctx, turn.Namespace, spawnerName)
	sessionRecord, turnRecord := RecordsFromAgentTurn(turn, sessionPtr, c.Cluster, meta)
	if err := c.Store.UpsertSessionAndTurn(ctx, sessionRecord, turnRecord); err != nil {
		return fmt.Errorf("upserting AgentTurn %s/%s usage: %w", turn.Namespace, turn.Name, err)
	}
	syncResourcesTotal.WithLabelValues("agentturn").Inc()
	return nil
}

func (c *Collector) taskSpawnerMeta(ctx context.Context, namespace, name string) *TaskSpawnerMeta {
	if name == "" {
		return nil
	}
	var spawner kelosv1alpha1.TaskSpawner
	if err := c.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &spawner); err != nil {
		return nil
	}
	return &TaskSpawnerMeta{Name: spawner.Name, Labels: spawner.Labels}
}

func (c *Collector) inScope(namespace string) bool {
	return c.Namespace == "" || c.Namespace == namespace
}

func (c *Collector) listOptions() []client.ListOption {
	if c.Namespace == "" {
		return nil
	}
	return []client.ListOption{client.InNamespace(c.Namespace)}
}

type initialSyncRunnable struct {
	collector *Collector
}

func (r *initialSyncRunnable) Start(ctx context.Context) error {
	log := ctrl.Log.WithName("usage-initial-sync")
	if err := r.collector.SyncAll(ctx); err != nil {
		log.Error(err, "Initial usage sync failed")
	} else {
		log.Info("Initial usage sync completed")
	}
	<-ctx.Done()
	return nil
}

func (r *initialSyncRunnable) NeedLeaderElection() bool { return true }

type taskReconciler struct {
	collector *Collector
}

func (r *taskReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var task kelosv1alpha1.Task
	if err := r.collector.Client.Get(ctx, req.NamespacedName, &task); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if err := r.collector.SyncTask(ctx, &task); err != nil {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, err
	}
	return reconcile.Result{}, nil
}

type agentSessionReconciler struct {
	collector *Collector
}

func (r *agentSessionReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var session kelosv1alpha1.AgentSession
	if err := r.collector.Client.Get(ctx, req.NamespacedName, &session); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if err := r.collector.SyncAgentSession(ctx, &session); err != nil {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, err
	}
	return reconcile.Result{}, nil
}

type agentTurnReconciler struct {
	collector *Collector
}

func (r *agentTurnReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var turn kelosv1alpha1.AgentTurn
	if err := r.collector.Client.Get(ctx, req.NamespacedName, &turn); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if err := r.collector.SyncAgentTurn(ctx, &turn); err != nil {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, err
	}
	return reconcile.Result{}, nil
}
