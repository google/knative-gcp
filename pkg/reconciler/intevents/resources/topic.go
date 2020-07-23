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

	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	inteventsv1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
)

type TopicArgs struct {
	Namespace       string
	Name            string
	Spec            *duckv1.PubSubSpec
	EnablePublisher *bool
	Owner           kmeta.OwnerRefable
	Topic           string
	Labels          map[string]string
	Annotations     map[string]string
}

// MakeTopic creates the spec for, but does not create, a GCP Topic
// for a given GCS.
func MakeTopic(args *TopicArgs) *inteventsv1.Topic {
	return &inteventsv1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name:            args.Name,
			Namespace:       args.Namespace,
			Labels:          args.Labels,
			Annotations:     args.Annotations,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Owner)},
		},
		Spec: inteventsv1.TopicSpec{
			IdentitySpec: duckv1.IdentitySpec{
				ServiceAccountName: args.Spec.IdentitySpec.ServiceAccountName,
			},
			Secret:            args.Spec.Secret,
			Project:           args.Spec.Project,
			Topic:             args.Topic,
			PropagationPolicy: inteventsv1.TopicPolicyCreateDelete,
			EnablePublisher:   args.EnablePublisher,
		},
	}
}
