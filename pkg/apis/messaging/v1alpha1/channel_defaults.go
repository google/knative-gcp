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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"knative.dev/eventing/pkg/apis/messaging"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/messaging/internal"
	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
)

const (
	defaultSecretName = "google-cloud-key"
	defaultSecretKey  = "key.json"
)

// defaultSecretSelector is the default secret selector used to load the creds
// to pass to the Topic and PullSubscriptions to auth with Google Cloud.
func defaultSecretSelector() *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: defaultSecretName,
		},
		Key: defaultSecretKey,
	}
}

func (c *Channel) SetDefaults(ctx context.Context) {
	// We need to set this to the _stored_ version of the Channel. If we set to anything other than
	// the stored version, then when reading the stored version, conversion won't be called so
	// nothing will set it to the stored version.
	// Note that if a user sends a bad version of this annotation (e.g. sets it to v1beta1), then we
	// won't overwrite their bad input. This is because the webhook:
	// 1. Reads the stored version.
	// 2. Converts to the desired version.
	// 3. Defaults the desired version.
	// So we don't know if the user or the converter put the value here, therefore we are forced to
	// assume it was the converter and shouldn't change it.
	if c.Annotations == nil {
		c.Annotations = make(map[string]string, 1)
	}
	if _, present := c.Annotations[messaging.SubscribableDuckVersionAnnotation]; !present {
		c.Annotations[messaging.SubscribableDuckVersionAnnotation] = internal.StoredChannelVersion
	}
	c.Spec.SetDefaults(ctx)
	duckv1alpha1.SetClusterNameAnnotation(&c.ObjectMeta, metadataClient.NewDefaultMetadataClient())
}

func (cs *ChannelSpec) SetDefaults(ctx context.Context) {
	if cs.GoogleServiceAccount == "" && (cs.Secret == nil || equality.Semantic.DeepEqual(cs.Secret, &corev1.SecretKeySelector{})) {
		cs.Secret = defaultSecretSelector()
	}
}
