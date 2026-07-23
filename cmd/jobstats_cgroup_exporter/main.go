// Copyright 2026 Grand Valley State University
// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"

	"github.com/edingc/jobstats_cgroup_exporter/internal/collector"
)

var (
	configPaths            = kingpin.Flag("config.paths", "Comma separated list of cgroup paths to check, e.g. /system.slice/slurmstepd.scope").Default("/system.slice/slurmstepd.scope").String()
	metricsPath            = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	disableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter (promhttp_*, process_*, go_*).").Default("true").Bool()
	maxRequests            = kingpin.Flag("web.max-requests", "Maximum number of parallel scrape requests. Use 0 to disable.").Default("1").Int()
	enablePprof            = kingpin.Flag("web.enable-pprof", "Enable pprof profiling endpoints under /debug/pprof.").Bool()
	toolkitFlags           = kingpinflag.AddFlags(kingpin.CommandLine, ":9306")
)

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("cgroup_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)
	slog.SetDefault(logger)
	logger.Info("Starting cgroup_exporter", "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())

	paths := strings.Split(*configPaths, ",")
	logger.Info("Monitoring cgroup paths", "paths", strings.Join(paths, ", "))

	// Register the collector once. Collection happens lazily at scrape time,
	// so the exporter always reflects live cgroup state.
	reg := prometheus.NewRegistry()
	reg.MustRegister(versioncollector.NewCollector(fmt.Sprintf("%s_exporter", collector.Namespace)))
	reg.MustRegister(collector.NewCgroupV2Collector(paths, logger))

	// Use a dedicated mux instead of DefaultServeMux so that dependencies
	// (e.g. net/http/pprof) can't silently register handlers. pprof is only
	// mounted when explicitly enabled.
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck
		w.Write([]byte(`{"status":"ok"}`))
	})

	if *enablePprof {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	opts := promhttp.HandlerOpts{
		ErrorLog:            slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ErrorHandling:       promhttp.ContinueOnError,
		MaxRequestsInFlight: *maxRequests,
	}

	exporterReg := prometheus.NewRegistry()
	if !*disableExporterMetrics {
		exporterReg.MustRegister(
			promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{}),
			promcollectors.NewGoCollector(),
		)
	}

	var metricsHandler http.Handler = promhttp.HandlerFor(prometheus.Gatherers{exporterReg, reg}, opts)
	if !*disableExporterMetrics {
		metricsHandler = promhttp.InstrumentMetricHandler(exporterReg, metricsHandler)
	}
	metricsHandler = scrapeLoggingMiddleware(metricsHandler, logger)
	mux.Handle(*metricsPath, metricsHandler)

	landingConfig := web.LandingConfig{
		Name:        "Jobstats cgroup v2 Exporter",
		Description: "Prometheus Exporter for Slurm job cgroup v2 resource usage",
		Version:     version.Info(),
		Links: []web.LandingLinks{
			{Address: *metricsPath, Text: "Metrics"},
			{Address: "/health", Text: "Health"},
		},
		Profiling: fmt.Sprintf("%t", *enablePprof),
	}
	landingPage, err := web.NewLandingPage(landingConfig)
	if err != nil {
		logger.Error("Error creating landing page", "err", err)
		os.Exit(1)
	}
	mux.Handle("/", landingPage)

	srv := &http.Server{Handler: mux}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		if err := web.ListenAndServe(srv, toolkitFlags, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Error starting HTTP server", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during server shutdown", "err", err)
	}
}

// scrapeLoggingMiddleware logs the start and end of each /metrics request at debug level.
func scrapeLoggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger.Debug("Scrape request received", "remote_addr", r.RemoteAddr, "user_agent", r.UserAgent())
		next.ServeHTTP(w, r)
		logger.Debug("Scrape request completed", "remote_addr", r.RemoteAddr, "duration", time.Since(start).Round(time.Millisecond))
	})
}
