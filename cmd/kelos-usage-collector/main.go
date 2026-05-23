package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	kelosv1alpha1 "github.com/kelos-dev/kelos/api/v1alpha1"
	"github.com/kelos-dev/kelos/internal/logging"
	"github.com/kelos-dev/kelos/internal/usage"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(kelosv1alpha1.AddToScheme(scheme))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: kelos-usage-collector <run|backfill-loki>")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	case "backfill-loki":
		backfillLoki(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var (
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		migrate              bool
		namespace            string
		databaseURL          string
		cluster              string
		instance             string
	)
	fs.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for the usage collector.")
	fs.BoolVar(&migrate, "migrate", true, "Run embedded PostgreSQL migrations on startup.")
	fs.StringVar(&namespace, "namespace", "", "Namespace to watch. Empty watches all namespaces.")
	fs.StringVar(&databaseURL, "database-url", "", "PostgreSQL connection URL. Falls back to DATABASE_URL.")
	fs.StringVar(&cluster, "cluster", "", "Cluster label for usage rows. Falls back to KELOS_USAGE_CLUSTER.")
	fs.StringVar(&instance, "instance", "", "Instance label for logs. Falls back to KELOS_USAGE_INSTANCE.")
	opts, applyVerbosity := logging.SetupZapOptions(fs)
	_ = fs.Parse(args)

	if err := applyVerbosity(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(opts)))

	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if cluster == "" {
		cluster = os.Getenv("KELOS_USAGE_CLUSTER")
	}
	if instance == "" {
		instance = os.Getenv("KELOS_USAGE_INSTANCE")
	}
	if databaseURL == "" {
		setupLog.Error(fmt.Errorf("missing database URL"), "DATABASE_URL or --database-url is required")
		os.Exit(2)
	}
	if cluster == "" {
		setupLog.Error(fmt.Errorf("missing cluster"), "KELOS_USAGE_CLUSTER or --cluster is required")
		os.Exit(2)
	}
	if instance == "" {
		instance = "cody"
	}

	ctx := ctrl.SetupSignalHandler()
	store, err := usage.NewStore(ctx, databaseURL)
	if err != nil {
		setupLog.Error(err, "Unable to connect to PostgreSQL")
		os.Exit(1)
	}
	defer store.Close()
	if migrate {
		if err := store.ApplyMigrations(ctx); err != nil {
			setupLog.Error(err, "Unable to apply migrations")
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "kelos-usage-collector",
	})
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}
	collector := &usage.Collector{
		Client:    mgr.GetClient(),
		Store:     store,
		Cluster:   cluster,
		Namespace: namespace,
	}
	if err := collector.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to set up usage collector")
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("postgres", func(req *http.Request) error {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		return store.Ping(ctx)
	}); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting usage collector", "cluster", cluster, "instance", instance, "namespace", namespace)
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Problem running usage collector")
		os.Exit(1)
	}
}

func backfillLoki(args []string) {
	fs := flag.NewFlagSet("backfill-loki", flag.ExitOnError)
	var (
		databaseURL     string
		lokiURL         string
		lokiTenantID    string
		cluster         string
		instance        string
		namespace       string
		fromValue       string
		toValue         string
		agentQuery      string
		slackQuery      string
		controllerQuery string
		limit           int
		parallelism     int
		checkpointKey   string
		dryRun          bool
		migrate         bool
		skipKubernetes  bool
	)
	fs.StringVar(&databaseURL, "database-url", "", "PostgreSQL connection URL. Falls back to DATABASE_URL.")
	fs.StringVar(&lokiURL, "loki-url", "", "Loki base URL.")
	fs.StringVar(&lokiTenantID, "loki-tenant-id", "", "Optional Loki tenant ID sent as X-Scope-OrgID.")
	fs.StringVar(&cluster, "cluster", "", "Cluster label for usage rows. Falls back to KELOS_USAGE_CLUSTER.")
	fs.StringVar(&instance, "instance", "", "Instance label for logs. Falls back to KELOS_USAGE_INSTANCE.")
	fs.StringVar(&namespace, "namespace", "", "Kelos namespace to backfill.")
	fs.StringVar(&fromValue, "from", "", "Backfill start time in RFC3339 format.")
	fs.StringVar(&toValue, "to", "", "Backfill end time in RFC3339 format.")
	fs.StringVar(&agentQuery, "agent-query", usage.DefaultAgentQuery, "Agent-pod LogQL selector.")
	fs.StringVar(&slackQuery, "slack-query", usage.DefaultSlackQuery, "Slack server LogQL selector.")
	fs.StringVar(&controllerQuery, "controller-query", usage.DefaultControllerQuery, "Kelos controller LogQL selector.")
	fs.IntVar(&limit, "limit", 5000, "Loki page size.")
	fs.IntVar(&parallelism, "parallelism", 2, "Number of query workers. Reserved for future use.")
	fs.StringVar(&checkpointKey, "checkpoint-key", "", "Key in cody_usage_collector_offsets to update after a successful write.")
	fs.BoolVar(&dryRun, "dry-run", false, "Parse and summarize without writing to PostgreSQL.")
	fs.BoolVar(&migrate, "migrate", true, "Run embedded PostgreSQL migrations before writing.")
	fs.BoolVar(&skipKubernetes, "skip-kubernetes", false, "Skip the live Kubernetes resource merge.")
	opts, applyVerbosity := logging.SetupZapOptions(fs)
	_ = fs.Parse(args)

	if err := applyVerbosity(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(opts)))

	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if cluster == "" {
		cluster = os.Getenv("KELOS_USAGE_CLUSTER")
	}
	if instance == "" {
		instance = os.Getenv("KELOS_USAGE_INSTANCE")
	}
	if instance == "" {
		instance = "cody"
	}

	from, err := parseTimeFlag("--from", fromValue)
	if err != nil {
		setupLog.Error(err, "Invalid time flag")
		os.Exit(2)
	}
	to, err := parseTimeFlag("--to", toValue)
	if err != nil {
		setupLog.Error(err, "Invalid time flag")
		os.Exit(2)
	}
	if databaseURL == "" {
		setupLog.Error(fmt.Errorf("missing database URL"), "DATABASE_URL or --database-url is required")
		os.Exit(2)
	}

	ctx := ctrl.SetupSignalHandler()
	var store *usage.Store
	if !dryRun {
		store, err = usage.NewStore(ctx, databaseURL)
		if err != nil {
			setupLog.Error(err, "Unable to connect to PostgreSQL")
			os.Exit(1)
		}
		defer store.Close()
		if migrate {
			if err := store.ApplyMigrations(ctx); err != nil {
				setupLog.Error(err, "Unable to apply migrations")
				os.Exit(1)
			}
		}
	}

	if !skipKubernetes && store != nil {
		if err := syncLiveKubernetes(ctx, store, cluster, namespace); err != nil {
			setupLog.Error(err, "Live Kubernetes resource merge failed; continuing with Loki backfill")
		}
	}

	summary, err := usage.RunLokiBackfill(ctx, store, usage.LokiBackfillOptions{
		DatabaseURL:     databaseURL,
		LokiURL:         lokiURL,
		LokiTenantID:    lokiTenantID,
		Cluster:         cluster,
		Instance:        instance,
		Namespace:       namespace,
		From:            from,
		To:              to,
		AgentQuery:      agentQuery,
		SlackQuery:      slackQuery,
		ControllerQuery: controllerQuery,
		Limit:           limit,
		Parallelism:     parallelism,
		CheckpointKey:   checkpointKey,
		DryRun:          dryRun,
	})
	if err != nil {
		setupLog.Error(err, "Loki backfill failed")
		os.Exit(1)
	}
	fmt.Printf("agent_entries=%d slack_entries=%d controller_entries=%d sessions=%d turns=%d partial_rows=%d dry_run=%t\n",
		summary.AgentEntries,
		summary.SlackEntries,
		summary.ControllerEntries,
		summary.Sessions,
		summary.Turns,
		summary.PartialRows,
		dryRun,
	)
}

func syncLiveKubernetes(ctx context.Context, store *usage.Store, cluster, namespace string) error {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return err
	}
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}
	collector := &usage.Collector{
		Client:    cl,
		Store:     store,
		Cluster:   cluster,
		Namespace: namespace,
	}
	return collector.SyncAll(ctx)
}

func parseTimeFlag(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("%s is required", name)
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing %s: %w", name, err)
	}
	return parsed, nil
}
