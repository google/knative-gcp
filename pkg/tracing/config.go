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

// TODO Move this to knative/pkg's tracing package.
package tracing

import (
	"encoding/json"
	"errors"
	"fmt"

	tracingconfig "knative.dev/pkg/tracing/config"
)

// JSONToConfig converts a JSON marshaled version of the tracingconfig.Config back to the structure.
// It should round-trip with ConfigToJSON. E.g. cfg == JSONToConfig(ConfigToJSON(cfg))
func JSONToConfig(jsonConfig string) (*tracingconfig.Config, error) {
	var cfg tracingconfig.Config
	if jsonConfig == "" {
		return nil, errors.New("tracing config json string is empty")
	}

	if err := json.Unmarshal([]byte(jsonConfig), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling tracing config json: %w", err)
	}

	return &cfg, nil
}

// ConfigToJSON marshals a tracingconfig.Config to a JSON string. It should round-trip with
// JSONToConfig. E.g. cfg == JSONToConfig(ConfigToJSON(cfg))
func ConfigToJSON(cfg *tracingconfig.Config) (string, error) {
	if cfg == nil {
		return "", nil
	}

	jsonCfg, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	return string(jsonCfg), nil
}
