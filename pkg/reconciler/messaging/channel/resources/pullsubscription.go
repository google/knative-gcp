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
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	gcpduckv1alpha1 "github.com/google/knative-gcp/pkg/apis/duck/v1alpha1"
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

// PullSubscriptionArgs are the arguments needed to create a Channel Subscriber.
// Every field is required.
type PullSubscriptionArgs struct {
	Owner          kmeta.OwnerRefable
	Name           string
	Project        string
	Topic          string
	ServiceAccount string
	Secret         *corev1.SecretKeySelector
	Labels         map[string]string
	Annotations    map[string]string
	Subscriber     duckv1alpha1.SubscriberSpec
}

// MakePullSubscription generates (but does not insert into K8s) the
// PullSubscription for Channels.
func MakePullSubscription(args *PullSubscriptionArgs) *v1alpha1.PullSubscription {

	spec := v1alpha1.PullSubscriptionSpec{
		PubSubSpec: gcpduckv1alpha1.PubSubSpec{
			SourceSpec: duckv1.SourceSpec{},
			IdentitySpec: gcpduckv1alpha1.IdentitySpec{
				GoogleServiceAccount: args.ServiceAccount,
			},
			Secret:  args.Secret,
			Project: args.Project,
		},
		Topic: args.Topic,
	}

	reply := args.Subscriber.ReplyURI
	subscriber := args.Subscriber.SubscriberURI

	// If subscriber and reply is used, map:
	//   pull.transformer to sub.subscriber
	//   pull.sink to sub.reply
	// Otherwise, pull.sink has to be used, but subscriptions allow for just
	// reply or just subscriber. So set the single non-nil uri to to pull.sink.
	if subscriber != nil && reply != nil {
		spec.Transformer = &duckv1.Destination{
			URI: subscriber,
		}
		spec.Sink = duckv1.Destination{
			URI: reply,
		}
	} else if subscriber != nil {
		spec.Sink = duckv1.Destination{
			URI: subscriber,
		}
	} else if reply != nil {
		spec.Sink = duckv1.Destination{
			URI: reply,
		}
	}

	return &v1alpha1.PullSubscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Owner.GetObjectMeta().GetNamespace(),
			Name:            args.Name,
			Labels:          args.Labels,
			Annotations:     args.Annotations,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Owner)},
		},
		Spec: spec,
	}
}
