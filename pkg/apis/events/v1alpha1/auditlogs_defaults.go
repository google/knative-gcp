/*
Copyright 2019 Google LLC.

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

package v1alpha1

import (
	"context"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

func (s *AuditLogsSource) SetDefaults(ctx context.Context) {
	s.Spec.SetDefaults(ctx)
}

func (s *AuditLogsSourceSpec) SetDefaults(ctx context.Context) {
	if s.Secret == nil || equality.Semantic.DeepEqual(s.Secret, &corev1.SecretKeySelector{}) {
		s.Secret = duckv1alpha1.DefaultGoogleCloudSecretSelector()
	}
}
