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
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	duckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

type PullSubscriptionArgs struct {
	Namespace     string
	Name          string
	Spec          *duckv1alpha1.PubSubSpec
	Owner         kmeta.OwnerRefable
	Topic         string
	AdapterType   string
	ResourceGroup string
	Mode          pubsubv1alpha1.ModeType
	Labels        map[string]string
}

// MakePullSubscription creates the spec for, but does not create, a GCP PullSubscription
// for a given GCS.
func MakePullSubscription(args *PullSubscriptionArgs) *pubsubv1alpha1.PullSubscription {
	annotations := map[string]string{
		"metrics-resource-group": args.ResourceGroup,
	}

	pubsubSecret := args.Spec.Secret
	if args.Spec.PubSubSecret != nil {
		pubsubSecret = args.Spec.PubSubSecret
	}

	ps := &pubsubv1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:            args.Name,
			Namespace:       args.Namespace,
			Labels:          args.Labels,
			Annotations:     annotations,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Owner)},
		},
		Spec: pubsubv1alpha1.PullSubscriptionSpec{
			Secret:      pubsubSecret,
			Project:     args.Spec.Project,
			Topic:       args.Topic,
			AdapterType: args.AdapterType,
			Mode:        args.Mode,
			SourceSpec: duckv1.SourceSpec{
				Sink: args.Spec.SourceSpec.Sink,
			},
		},
	}
	if args.Spec.CloudEventOverrides != nil && args.Spec.CloudEventOverrides.Extensions != nil {
		ps.Spec.SourceSpec.CloudEventOverrides = &duckv1.CloudEventOverrides{
			Extensions: args.Spec.CloudEventOverrides.Extensions,
		}
	}
	return ps
}
