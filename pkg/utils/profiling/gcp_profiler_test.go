/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package profiling

import "testing"

const (
	// Tricky to test this for real, because the profiler doesn't have a way to
	// check if the profiler is started, or to stop it. So we'll simulate by
	// giving it an empty service name that will cause profiler.Start to return an
	// error.
	invalidServiceName = ""
)

var (
	disabledEnv = GCPProfilerEnvConfig{GCPProfiler: false}
	enabledEnv  = GCPProfilerEnvConfig{GCPProfiler: true}
)

func TestStartGCPProfilerDisabled(t *testing.T) {
	if err := StartGCPProfiler(invalidServiceName, disabledEnv); err != nil {
		t.Errorf("Expected StartGCPProfiler to return nil, got %v", err)
	}
}

func TestStartGCPProfilerEnabled(t *testing.T) {
	if err := StartGCPProfiler(invalidServiceName, enabledEnv); err == nil {
		t.Error("Expected StartGCPProfiler to return an error, got nil")
	}
}

func TestGCPProfilerEnabled(t *testing.T) {
	if disabledEnv.GCPProfilerEnabled() {
		t.Errorf("Expected profiler to be disabled for %#v", disabledEnv)
	}
	if !enabledEnv.GCPProfilerEnabled() {
		t.Errorf("Expected profiler to be enabled for %#v", enabledEnv)
	}
}
