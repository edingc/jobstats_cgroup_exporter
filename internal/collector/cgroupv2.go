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
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/prometheus/procfs"
)

var (
	// Use this hack to allow unit tests to override /proc location
	PidGroupPath = cgroup2.PidGroupPath
)

func getInfov2(name string, pids []int, metric *CgroupMetric, logger *slog.Logger) {
	slurmPattern := regexp.MustCompile("/job_([0-9]+)(/step_([^/]+)(/user/task_([0-9]+|special))?)?$")
	slurmMatch := slurmPattern.FindStringSubmatch(name)
	logger.Debug("Got for match", "name", name, "len(slurmMatch)", len(slurmMatch), "slurmMatch", fmt.Sprintf("%v", slurmMatch))
	if len(slurmMatch) == 6 {
		metric.job = true
		metric.jobid = slurmMatch[1]
		metric.step = slurmMatch[3]
		metric.task = slurmMatch[5]
		procFS, err := procfs.NewFS(*ProcRoot)
		if err != nil {
			logger.Error("Unable to get procfs", "root", *ProcRoot, "err", err)
			return
		}
		var proc procfs.Proc
		for _, pid := range pids {
			proc, err = procFS.Proc(pid)
			if err != nil {
				logger.Error("Unable to read PID", "pid", pid, "err", err)
				return
			}
			exec, err := proc.Executable()
			if err != nil {
				logger.Error("Unable to read process executable", "pid", pid, "err", err)
				return
			}
			if filepath.Base(exec) != "sleep" && filepath.Base(exec) != "slurmstepd" {
				break
			}
		}
		procStat, err := proc.NewStatus()
		if err != nil {
			logger.Error("Unable to get proc status for PID", "pid", proc.PID, "err", err)
			return
		}
		// effective UID
		uid := procStat.UIDs[1]
		metric.uid = strconv.FormatUint(uid, 10)
		user, err := user.LookupId(metric.uid)
		if err != nil {
			logger.Error("Error looking up slurm uid", "uid", metric.uid, "err", err)
			return
		}
		metric.username = user.Username
		return
	}
}

func getNamev2(pidPath string, path string, logger *slog.Logger) []string {
	dirs := strings.Split(pidPath, "/")
	var names []string

	if strings.Contains(path, "slurm") {
		// for slurm paths, collect at multiple levels:
		// 1) full task path (e.g., job_X/step_Y/user/task_Z)
		// 2) job level (e.g., job_X)

		// Find the job_* index
		jobIdx := -1
		for i, dir := range dirs {
			if strings.HasPrefix(dir, "job_") {
				jobIdx = i
				break
			}
		}

		if jobIdx >= 0 {
			// Add the full path
			names = append(names, pidPath)

			// Add the job level only (just up to job_X)
			jobLevelDirs := dirs[0 : jobIdx+1]
			jobLevelName := strings.Join(jobLevelDirs, "/")
			names = append(names, jobLevelName)
		} else {
			names = append(names, pidPath)
		}
	} else {
		names = append(names, pidPath)
	}

	logger.Debug("Get names from path", "names", fmt.Sprintf("%v", names), "pidPath", pidPath, "path", path)
	return names
}

func getStatv2(name string, path string) (float64, error) {
	if !fileExists(path) {
		return 0, fmt.Errorf("path %s does not exist", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) != 2 {
			return 0, cgroup2.ErrInvalidFormat
		}
		v, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return 0, cgroup2.ErrInvalidFormat
		}
		if parts[0] == name {
			return float64(v), nil
		}
	}

	if err := s.Err(); err != nil {
		return 0, err
	}

	return 0, fmt.Errorf("unable to find stat key %s in %s", name, path)
}

func (e *Exporter) getMetricsv2(name string, pids []int, opts cgroup2.InitOpts) (CgroupMetric, error) {
	metric := CgroupMetric{name: name}
	e.logger.Debug("Loading cgroup", "path", name)
	ctrl, err := cgroup2.Load(name, opts)
	if err != nil {
		e.logger.Error("Failed to load cgroups", "path", name, "err", err)
		metric.err = true
		return metric, err
	}
	stats, err := ctrl.Stat()
	if err != nil {
		e.logger.Error("Failed to get cgroup stats", "path", name)
		metric.err = true
		return metric, err
	}
	if stats == nil {
		e.logger.Error("Cgroup stats are nil", "path", name)
		metric.err = true
		return metric, err
	}
	if stats.CPU != nil {
		metric.cpuUser = float64(stats.CPU.UserUsec) / 1000000.0
		metric.cpuSystem = float64(stats.CPU.SystemUsec) / 1000000.0
		metric.cpuTotal = float64(stats.CPU.UsageUsec) / 1000000.0
	}
	// TODO: Move to https://github.com/containerd/cgroups/blob/d131035c7599c51ff4aed27903c45eb3b2cc29d0/cgroup2/manager.go#L593
	// memoryStatPath := filepath.Join(*CgroupRoot, name, "memory.stat")
	// swapcached, err := getStatv2("swapcached", memoryStatPath)
	// if err != nil {
	// 	e.logger.Error("Unable to get swapcached", "path", name, "err", err)
	// 	metric.err = true
	// 	return metric, err
	// }
	if stats.Memory != nil {
		// slurm 25.11.1 supposedly fixes this, but this actual gets us "real" used memory and not caches
		//metric.memoryRSS = float64(stats.Memory.Anon) + swapcached + float64(stats.Memory.File)
		metric.memoryRSS = float64(stats.Memory.Usage) - float64(stats.Memory.File)
		metric.memoryUsed = float64(stats.Memory.Usage)
		metric.memoryTotal = float64(stats.Memory.UsageLimit)
		metric.memoryCache = float64(stats.Memory.File)
		metric.memswUsed = float64(stats.Memory.SwapUsage)
		metric.memswTotal = float64(stats.Memory.SwapLimit)
		if stats.MemoryEvents != nil {
			metric.memoryFailCount = float64(stats.MemoryEvents.Oom)
		}
	}
	// TODO: cpuset.cpus.effective?
	cpusPath := filepath.Join(*CgroupRoot, name, "cpuset.cpus")
	if cpus, err := getCPUs(cpusPath, e.logger); err == nil {
		metric.cpus = len(cpus)
		metric.cpu_list = strings.Join(cpus, ",")
	}
	getInfov2(name, pids, &metric, e.logger)
	return metric, nil
}

func (e *Exporter) collectv2() ([]CgroupMetric, error) {
	var names []string
	var metrics []CgroupMetric
	for _, path := range e.paths {
		var group string
		group = path
		e.logger.Debug("Loading cgroup", "path", path, "group", group, "root", *CgroupRoot)
		//TODO
		//control, err := cgroup2.LoadSystemd(path, group)
		opts := cgroup2.WithMountpoint(*CgroupRoot)
		control, err := cgroup2.Load(group, opts)
		if err != nil {
			e.logger.Error("Error loading cgroup", "path", path, "group", group, "err", err)
			metric := CgroupMetric{name: path, err: true}
			metrics = append(metrics, metric)
			continue
		}
		processes, err := control.Procs(true)
		if err != nil {
			e.logger.Error("Error loading cgroup processes", "path", path, "group", group, "err", err)
			metric := CgroupMetric{name: path, err: true}
			metrics = append(metrics, metric)
			continue
		}
		e.logger.Debug("Found processes", "path", path, "group", group, "processes", len(processes))
		pids := make(map[string][]int)
		for _, p := range processes {
			pid := int(p)
			pidPath, err := PidGroupPath(pid)
			if err != nil {
				e.logger.Error("Error getting PID group path", "path", path, "group", group, "pid", pid, "err", err)
				continue
			}
			e.logger.Debug("Get Name", "pid", pid, "path", path)
			nameList := getNamev2(pidPath, path, e.logger)
			for _, name := range nameList {
				if strings.Contains(path, "slurm") && filepath.Base(name) == "system" {
					e.logger.Debug("Skip system cgroup", "name", name)
					continue
				}
				if strings.Contains(path, "slurm") && strings.HasSuffix(name, "/slurm") {
					e.logger.Debug("Skip slurm cgroup", "name", name)
					continue
				}
				if !sliceContains(names, name) {
					names = append(names, name)
				}
				if val, ok := pids[name]; ok {
					if !sliceContains(val, pid) {
						val = append(val, pid)
					}
					pids[name] = val
				} else {
					pids[name] = []int{pid}
				}
			}
		}
		wg := &sync.WaitGroup{}
		wg.Add(len(names))
		for _, name := range names {
			go func(n string, p map[string][]int) {
				defer wg.Done()
				var pids []int
				if val, ok := p[n]; ok {
					pids = val
				} else {
					e.logger.Error("Unable to get PIDs for name", "name", n)
					return
				}
				metric, _ := e.getMetricsv2(n, pids, opts)
				metricLock.Lock()
				metrics = append(metrics, metric)
				metricLock.Unlock()
			}(name, pids)
		}
		wg.Wait()
	}
	return metrics, nil
}
