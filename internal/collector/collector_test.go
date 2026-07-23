// Copyright 2026 Grand Valley State University
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
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// fixedCollector wraps an Exporter but emits a fixed set of CgroupMetrics
// instead of reading the live cgroup filesystem, so metric emission can be
// verified with testutil.CollectAndCompare.
type fixedCollector struct {
	e       *Exporter
	metrics []CgroupMetric
}

func (f *fixedCollector) Describe(ch chan<- *prometheus.Desc) { f.e.Describe(ch) }
func (f *fixedCollector) Collect(ch chan<- prometheus.Metric) { f.e.emitMetrics(ch, f.metrics) }

func TestEmitMetrics_JobLevel(t *testing.T) {
	e := NewExporter([]string{"/system.slice/slurmstepd.scope"}, testLogger())
	fc := &fixedCollector{
		e: e,
		metrics: []CgroupMetric{
			{
				name:        "/system.slice/slurmstepd.scope/job_123",
				cpuUser:     1.5,
				cpuTotal:    2,
				cpus:        4,
				cpu_list:    "0-3",
				memoryUsed:  2048,
				memoryTotal: 4096,
				uid:         "1000",
				username:    "alice",
				jobid:       "123",
			},
		},
	}

	expected := `
# HELP cgroup_cpu_info Information about the cgroup CPUs
# TYPE cgroup_cpu_info gauge
cgroup_cpu_info{cgroup="/system.slice/slurmstepd.scope/job_123",cpus="0-3",jobid="123"} 1
# HELP cgroup_cpu_user_seconds Cumalitive CPU user seconds for cgroup
# TYPE cgroup_cpu_user_seconds gauge
cgroup_cpu_user_seconds{cgroup="/system.slice/slurmstepd.scope/job_123",jobid="123",step="",task=""} 1.5
# HELP cgroup_info User slice information
# TYPE cgroup_info gauge
cgroup_info{cgroup="/system.slice/slurmstepd.scope/job_123",jobid="123",uid="1000",username="alice"} 1
# HELP cgroup_memory_used_bytes Memory used in bytes
# TYPE cgroup_memory_used_bytes gauge
cgroup_memory_used_bytes{cgroup="/system.slice/slurmstepd.scope/job_123",jobid="123",step="",task=""} 2048
# HELP cgroup_uid Uid number of user running this job
# TYPE cgroup_uid gauge
cgroup_uid{jobid="123",username="alice"} 1000
`

	if err := testutil.CollectAndCompare(fc, strings.NewReader(expected),
		"cgroup_cpu_info", "cgroup_cpu_user_seconds", "cgroup_info", "cgroup_memory_used_bytes", "cgroup_uid"); err != nil {
		t.Error(err)
	}
}

// TestEmitMetrics_StepLevel verifies that uid/info metrics are only emitted at
// the job level (empty step and task), not for individual steps/tasks.
func TestEmitMetrics_StepLevel(t *testing.T) {
	e := NewExporter([]string{"/system.slice/slurmstepd.scope"}, testLogger())
	fc := &fixedCollector{
		e: e,
		metrics: []CgroupMetric{
			{
				name:  "/system.slice/slurmstepd.scope/job_123/step_0/user/task_0",
				jobid: "123",
				step:  "0",
				task:  "0",
				uid:   "1000",
			},
		},
	}

	if got := testutil.CollectAndCount(fc, "cgroup_uid"); got != 0 {
		t.Errorf("expected no cgroup_uid at step level, got %d", got)
	}
	if got := testutil.CollectAndCount(fc, "cgroup_info"); got != 0 {
		t.Errorf("expected no cgroup_info at step level, got %d", got)
	}
}

func TestEmitMetrics_CollectError(t *testing.T) {
	e := NewExporter([]string{"/system.slice/slurmstepd.scope"}, testLogger())
	fc := &fixedCollector{
		e: e,
		metrics: []CgroupMetric{
			{name: "/system.slice/slurmstepd.scope", err: true},
		},
	}

	expected := `
# HELP cgroup_exporter_collect_error Indicates collection error, 0=no error, 1=error
# TYPE cgroup_exporter_collect_error gauge
cgroup_exporter_collect_error{cgroup="/system.slice/slurmstepd.scope"} 1
`
	if err := testutil.CollectAndCompare(fc, strings.NewReader(expected), "cgroup_exporter_collect_error"); err != nil {
		t.Error(err)
	}
}

// TestCollect_EmitsScrapeMeta ensures Collect always emits the scrape meta
// metrics even when there are no cgroups to read.
func TestCollect_EmitsScrapeMeta(t *testing.T) {
	// Point CgroupRoot at an empty dir so collectv2 finds nothing to load.
	dir := t.TempDir()
	*CgroupRoot = dir

	e := NewExporter([]string{"/nonexistent"}, testLogger())

	if got := testutil.CollectAndCount(e, "cgroup_scrape_success"); got != 1 {
		t.Errorf("expected exactly 1 cgroup_scrape_success sample, got %d", got)
	}
	if got := testutil.CollectAndCount(e, "cgroup_scrape_duration_seconds"); got != 1 {
		t.Errorf("expected exactly 1 cgroup_scrape_duration_seconds sample, got %d", got)
	}
}
