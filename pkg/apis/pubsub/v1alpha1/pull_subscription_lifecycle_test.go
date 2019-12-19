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

package v1alpha1

import (
	"testing"

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

func TestPubSubStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *PullSubscriptionStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &PullSubscriptionStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkDeployed()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		want: false,
	}, {
		name: "mark subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSubscribed("subID")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed and subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			return s
		}(),
		want: true,
	}, {
		name: "mark sink and deployed and subscribed, then no sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			s.MarkNoSink("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed and subscribed then not deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			s.MarkNotDeployed("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and subscribed and not deployed then deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSubscribed("subID")
			s.MarkNotDeployed("MarkNotDeployed", "")
			s.MarkDeployed()
			return s
		}(),
		want: true,
	}, {
		name: "mark sink empty and deployed and subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink empty and deployed and subscribed then sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			s.MarkSink("uri://example")
			return s
		}(),
		want: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.IsReady()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: unexpected condition (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestPubSubStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *PullSubscriptionStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &PullSubscriptionStatus{},
		condQuery: PullSubscriptionConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkDeployed()
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSubscribed("subID")
			return s
		}(),
		condQuery: PullSubscriptionConditionSubscribed,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionSubscribed,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark not subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkNoSubscription("reason", "%s", "message")
			return s
		}(),
		condQuery: PullSubscriptionConditionSubscribed,
		want: &apis.Condition{
			Type:    PullSubscriptionConditionSubscribed,
			Status:  corev1.ConditionFalse,
			Reason:  "reason",
			Message: "message",
		},
	}, {
		name: "mark transformer",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkTransformer("url")
			return s
		}(),
		condQuery: PullSubscriptionConditionTransformerProvided,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionTransformerProvided,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark transformer unknown",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkTransformer("")
			return s
		}(),
		condQuery: PullSubscriptionConditionTransformerProvided,
		want: &apis.Condition{
			Type:    PullSubscriptionConditionTransformerProvided,
			Status:  corev1.ConditionUnknown,
			Reason:  "TransformerEmpty",
			Message: "Transformer has resolved to empty.",
		},
	}, {
		name: "mark no transformer",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkNoTransformer("reason", "%s", "message")
			return s
		}(),
		condQuery: PullSubscriptionConditionTransformerProvided,
		want: &apis.Condition{
			Type:    PullSubscriptionConditionTransformerProvided,
			Status:  corev1.ConditionFalse,
			Reason:  "reason",
			Message: "message",
		},
	}, {
		name: "mark sink and deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed and subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and deployed and subscribed then no sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			s.MarkNoSink("Testing", "hi")
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:    PullSubscriptionConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and subscribed then not deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			s.MarkNotDeployed("Testing", "hi")
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:    PullSubscriptionConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink empty and deployed and subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:    PullSubscriptionConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty",
		},
	}, {
		name: "mark sink empty and deployed and subscribed then sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed("subID")
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:   PullSubscriptionConditionReady,
			Status: corev1.ConditionTrue,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetCondition(test.condQuery)
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}
