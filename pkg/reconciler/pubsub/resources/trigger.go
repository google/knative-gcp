/*
Copyright 2020 Google LLC

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

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

type TriggerArgs struct {
	Namespace  string
	Name       string
	Spec       *duckv1alpha1.PubSubSpec
	Owner      kmeta.OwnerRefable
	Trigger    string
	Labels     map[string]string
	SourceType string
	Filters    map[string]string
}

// MakeTrigger creates the spec for, but does not create, a GCP Trigger
// for a given GCS.
func MakeTrigger(args *TriggerArgs) *pubsubv1alpha1.Trigger {
	return &pubsubv1alpha1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:            args.Name,
			Namespace:       args.Namespace,
			Labels:          args.Labels,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Owner)},
		},
		Spec: pubsubv1alpha1.TriggerSpec{
			IdentitySpec: duckv1alpha1.IdentitySpec{
				GoogleServiceAccount: args.Spec.IdentitySpec.GoogleServiceAccount,
			},
			Secret:     args.Spec.Secret,
			Project:    args.Spec.Project,
			Trigger:    args.Trigger,
			SourceType: args.SourceType,
			Filters:    args.Filters,
		},
	}
}
