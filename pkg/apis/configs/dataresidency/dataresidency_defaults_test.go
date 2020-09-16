/*
Copyright 2020 Google LLC.

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

package dataresidency

import (
	"testing"

	"cloud.google.com/go/pubsub"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "knative.dev/pkg/configmap/testing"
	_ "knative.dev/pkg/system/testing"
)

func TestDefaultsConfigurationFromFile(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, configName, defaulterKey)
	if _, err := NewDefaultsConfigFromConfigMap(example); err != nil {
		t.Errorf("NewDefaultsConfigFromConfigMap(example) = %v", err)
	}
}

func TestNewDefaultsConfigFromConfigMap(t *testing.T) {
	_, example := ConfigMapsFromTestFile(t, configName, defaulterKey)
	defaults, err := NewDefaultsConfigFromConfigMap(example)
	if err != nil {
		t.Fatalf("NewDefaultsConfigFromConfigMap(example) = %v", err)
	}

	// Only cluster wide configuration is supported now, but we use the namespace
	// as the test name and for future extension.
	testCases := []struct {
		ns      string
		regions []string
	}{
		{
			ns:      "cluster-wide",
			regions: []string{"us-east1", "us-west1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.ns, func(t *testing.T) {
			if diff := cmp.Diff(tc.regions, defaults.AllowedPersistenceRegions()); diff != "" {
				t.Errorf("Unexpected value (-want +got): %s", diff)
			}
		})
	}
}

func TestComputeAllowedPersistenceRegions(t *testing.T) {
	// Only cluster wide configuration is supported now, but we use the namespace
	// as the test name and for future extension.
	testCases := []struct {
		ns                 string
		topicConfigRegions []string
		dsRegions          []string
		expectedRegions    []string
	}{
		{
			ns:                 "subset",
			topicConfigRegions: []string{"us-east1", "us-west1"},
			dsRegions:          []string{"us-west1"},
			expectedRegions:    []string{"us-west1"},
		},
		{
			ns:                 "conflict",
			topicConfigRegions: []string{"us-east1"},
			dsRegions:          []string{"us-west1"},
			expectedRegions:    []string{"us-west1"},
		},
		{
			ns:                 "topic-nil",
			topicConfigRegions: nil,
			dsRegions:          []string{"us-west1"},
			expectedRegions:    []string{"us-west1"},
		},
		{
			ns:                 "topic-nil-ds-empty",
			topicConfigRegions: nil,
			dsRegions:          []string{},
			expectedRegions:    nil,
		},
		{
			ns:                 "ds-empty",
			topicConfigRegions: []string{"us-east1"},
			dsRegions:          []string{},
			expectedRegions:    []string{"us-east1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.ns, func(t *testing.T) {
			defaults, err := NewDefaultsConfigFromMap(map[string]string{})
			if err != nil {
				t.Fatalf("NewDefaultsConfigFromConfigMap(empty) = %v", err)
			}
			defaults.ClusterDefaults.AllowedPersistenceRegions = tc.dsRegions
			topicConfig := &pubsub.TopicConfig{}
			topicConfig.MessageStoragePolicy.AllowedPersistenceRegions = tc.topicConfigRegions
			defaults.ComputeAllowedPersistenceRegions(topicConfig)
			if diff := cmp.Diff(tc.expectedRegions, topicConfig.MessageStoragePolicy.AllowedPersistenceRegions); diff != "" {
				t.Errorf("Unexpected value (-want +got): %s", diff)
			}
		})
	}
}

func TestNewDefaultsConfigFromConfigMapEmpty(t *testing.T) {
	testCases := map[string]struct {
		name   string
		config *corev1.ConfigMap
	}{
		"empty data": {
			config: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cloud-run-events",
					Name:      configName,
				},
				Data: map[string]string{},
			},
		},
		"missing key": {
			config: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cloud-run-events",
					Name:      configName,
				},
				Data: map[string]string{
					"other-keys": "are-present",
				},
			},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, err := NewDefaultsConfigFromConfigMap(tc.config)
			if err != nil {
				t.Errorf("Empty value or no key should pass")
			}
		})
	}
}
