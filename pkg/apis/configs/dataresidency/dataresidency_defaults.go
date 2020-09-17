/*
Copyright 2020 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dataresidency

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	// configName is the name of config map for the default data residency that
	// GCP resources should use.
	configName = "config-dataresidency"

	// defaulterKey is the key in the ConfigMap to get the name of the default
	// DataResidency setting.
	defaulterKey = "default-dataresidency-config"
)

// ConfigMapName returns the name of the configmap to read for default data residency settings.
func ConfigMapName() string {
	return configName
}

// NewDefaultsConfigFromConfigMap creates a Defaults from the supplied configMap.
func NewDefaultsConfigFromConfigMap(config *corev1.ConfigMap) (*Defaults, error) {
	return NewDefaultsConfigFromMap(config.Data)
}

// NewDefaultsConfigFromMap creates a Defaults from the supplied Map.
func NewDefaultsConfigFromMap(data map[string]string) (*Defaults, error) {
	nc := &Defaults{}

	// Parse out the data residency configuration.
	value, present := data[defaulterKey]
	// Data residency configuration is not required, in which case it will mean
	// allow all regions, so we simply use an empty one
	if !present || value == "" {
		return nc, nil
	}
	if err := parseEntry(value, nc); err != nil {
		return nil, fmt.Errorf("failed to parse the entry: %s", err)
	}
	return nc, nil
}

func parseEntry(entry string, out interface{}) error {
	j, err := yaml.YAMLToJSON([]byte(entry))
	if err != nil {
		return fmt.Errorf("ConfigMap's value could not be converted to JSON: %s : %v", err, entry)
	}
	return json.Unmarshal(j, &out)
}
