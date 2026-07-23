# Jobstats-Compatible cgroup Prometheus Exporter

[![GitHub release](https://img.shields.io/github/v/release/edingc/jobstats_cgroup_exporter?include_prereleases&sort=semver)](https://github.com/edingc/jobstats_cgroup_exporter/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/edingc/jobstats_cgroup_exporter/total)

The `jobstats_cgroup_exporter` produces metrics from v2 cgroups specifically for use with [Jobstats](https://princetonuniversity.github.io/jobstats/), a tool used for monitoring resource utilization on Slurm HPC clusters.

This exporter is a modified adaptation of treydock’s excellent [cgroup_exporter](https://github.com/treydock/cgroup_exporter), on which the [Princeton University fork](https://github.com/plazonic/cgroup_exporter) is also based. Unlike the Princeton fork, `jobstats_cgroup_exporter` is based on a later version of treydock’s exporter and fully supports v2 cgroups. Unlike treydock’s exporter, this adaptation **only** supports v2 cgroups. It also removes the ability to track other cgroups or processes outside of Slurm. This reduces the amount of code required and focuses this build solely on providing the metrics needed for Jobstats.

By default, this exporter listens on port `9306`, and all metrics are exposed via the `/metrics` endpoint. A landing page is served at `/` and a health check at `/health`.

# Usage

The exporter scans `/system.slice/slurmstepd.scope` by default. This can be overridden by providing a comma-separated list of paths via the `--config.paths` flag.

For example, if Slurm is compiled to support multiple `slurmd` instances and your cgroup paths are:

`/sys/fs/cgroup/system.slice/<nodename>_slurmstepd.scope`

you must pass:

`--config.paths=/system.slice/<nodename>_slurmstepd.scope`

replacing `<nodename>` with the host’s `slurmd` `NodeName`.

## Command line flags

| Flag | Default | Description |
| --- | --- | --- |
| `--config.paths` | `/system.slice/slurmstepd.scope` | Comma-separated list of cgroup paths to scan. |
| `--web.listen-address` | `:9306` | Address(es) to listen on for the web interface and telemetry. May be repeated. |
| `--web.config.file` | | Path to a TLS / basic-auth [web configuration file](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md). |
| `--web.telemetry-path` | `/metrics` | Path under which to expose metrics. |
| `--web.disable-exporter-metrics` | `true` | Exclude metrics about the exporter itself (`promhttp_*`, `process_*`, `go_*`). |
| `--web.max-requests` | `1` | Maximum number of parallel scrape requests. Use `0` to disable the limit. |
| `--web.enable-pprof` | `false` | Enable `pprof` profiling endpoints under `/debug/pprof`. |
| `--path.cgroup.root` | `/sys/fs/cgroup` | Root path to the cgroup filesystem. |
| `--path.proc.root` | `/proc` | Root path to the proc filesystem. |
| `--log.level` | `info` | Log level: `debug`, `info`, `warn`, `error`. |
| `--log.format` | `logfmt` | Log format: `logfmt` or `json`. |
| `--version` | | Print version information and exit. |

Run `jobstats_cgroup_exporter --help` for the full list.

## TLS and basic authentication

TLS and HTTP basic authentication are provided by the Prometheus
[exporter-toolkit](https://github.com/prometheus/exporter-toolkit). Point
`--web.config.file` at a YAML file to enable them; see the
[web configuration reference](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md)
for the format.

## Running as a service

A sample `systemd` unit and defaults file are provided under
[`examples/systemd`](examples/systemd), and an example Prometheus scrape
configuration under [`examples/prometheus`](examples/prometheus). The exporter
must run as `root` (or with the capability described below) because it reads
other users' cgroups and process information.

## Install

Download the [latest release](https://github.com/edingc/jobstats_cgroup_exporter/releases)

## Build from source

The exporter follows the standard Go project layout: the entrypoint lives in
`cmd/jobstats_cgroup_exporter` and the collector in `internal/collector`. It
targets Linux only (it reads the cgroup v2 filesystem) and builds as a static,
CGO-free binary.

To produce the `jobstats_cgroup_exporter` binary:

```
make build
```

Or build the command directly:

```
go build ./cmd/jobstats_cgroup_exporter
```

Other useful targets: `make test`, `make lint`, `make vet`, `make fmt`, and
`make docker-build`. Run `make help` for the full list.

## Process metrics

The exporter must be able to read system process information:

```
setcap cap_sys_ptrace=eip /<path>/<to>/jobstats_cgroup_exporter
```

If running as a systemd service:

```
AmbientCapabilities=CAP_SYS_PTRACE
```

## Metrics

Example of metrics exposed by this exporter with default settings:

```
# HELP cgroup_cpu_info Information about the cgroup CPUs
# TYPE cgroup_cpu_info gauge
cgroup_cpu_info{cgroup="/system.slice/slurmstepd.scope/job_223478",cpus="0,4,8,12,16,20,24,28",jobid="223478"} 1
cgroup_cpu_info{cgroup="/system.slice/slurmstepd.scope/job_223482",cpus="1,2,5,6,9,10,13,14,18,22,26,30,32,34,36",jobid="223482"} 1
# HELP cgroup_cpu_system_seconds Cumalitive CPU system seconds for cgroup
# TYPE cgroup_cpu_system_seconds gauge
cgroup_cpu_system_seconds{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 30836.499999
cgroup_cpu_system_seconds{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 30824.641046
cgroup_cpu_system_seconds{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 804.392829
cgroup_cpu_system_seconds{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 798.801398
# HELP cgroup_cpu_total_seconds Cumalitive CPU total seconds for cgroup
# TYPE cgroup_cpu_total_seconds gauge
cgroup_cpu_total_seconds{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 671488.627949
cgroup_cpu_total_seconds{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 671471.974181
cgroup_cpu_total_seconds{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 165449.390505
cgroup_cpu_total_seconds{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 165441.533265
# HELP cgroup_cpu_user_seconds Cumalitive CPU user seconds for cgroup
# TYPE cgroup_cpu_user_seconds gauge
cgroup_cpu_user_seconds{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 640652.127949
cgroup_cpu_user_seconds{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 640647.333135
cgroup_cpu_user_seconds{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 164644.997675
cgroup_cpu_user_seconds{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 164642.731867
# HELP cgroup_cpus Number of CPUs in the cgroup
# TYPE cgroup_cpus gauge
cgroup_cpus{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 8
cgroup_cpus{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 0
cgroup_cpus{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 15
cgroup_cpus{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 0
# HELP cgroup_exporter_build_info A metric with a constant '1' value labeled by version, revision, branch, goversion from which cgroup_exporter was built, and the goos and goarch for the build.
# TYPE cgroup_exporter_build_info gauge
cgroup_exporter_build_info{branch="HEAD",goarch="amd64",goos="linux",goversion="go1.25.0",revision="565e497926b1dc69936b5b98bc3f16d5a9265f6c",tags="unknown",version="0.9.0-alpha"} 1
# HELP cgroup_info User slice information
# TYPE cgroup_info gauge
cgroup_info{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",uid="750920098",username="hpcuser1"} 1
cgroup_info{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",uid="209096162",username="hpcuser2"} 1
# HELP cgroup_memory_cache_bytes Memory cache used in bytes
# TYPE cgroup_memory_cache_bytes gauge
cgroup_memory_cache_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 1.3164015616e+10
cgroup_memory_cache_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 1.3163266048e+10
cgroup_memory_cache_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 6.1180706816e+10
cgroup_memory_cache_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 6.1180125184e+10
# HELP cgroup_memory_fail_count Memory fail count
# TYPE cgroup_memory_fail_count gauge
cgroup_memory_fail_count{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 0
cgroup_memory_fail_count{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 0
cgroup_memory_fail_count{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 0
cgroup_memory_fail_count{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 0
# HELP cgroup_memory_rss_bytes Memory RSS used in bytes
# TYPE cgroup_memory_rss_bytes gauge
cgroup_memory_rss_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 1.1304026112e+10
cgroup_memory_rss_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 1.1299147776e+10
cgroup_memory_rss_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 2.997366784e+09
cgroup_memory_rss_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 2.988306432e+09
# HELP cgroup_memory_total_bytes Memory total given to cgroup in bytes
# TYPE cgroup_memory_total_bytes gauge
cgroup_memory_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 3.4359738368e+10
cgroup_memory_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 1.8446744073709552e+19
cgroup_memory_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 6.442450944e+10
cgroup_memory_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 1.8446744073709552e+19
# HELP cgroup_memory_used_bytes Memory used in bytes
# TYPE cgroup_memory_used_bytes gauge
cgroup_memory_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 2.4468041728e+10
cgroup_memory_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 2.4462413824e+10
cgroup_memory_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 6.41780736e+10
cgroup_memory_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 6.4168431616e+10
# HELP cgroup_memsw_total_bytes Swap total given to cgroup in bytes
# TYPE cgroup_memsw_total_bytes gauge
cgroup_memsw_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 0
cgroup_memsw_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 1.8446744073709552e+19
cgroup_memsw_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 0
cgroup_memsw_total_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 1.8446744073709552e+19
# HELP cgroup_memsw_used_bytes Swap used in bytes
# TYPE cgroup_memsw_used_bytes gauge
cgroup_memsw_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478",jobid="223478",step="",task=""} 0
cgroup_memsw_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223478/step_batch/user/task_0",jobid="223478",step="batch",task="0"} 0
cgroup_memsw_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482",jobid="223482",step="",task=""} 0
cgroup_memsw_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_223482/step_batch/user/task_0",jobid="223482",step="batch",task="0"} 0
# HELP cgroup_scrape_duration_seconds Duration of the cgroup scrape in seconds
# TYPE cgroup_scrape_duration_seconds gauge
cgroup_scrape_duration_seconds 0.012345
# HELP cgroup_scrape_success Whether the last cgroup scrape succeeded (1=success, 0=failure)
# TYPE cgroup_scrape_success gauge
cgroup_scrape_success 1
# HELP cgroup_uid Uid number of user running this job
# TYPE cgroup_uid gauge
cgroup_uid{jobid="223478",username="hpcuser1"} 7.50920098e+08
cgroup_uid{jobid="223482",username="hpcuser2"} 2.09096162e+08
```