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

package gcpauth

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "knative.dev/pkg/configmap/testing"
	_ "knative.dev/pkg/system/testing"
)

const (
	clusterDefaultedNS = "cluster"
	// customizedNS is the namespace that has customizations in the testdata.
	customizedNS = "customized-ns"
	// emptyNS is the namespace that is customized in the testdata to have no defaults.
	emptyNS = "empty-ns"
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

	testCases := []struct {
		ns     string
		ksa    string
		secret *corev1.SecretKeySelector
		wi     map[string]string
	}{
		{
			ns:  clusterDefaultedNS,
			ksa: "cluster-default-ksa",
			secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "google-cloud-key",
				},
				Key: "key.json",
			},
			wi: map[string]string{
				"cluster-wi-ksa1": "cluster-wi-gsa1@PROJECT.iam.gserviceaccount.com",
				"cluster-wi-ksa2": "cluster-wi-gsa2@PROJECT.iam.gserviceaccount.com",
			},
		},
		{
			ns:  customizedNS,
			ksa: "ns-default-ksa",
			secret: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "some-other-name",
				},
				Key: "some-other-key",
			},
			wi: map[string]string{
				"ns-wi-ksa1": "ns-wi-gsa1@PROJECT.iam.gserviceaccount.com",
				"ns-wi-ksa2": "ns-wi-gsa2@PROJECT.iam.gserviceaccount.com",
			},
		},
		{
			ns:     emptyNS,
			ksa:    "",
			secret: nil,
			wi:     map[string]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.ns, func(t *testing.T) {
			if want, got := tc.ksa, defaults.KSA(tc.ns); want != got {
				t.Errorf("Unexpected value. Expected %q Got %q", want, got)
			}

			if diff := cmp.Diff(tc.secret, defaults.Secret(tc.ns)); diff != "" {
				t.Errorf("Unexpected value (-want +got): %s", diff)
			}

			ksaNames := []string{"cluster-wi-ksa1", "cluster-wi-ksa2", "ns-wi-ksa1", "ns-wi-ksa2", "other-ksa"}
			for _, ksaName := range ksaNames {
				if want, got := tc.wi[ksaName], defaults.WorkloadIdentityGSA(tc.ns, ksaName); want != got {
					t.Errorf("Unexpected value. Expected %q Got %q", want, got)
				}
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
