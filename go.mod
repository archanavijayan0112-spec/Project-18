package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s-scheduler/pkg/config"
	"k8s-scheduler/pkg/metrics"
	"k8s-scheduler/pkg/scheduler"

	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig     string
	schedulerName  string
	policy         string
	metricsAddr    string
	logLevel       string
	resyncPeriod   time.Duration
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (leave empty for in-cluster)")
	flag.StringVar(&schedulerName, "scheduler-name", "custom-scheduler", "Name of this scheduler")
	flag.StringVar(&policy, "policy", "bin-packing", "Scheduling policy: bin-packing | load-balancing | affinity")
	flag.StringVar(&metricsAddr, "metrics-addr", ":9090", "Address to expose Prometheus metrics")
	flag.StringVar(&logLevel, "log-level", "info", "Log level: debug | info | warn | error")
	flag.DurationVar(&resyncPeriod, "resync-period", 30*time.Second, "Informer resync period")
}

func main() {
	flag.Parse()

	logger, err := buildLogger(logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting custom Kubernetes scheduler",
		zap.String("name", schedulerName),
		zap.String("policy", policy),
	)

	// Build kubeconfig
	cfg, err := buildKubeConfig(kubeconfig)
	if err != nil {
		logger.Fatal("Failed to build kube config", zap.Error(err))
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatal("Failed to create clientset", zap.Error(err))
	}

	// Load scheduler configuration
	schedulerCfg := config.DefaultConfig()
	schedulerCfg.SchedulerName = schedulerName
	schedulerCfg.Policy = config.PolicyType(policy)

	// Start metrics server
	metricsServer := metrics.NewServer(metricsAddr, logger)
	go metricsServer.Start()

	// Create shared informer factory
	factory := informers.NewSharedInformerFactory(clientset, resyncPeriod)

	// Build and start scheduler
	sched, err := scheduler.New(
		clientset,
		factory,
		schedulerCfg,
		logger,
		metricsServer.Registry(),
	)
	if err != nil {
		logger.Fatal("Failed to create scheduler", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start informers
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	logger.Info("Cache synced, scheduler ready")

	// Start scheduling loop
	go sched.Run(ctx)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))
	cancel()
	time.Sleep(2 * time.Second)
	logger.Info("Scheduler stopped")
}

func buildKubeConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		// Fallback to default kubeconfig
		return clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	}
	return cfg, nil
}

func buildLogger(level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	switch level {
	case "debug":
		cfg.Level.SetLevel(zap.DebugLevel)
	case "warn":
		cfg.Level.SetLevel(zap.WarnLevel)
	case "error":
		cfg.Level.SetLevel(zap.ErrorLevel)
	default:
		cfg.Level.SetLevel(zap.InfoLevel)
	}
	return cfg.Build()
}
