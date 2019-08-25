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

package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

// MakePullSubscription creates the spec for, but does not create, a GCP PullSubscrkiption
// for a given GCS.
func MakePullSubscription(s *v1alpha1.Scheduler, topic string) *pubsubv1alpha1.PullSubscription {
	labels := map[string]string{
		"receive-adapter": "storage.events.cloud.run",
	}

	pubsubSecret := s.Spec.Secret
	if s.Spec.PubSubSecret != nil {
		pubsubSecret = s.Spec.PubSubSecret
	}

	ps := &pubsubv1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(s),
			},
		},
		Spec: pubsubv1alpha1.PullSubscriptionSpec{
			Secret:  pubsubSecret,
			Project: s.Spec.Project,
			Topic:   topic,
			Sink:    s.Spec.Sink,
		},
	}
	if s.Spec.CloudEventOverrides != nil && s.Spec.CloudEventOverrides.Extensions != nil {
		ps.Spec.CloudEventOverrides = &pubsubv1alpha1.CloudEventOverrides{
			Extensions: s.Spec.CloudEventOverrides.Extensions,
		}
	}
	return ps
}
