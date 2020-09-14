/*
Copyright 2019 The Knative Authors

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
	"github.com/google/knative-gcp/pkg/apis/duck"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"

	"github.com/google/knative-gcp/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/pkg/apis"
)

var (
	trueVal = true
)

func (t *Topic) SetDefaults(ctx context.Context) {
	ctx = apis.WithinParent(ctx, t.ObjectMeta)
	t.Spec.SetDefaults(ctx)
	duck.SetClusterNameAnnotation(&t.ObjectMeta, metadataClient.NewDefaultMetadataClient())
}

func (ts *TopicSpec) SetDefaults(ctx context.Context) {
	if ts.PropagationPolicy == "" {
		ts.PropagationPolicy = TopicPolicyCreateNoDelete
	}

	ad := gcpauth.FromContextOrDefaults(ctx).GCPAuthDefaults
	if ad == nil {
		// TODO This should probably error out, rather than silently allow in non-defaulted COs.
		logging.FromContext(ctx).Error("Failed to get the GCPAuthDefaults")
		return
	}
	if ts.ServiceAccountName == "" &&
		(ts.Secret == nil || equality.Semantic.DeepEqual(ts.Secret, &corev1.SecretKeySelector{})) {
		ts.ServiceAccountName = ad.KSA(apis.ParentMeta(ctx).Namespace)
		ts.Secret = ad.Secret(apis.ParentMeta(ctx).Namespace)
	}

	if ts.EnablePublisher == nil {
		ts.EnablePublisher = &trueVal
	}
}
