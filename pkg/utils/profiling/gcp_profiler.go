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

import (
	"cloud.google.com/go/profiler"
)

// GCPProfilerEnvConfig defines environment variables for enabling and
// configuring the GCP Profiler. Intended to be embedded in per-binary
// envconfig struct types.
type GCPProfilerEnvConfig struct {
	// GCPProfiler enables the Google Cloud Profiler when GCP_PROFILER=1
	// (or any other true value that strconv.ParseBool supports). Defaults to
	// false.
	GCPProfiler bool `envconfig:"GCP_PROFILER"`
	// GCPProfilerProject sets the GCP project to use when profiling with
	// GCP_PROFILER_PROJECT. Optional on GCP.
	GCPProfilerProject string `envconfig:"GCP_PROFILER_PROJECT"`
	// GCPProfilerServiceVersion sets the version string of the service being
	// profiled with GCP_PROFILER_SERVICE_VERSION. Defaults to 0.1.
	GCPProfilerServiceVersion string `envconfig:"GCP_PROFILER_SERVICE_VERSION" default:"0.1"`
	// GCPProfilerDebugLogging enables debug logging when
	// GCP_PROFILER_DEBUG_LOGGING=1 (or any other true value that
	// strconv.ParseBool supports). Defaults to false.
	GCPProfilerDebugLogging bool `envconfig:"GCP_PROFILER_DEBUG_LOGGING"`
}

// GCPProfilerEnabled returns true if the config enables the GCP Profiler.
func (c GCPProfilerEnvConfig) GCPProfilerEnabled() bool {
	return c.GCPProfiler
}

// StartGCPProfiler starts the GCP Profiler if enabled in the given EnvConfig
// struct. Returns an error or nil if the GCP Profiler is disabled.
func StartGCPProfiler(serviceName string, env GCPProfilerEnvConfig) error {
	if env.GCPProfilerEnabled() {
		return profiler.Start(profiler.Config{
			Service:        serviceName,
			ServiceVersion: env.GCPProfilerServiceVersion,
			ProjectID:      env.GCPProfilerProject, // optional on GCP
			DebugLogging:   env.GCPProfilerDebugLogging,
		})
	}
	return nil
}
