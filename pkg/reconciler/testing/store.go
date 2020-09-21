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

package testing

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	logtesting "knative.dev/pkg/logging/testing"

	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
)

func NewGCPAuthTestStore(t *testing.T, config *corev1.ConfigMap) *gcpauth.Store {
	gcpAuthTestStore := gcpauth.NewStore(logtesting.TestLogger(t))
	if config != nil {
		gcpAuthTestStore.OnConfigChanged(config)
	}
	return gcpAuthTestStore
}

func NewDataresidencyTestStore(t *testing.T, config *corev1.ConfigMap) *dataresidency.Store {
	dataresidencyTestStore := dataresidency.NewStore(logtesting.TestLogger(t))
	if config != nil {
		dataresidencyTestStore.OnConfigChanged(config)
	}
	return dataresidencyTestStore
}
