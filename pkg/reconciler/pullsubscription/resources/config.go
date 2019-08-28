/*
Copyright 2019 Google LLC

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

package resources

import (
	"encoding/json"
	"fmt"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"strconv"
)

var zapLoggerConfig = "zap-logger-config"

// Base64ToMetricsOptions converts a json+base64 string of a
// metrics.ExporterOptions. Returns a non-nil metrics.ExporterOptions always.
func Base64ToMetricsOptions(base64 string) *metrics.ExporterOptions {
	var opts metrics.ExporterOptions
	if base64 == "" {
		return &opts
	}

	quoted64 := strconv.Quote(string(base64))

	var bytes []byte
	// Do not care about the unmarshal error.
	_ = json.Unmarshal([]byte(quoted64), &bytes)

	// Do not care about the unmarshal error.
	_ = json.Unmarshal(bytes, &opts)

	return &opts
}

// MetricsOptionsToBase64 converts a metrics.ExporterOptions to a json+base64
// string.
func MetricsOptionsToBase64(opts *metrics.ExporterOptions) string {
	if opts == nil {
		return ""
	}

	jsonOpts, err := json.Marshal(opts)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}
	// if we json.Marshal a []byte, we will get back a base64 encoded quoted string.
	base64Opts, err := json.Marshal(jsonOpts)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}

	base64, err := strconv.Unquote(string(base64Opts))
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}
	// Turn the base64 encoded []byte back into a string.
	return base64
}

// Base64ToLoggingConfig converts a json+base64 string of a logging.Config.
// Returns a non-nil logging.Config always.
func Base64ToLoggingConfig(base64 string) *logging.Config {
	if base64 == "" {
		return &logging.Config{}
	}

	quoted64 := strconv.Quote(string(base64))

	var bytes []byte
	// Do not care about the unmarshal error.
	_ = json.Unmarshal([]byte(quoted64), &bytes)

	var configMap map[string]string
	// Do not care about the unmarshal error.
	_ = json.Unmarshal(bytes, &configMap)

	cfg, err := logging.NewConfigFromMap(configMap)
	if err != nil {
		// Get the default config from logging package.
		cfg, _ = logging.NewConfigFromMap(map[string]string{})
	}
	return cfg
}

// LoggingConfigToBase64 converts a logging.Config to a json+base64 string.
func LoggingConfigToBase64(cfg *logging.Config) string {
	if cfg == nil || cfg.LoggingConfig == "" {
		return ""
	}

	jsonCfg, err := json.Marshal(map[string]string{
		zapLoggerConfig: cfg.LoggingConfig,
	})
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}
	// if we json.Marshal a []byte, we will get back a base64 encoded quoted string.
	base64Cfg, err := json.Marshal(jsonCfg)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}

	base64, err := strconv.Unquote(string(base64Cfg))
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}
	// Turn the base64 encoded []byte back into a string.
	return base64
}
