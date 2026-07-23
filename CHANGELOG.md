## Unreleased

* Restructured the project into a standard Go layout (`cmd/jobstats_cgroup_exporter`
  and `internal/collector`).
* Renamed the Go module to `github.com/edingc/jobstats_cgroup_exporter`.
* Added TLS and HTTP basic authentication support via the Prometheus
  exporter-toolkit (`--web.config.file`).
* Added graceful shutdown on `SIGTERM`/`SIGINT`, a `/health` endpoint, and a
  landing page at `/`.
* Added `--web.telemetry-path`, `--web.max-requests`, and `--web.enable-pprof`
  flags. `--web.listen-address` is now provided by the exporter-toolkit and may
  be repeated.
* Added `cgroup_scrape_success` and `cgroup_scrape_duration_seconds` metrics.
* Added `cgroup_exporter_collect_error` to the collector's `Describe` output.
* Switched to CGO-free static builds and added `linux/arm64` release artifacts.
* Updated to Go 1.25.
* Added unit tests, a `golangci-lint` configuration, a `Dockerfile`, a `VERSION`
  file, systemd/Prometheus examples, and GitHub Actions workflows for testing,
  releases, and Docker image publishing.

## v0.9.0-alpha / 2026-02-09

* Initial release based on [treydock/cgroup_exporter v1.0.1](https://github.com/treydock/cgroup_exporter/tree/v1.0.1).
* Updated to go v1.24.0.
* Replaced legacy `go-kit` logging with `log/slog`.
* Added support for tracking job step and task needed by Jobstats.
* Added UID metric needed for Jobstats.
* Removed support for gathering process information.
* Removed support for cgroupv1.
* Fixed calculation of memory RSS for cgroupv2.
* Disabled exporter metrics by default.