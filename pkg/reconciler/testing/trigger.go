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

package testing

import (
	"context"
	"time"

	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TriggerOption enables further configuration of a Trigger.
type TriggerOption func(*brokerv1beta1.Trigger)

// NewTrigger creates a Trigger with TriggerOptions.
func NewTrigger(name, namespace, broker string, to ...TriggerOption) *brokerv1beta1.Trigger {
	t := &brokerv1beta1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			// Webhook normally adds these
			Labels: map[string]string{
				eventing.BrokerLabelKey: broker,
			},
		},
		Spec: eventingv1beta1.TriggerSpec{
			Broker: broker,
		},
	}
	for _, opt := range to {
		opt(t)
	}
	return t
}

func WithTriggerSubscriberURI(rawurl string) TriggerOption {
	uri, _ := apis.ParseURL(rawurl)
	return func(t *brokerv1beta1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{URI: uri}
	}
}

func WithTriggerSubscriberRef(gvk metav1.GroupVersionKind, name, namespace string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
				Namespace:  namespace,
			},
		}
	}
}

func WithTriggerSubscriberRefAndURIReference(gvk metav1.GroupVersionKind, name, namespace string, rawuri string) TriggerOption {
	uri, _ := apis.ParseURL(rawuri)
	return func(t *brokerv1beta1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{
			Ref: &duckv1.KReference{
				APIVersion: ApiVersion(gvk),
				Kind:       gvk.Kind,
				Name:       name,
				Namespace:  namespace,
			},
			URI: uri,
		}
	}
}

// WithInitTriggerConditions initializes the Triggers's conditions.
func WithInitTriggerConditions(t *brokerv1beta1.Trigger) {
	t.Status.InitializeConditions()
}

func WithTriggerGeneration(gen int64) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Generation = gen
	}
}

func WithTriggerStatusObservedGeneration(gen int64) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Status.ObservedGeneration = gen
	}
}

// WithTriggerBrokerReady initializes the Triggers's conditions as if its
// Broker were ready.
func WithTriggerBrokerReady(t *brokerv1beta1.Trigger) {
	t.Status.PropagateBrokerStatus(brokerv1beta1.TestHelper.ReadyBrokerStatus())
}

// WithTriggerBrokerFailed marks the Broker as failed
func WithTriggerBrokerFailed(reason, message string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Status.MarkBrokerFailed(reason, message)
	}
}

// WithTriggerBrokerUnknown marks the Broker as unknown
func WithTriggerBrokerUnknown(reason, message string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Status.MarkBrokerUnknown(reason, message)
	}
}

func WithTriggerStatusSubscriberURI(uri string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		u, _ := apis.ParseURL(uri)
		t.Status.SubscriberURI = u
	}
}

func WithInjectionAnnotation(injectionAnnotation string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[v1beta1.InjectionAnnotation] = injectionAnnotation
	}
}

func WithDependencyAnnotation(dependencyAnnotation string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[v1beta1.DependencyAnnotation] = dependencyAnnotation
	}
}

func WithTriggerDependencyReady(t *brokerv1beta1.Trigger) {
	t.Status.MarkDependencySucceeded()
}

func WithTriggerDependencyFailed(reason, message string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Status.MarkDependencyFailed(reason, message)
	}
}

func WithTriggerDependencyUnknown(reason, message string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Status.MarkDependencyUnknown(reason, message)
	}
}

func WithTriggerSubscriberResolvedSucceeded(t *brokerv1beta1.Trigger) {
	t.Status.MarkSubscriberResolvedSucceeded()
}

func WithTriggerSubscriberResolvedFailed(reason, message string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Status.MarkSubscriberResolvedFailed(reason, message)
	}
}

func WithTriggerSubscriberResolvedUnknown(reason, message string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Status.MarkSubscriberResolvedUnknown(reason, message)
	}
}

func WithTriggerSubscriptionReady(t *brokerv1beta1.Trigger) {
	t.Status.MarkSubscriptionReady()
}

func WithTriggerTopicReady(t *brokerv1beta1.Trigger) {
	t.Status.MarkTopicReady()
}

func WithTriggerDeletionTimestamp(t *brokerv1beta1.Trigger) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	t.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithTriggerUID(uid string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.UID = types.UID(uid)
	}
}

func WithTriggerFinalizers(finalizers ...string) TriggerOption {
	return func(t *brokerv1beta1.Trigger) {
		t.Finalizers = finalizers
	}
}

func WithTriggerSetDefaults(t *brokerv1beta1.Trigger) {
	t.SetDefaults(context.Background())
}
