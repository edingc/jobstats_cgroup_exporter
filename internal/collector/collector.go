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

package collector

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	CgroupRoot         = kingpin.Flag("path.cgroup.root", "Root path to cgroup fs").Default(defCgroupRoot).String()
	collectProcMaxExec = kingpin.Flag("collect.proc.max-exec", "Max length of process executable to record").Default("100").Int()
	ProcRoot           = kingpin.Flag("path.proc.root", "Root path to proc fs").Default(defProcRoot).String()
	metricLock         = sync.RWMutex{}
)

const (
	Namespace     = "cgroup"
	defCgroupRoot = "/sys/fs/cgroup"
	defProcRoot   = "/proc"
)

type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric)
}

type Exporter struct {
	paths           []string
	collectError    *prometheus.Desc
	cpuUser         *prometheus.Desc
	cpuSystem       *prometheus.Desc
	cpuTotal        *prometheus.Desc
	cpus            *prometheus.Desc
	cpuInfo         *prometheus.Desc
	memoryRSS       *prometheus.Desc
	memoryCache     *prometheus.Desc
	memoryUsed      *prometheus.Desc
	memoryTotal     *prometheus.Desc
	memoryFailCount *prometheus.Desc
	memswUsed       *prometheus.Desc
	memswTotal      *prometheus.Desc
	memswFailCount  *prometheus.Desc
	info            *prometheus.Desc
	uid             *prometheus.Desc

	// Scrape meta
	scrapeSuccess         *prometheus.Desc
	scrapeDurationSeconds *prometheus.Desc

	logger *slog.Logger
}

type CgroupMetric struct {
	name            string
	cpuUser         float64
	cpuSystem       float64
	cpuTotal        float64
	cpus            int
	cpu_list        string
	memoryRSS       float64
	memoryCache     float64
	memoryUsed      float64
	memoryTotal     float64
	memoryFailCount float64
	memswUsed       float64
	memswTotal      float64
	memswFailCount  float64
	userslice       bool
	job             bool
	uid             string
	username        string
	jobid           string
	step            string
	task            string
	err             bool
}

func NewCgroupV2Collector(paths []string, logger *slog.Logger) Collector {
	return NewExporter(paths, logger)
}

func NewExporter(paths []string, logger *slog.Logger) *Exporter {
	// label builds a descriptor with the given subsystem, name, help text and
	// variable labels, all under the exporter's namespace.
	label := func(subsystem, name, help string, variableLabels ...string) *prometheus.Desc {
		return prometheus.NewDesc(prometheus.BuildFQName(Namespace, subsystem, name), help, variableLabels, nil)
	}

	cgroupLabels := []string{"cgroup", "jobid", "step", "task"}

	return &Exporter{
		paths:           paths,
		uid:             label("", "uid", "Uid number of user running this job", "jobid", "username"),
		cpuUser:         label("cpu", "user_seconds", "Cumalitive CPU user seconds for cgroup", cgroupLabels...),
		cpuSystem:       label("cpu", "system_seconds", "Cumalitive CPU system seconds for cgroup", cgroupLabels...),
		cpuTotal:        label("cpu", "total_seconds", "Cumalitive CPU total seconds for cgroup", cgroupLabels...),
		cpus:            label("", "cpus", "Number of CPUs in the cgroup", cgroupLabels...),
		cpuInfo:         label("", "cpu_info", "Information about the cgroup CPUs", "cgroup", "cpus", "jobid"),
		memoryRSS:       label("memory", "rss_bytes", "Memory RSS used in bytes", cgroupLabels...),
		memoryCache:     label("memory", "cache_bytes", "Memory cache used in bytes", cgroupLabels...),
		memoryUsed:      label("memory", "used_bytes", "Memory used in bytes", cgroupLabels...),
		memoryTotal:     label("memory", "total_bytes", "Memory total given to cgroup in bytes", cgroupLabels...),
		memoryFailCount: label("memory", "fail_count", "Memory fail count", cgroupLabels...),
		memswUsed:       label("memsw", "used_bytes", "Swap used in bytes", cgroupLabels...),
		memswTotal:      label("memsw", "total_bytes", "Swap total given to cgroup in bytes", cgroupLabels...),
		memswFailCount:  label("memsw", "fail_count", "Swap fail count", cgroupLabels...),
		info:            label("", "info", "User slice information", "cgroup", "username", "uid", "jobid"),
		collectError:    label("exporter", "collect_error", "Indicates collection error, 0=no error, 1=error", "cgroup"),

		scrapeSuccess:         label("scrape", "success", "Whether the last cgroup scrape succeeded (1=success, 0=failure)"),
		scrapeDurationSeconds: label("scrape", "duration_seconds", "Duration of the cgroup scrape in seconds"),

		logger: logger,
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.uid
	ch <- e.cpuUser
	ch <- e.cpuSystem
	ch <- e.cpuTotal
	ch <- e.cpus
	ch <- e.cpuInfo
	ch <- e.memoryRSS
	ch <- e.memoryCache
	ch <- e.memoryUsed
	ch <- e.memoryTotal
	ch <- e.memoryFailCount
	ch <- e.memswUsed
	ch <- e.memswTotal
	ch <- e.memswFailCount
	ch <- e.info
	ch <- e.collectError
	ch <- e.scrapeSuccess
	ch <- e.scrapeDurationSeconds
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	metrics, err := e.collectv2()

	// The scrape is considered a failure if collection returned an error or if
	// any individual cgroup reported an error while being read.
	success := 1.0
	if err != nil {
		success = 0
	}
	for _, m := range metrics {
		if m.err {
			success = 0
			break
		}
	}

	e.emitMetrics(ch, metrics)

	elapsed := time.Since(start)
	e.logger.Debug("Scrape finished", "duration", elapsed.Round(time.Millisecond), "success", success == 1.0)
	ch <- prometheus.MustNewConstMetric(e.scrapeSuccess, prometheus.GaugeValue, success)
	ch <- prometheus.MustNewConstMetric(e.scrapeDurationSeconds, prometheus.GaugeValue, elapsed.Seconds())
}

// emitMetrics writes the const metrics for the collected cgroups to ch. It is
// separated from data gathering so it can be exercised directly in tests.
func (e *Exporter) emitMetrics(ch chan<- prometheus.Metric, metrics []CgroupMetric) {
	for _, m := range metrics {
		if m.err {
			ch <- prometheus.MustNewConstMetric(e.collectError, prometheus.GaugeValue, 1, m.name)
		}

		// unlike princeton's cgroup_exporter, uid is returned as a string not int
		// convert this to the needed float value
		if m.task == "" && m.step == "" {
			uid, _ := strconv.ParseFloat(m.uid, 64)
			ch <- prometheus.MustNewConstMetric(e.uid, prometheus.GaugeValue, uid, m.jobid, m.username)
			ch <- prometheus.MustNewConstMetric(e.info, prometheus.GaugeValue, 1, m.name, m.username, m.uid, m.jobid)
		}
		ch <- prometheus.MustNewConstMetric(e.cpuUser, prometheus.GaugeValue, m.cpuUser, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.cpuSystem, prometheus.GaugeValue, m.cpuSystem, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.cpuTotal, prometheus.GaugeValue, m.cpuTotal, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.cpus, prometheus.GaugeValue, float64(m.cpus), m.name, m.jobid, m.step, m.task)

		// cpu_list will only be populated for parent cgroup
		if m.cpu_list != "" {
			ch <- prometheus.MustNewConstMetric(e.cpuInfo, prometheus.GaugeValue, 1, m.name, m.cpu_list, m.jobid)
		}
		ch <- prometheus.MustNewConstMetric(e.memoryRSS, prometheus.GaugeValue, m.memoryRSS, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.memoryUsed, prometheus.GaugeValue, m.memoryUsed, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.memoryTotal, prometheus.GaugeValue, m.memoryTotal, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.memoryCache, prometheus.GaugeValue, m.memoryCache, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.memoryFailCount, prometheus.GaugeValue, m.memoryFailCount, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.memswUsed, prometheus.GaugeValue, m.memswUsed, m.name, m.jobid, m.step, m.task)
		ch <- prometheus.MustNewConstMetric(e.memswTotal, prometheus.GaugeValue, m.memswTotal, m.name, m.jobid, m.step, m.task)

	}
}

// parseCpuSet expands a Linux cpuset string (e.g. "0-3,7,9-10") into the
// individual CPU IDs it represents. It returns an error for any malformed
// component rather than silently skipping it.
func parseCpuSet(cpuset string) ([]string, error) {
	if cpuset == "" {
		return nil, nil
	}
	var cpus []string
	for _, r := range strings.Split(cpuset, ",") {
		boundaries := strings.Split(r, "-")
		var start, end int
		var err error
		switch len(boundaries) {
		case 1:
			if start, err = strconv.Atoi(boundaries[0]); err != nil {
				return nil, fmt.Errorf("invalid cpuset %q: %w", r, err)
			}
			end = start
		case 2:
			if start, err = strconv.Atoi(boundaries[0]); err != nil {
				return nil, fmt.Errorf("invalid cpuset range %q: %w", r, err)
			}
			if end, err = strconv.Atoi(boundaries[1]); err != nil {
				return nil, fmt.Errorf("invalid cpuset range %q: %w", r, err)
			}
		default:
			return nil, fmt.Errorf("invalid cpuset range %q", r)
		}
		if end < start {
			return nil, fmt.Errorf("invalid cpuset range %q: end before start", r)
		}
		for e := start; e <= end; e++ {
			cpus = append(cpus, strconv.Itoa(e))
		}
	}
	return cpus, nil
}

func getCPUs(path string, logger *slog.Logger) ([]string, error) {
	if !fileExists(path) {
		return nil, nil
	}
	cpusData, err := os.ReadFile(path)
	if err != nil {
		logger.Error("Error reading cpuset", "cpuset", path, "err", err)
		return nil, err
	}
	cpus, err := parseCpuSet(strings.TrimSuffix(string(cpusData), "\n"))
	if err != nil {
		logger.Error("Error parsing cpu set", "cpuset", path, "err", err)
		return nil, err
	}
	return cpus, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func sliceContains(s interface{}, v interface{}) bool {
	slice := reflect.ValueOf(s)
	for i := 0; i < slice.Len(); i++ {
		if slice.Index(i).Interface() == v {
			return true
		}
	}
	return false
}
