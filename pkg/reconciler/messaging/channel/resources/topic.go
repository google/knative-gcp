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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/kmeta"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

// TopicArgs are the arguments needed to create a Channel Topic.
// Every field is required.
type TopicArgs struct {
	Owner          kmeta.OwnerRefable
	Name           string
	Project        string
	Topic          string
	ServiceAccount string
	Secret         *corev1.SecretKeySelector
	Labels         map[string]string
}

// MakeInvoker generates (but does not insert into K8s) the Topic for Channels.
func MakeTopic(args *TopicArgs) *v1alpha1.Topic {
	return &v1alpha1.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Owner.GetObjectMeta().GetNamespace(),
			Name:            args.Name,
			Labels:          args.Labels,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Owner)},
		},
		Spec: v1alpha1.TopicSpec{
			IdentitySpec: duckv1alpha1.IdentitySpec{
				GoogleServiceAccount: args.ServiceAccount,
			},
			Secret:            args.Secret,
			Project:           args.Project,
			Topic:             args.Topic,
			PropagationPolicy: v1alpha1.TopicPolicyCreateDelete,
		},
	}
}
