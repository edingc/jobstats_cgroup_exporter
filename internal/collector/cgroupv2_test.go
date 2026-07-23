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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestParseCpuSet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"single", "3", []string{"3"}},
		{"range", "0-3", []string{"0", "1", "2", "3"}},
		{"list", "0,2,4", []string{"0", "2", "4"}},
		{"mixed", "0-2,5,7-8", []string{"0", "1", "2", "5", "7", "8"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCpuSet(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCpuSet(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseCpuSet_Invalid(t *testing.T) {
	if _, err := parseCpuSet("not-a-number"); err == nil {
		t.Error("expected error for non-numeric cpuset, got nil")
	}
}

func TestGetNamev2(t *testing.T) {
	logger := testLogger()
	tests := []struct {
		name    string
		pidPath string
		path    string
		want    []string
	}{
		{
			name:    "slurm task path yields full and job level",
			pidPath: "/job_123/step_0/user/task_0",
			path:    "/system.slice/slurmstepd.scope",
			want:    []string{"/job_123/step_0/user/task_0", "/job_123"},
		},
		{
			name:    "slurm path without job_ yields just the path",
			pidPath: "/system.slice/slurmstepd.scope",
			path:    "/system.slice/slurmstepd.scope",
			want:    []string{"/system.slice/slurmstepd.scope"},
		},
		{
			name:    "non-slurm path yields just the path",
			pidPath: "/foo/bar",
			path:    "/some/other/path",
			want:    []string{"/foo/bar"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNamev2(tt.pidPath, tt.path, logger)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNamev2(%q, %q) = %v, want %v", tt.pidPath, tt.path, got, tt.want)
			}
		})
	}
}

func TestGetStatv2(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.stat")
	if err := os.WriteFile(path, []byte("anon 4096\nfile 8192\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	got, err := getStatv2("file", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 8192 {
		t.Errorf("getStatv2(file) = %v, want 8192", got)
	}

	if _, err := getStatv2("missing", path); err == nil {
		t.Error("expected error for missing key, got nil")
	}

	if _, err := getStatv2("anon", filepath.Join(dir, "does-not-exist")); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestGetCPUs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cpuset.cpus")
	if err := os.WriteFile(path, []byte("0-3\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	got, err := getCPUs(path, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"0", "1", "2", "3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("getCPUs = %v, want %v", got, want)
	}

	// Missing file returns nil, nil (treated as not applicable).
	got, err = getCPUs(filepath.Join(dir, "nope"), testLogger())
	if err != nil || got != nil {
		t.Errorf("getCPUs(missing) = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	if !fileExists(file) {
		t.Error("expected fileExists(file) = true")
	}
	if fileExists(dir) {
		t.Error("expected fileExists(dir) = false")
	}
	if fileExists(filepath.Join(dir, "missing")) {
		t.Error("expected fileExists(missing) = false")
	}
}

func TestSliceContains(t *testing.T) {
	strs := []string{"a", "b", "c"}
	if !sliceContains(strs, "b") {
		t.Error("expected sliceContains to find b")
	}
	if sliceContains(strs, "z") {
		t.Error("expected sliceContains not to find z")
	}

	ints := []int{1, 2, 3}
	if !sliceContains(ints, 3) {
		t.Error("expected sliceContains to find 3")
	}
}
