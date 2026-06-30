package metrics

import (
	"net/http"

	"go.uber.org/zap"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server exposes Prometheus metrics over HTTP
type Server struct {
	addr     string
	registry *prometheus.Registry
	logger   *zap.Logger
}

// NewServer creates a metrics server bound to addr
func NewServer(addr string, logger *zap.Logger) *Server {
	reg := prometheus.NewRegistry()
	// Register standard Go and process collectors
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return &Server{addr: addr, registry: reg, logger: logger}
}

// Registry exposes the prometheus.Registry so the scheduler can register its own counters
func (s *Server) Registry() *prometheus.Registry { return s.registry }

// Start begins serving metrics; blocks until the server exits
func (s *Server) Start() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: s.addr, Handler: mux}
	s.logger.Info("Metrics server listening", zap.String("addr", s.addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("Metrics server error", zap.Error(err))
	}
}
