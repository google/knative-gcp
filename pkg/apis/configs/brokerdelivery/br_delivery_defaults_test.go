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

package brokerdelivery

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	v1 "knative.dev/pkg/apis/duck/v1"
	. "knative.dev/pkg/configmap/testing"
	_ "knative.dev/pkg/system/testing"
)

const (
	clusterDefaultedNS = "cluster"
	// customizedNS is the namespace that has customizations in the testdata.
	customizedNS = "customized-ns"
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

	clusterDefaultedBackoffDelay := "PT1S"
	clusterDefaultedBackoffPolicy := eventingduckv1beta1.BackoffPolicyExponential
	clusterDefaultedRetry := int32(10)
	nsDefaultedBackoffDelay := "PT5S"
	nsDefaultedBackoffPolicy := eventingduckv1beta1.BackoffPolicyLinear
	nsDefaultedRetry := int32(20)
	testCases := []struct {
		ns             string
		backoffDelay   *string
		backoffPolicy  *eventingduckv1beta1.BackoffPolicyType
		deadLetterSink *v1.Destination
		retry          *int32
	}{
		{
			ns:            clusterDefaultedNS,
			backoffDelay:  &clusterDefaultedBackoffDelay,
			backoffPolicy: &clusterDefaultedBackoffPolicy,
			deadLetterSink: &v1.Destination{
				URI: &apis.URL{
					Scheme: "pubsub",
					Host:   "cluster-default-dead-letter-topic-id",
				},
			},
			retry: &clusterDefaultedRetry,
		},
		{
			ns:            customizedNS,
			backoffDelay:  &nsDefaultedBackoffDelay,
			backoffPolicy: &nsDefaultedBackoffPolicy,
			deadLetterSink: &v1.Destination{
				URI: &apis.URL{
					Scheme: "pubsub",
					Host:   "ns-default-dead-letter-topic-id",
				},
			},
			retry: &nsDefaultedRetry,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.ns, func(t *testing.T) {
			if diff := cmp.Diff(tc.backoffDelay, defaults.BackoffDelay(tc.ns)); diff != "" {
				t.Errorf("Unexpected value (-want +got): %s", diff)
			}
			if diff := cmp.Diff(tc.backoffPolicy, defaults.BackoffPolicy(tc.ns)); diff != "" {
				t.Errorf("Unexpected value (-want +got): %s", diff)
			}
			if diff := cmp.Diff(tc.deadLetterSink, defaults.DeadLetterSink(tc.ns)); diff != "" {
				t.Errorf("Unexpected value (-want +got): %s", diff)
			}
			if diff := cmp.Diff(tc.retry, defaults.Retry(tc.ns)); diff != "" {
				t.Errorf("Unexpected value (-want +got): %s", diff)
			}
		})
	}
}

func TestNewDefaultsConfigFromConfigMapWithError(t *testing.T) {
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
		"invalid YAML": {
			config: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "cloud-run-events",
					Name:      configName,
				},
				Data: map[string]string{
					"default-br-delivery-config": `
	clusterDefaults: !!binary
`,
				},
			},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			_, err := NewDefaultsConfigFromConfigMap(tc.config)
			if err == nil {
				t.Fatalf("Expected an error, actually nil")
			}
		})
	}
}
