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

	"github.com/google/knative-gcp/pkg/apis/configs/gcpauth"
	"github.com/google/knative-gcp/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

func (s *PubSubSpec) SetPubSubDefaults(ctx context.Context) {
	ad := gcpauth.FromContextOrDefaults(ctx).GCPAuthDefaults
	if ad == nil {
		// TODO This should probably error out, rather than silently allow in non-defaulted COs.
		logging.FromContext(ctx).Error("Failed to get the GCPAuthDefaults")
		return
	}
	if equality.Semantic.DeepEqual(s.Secret, &corev1.SecretKeySelector{}) {
		s.Secret = nil
	}
	if s.ServiceAccountName == "" && s.Secret == nil {
		s.ServiceAccountName = ad.KSA(apis.ParentMeta(ctx).Namespace)
		s.Secret = ad.Secret(apis.ParentMeta(ctx).Namespace)
	}
}
