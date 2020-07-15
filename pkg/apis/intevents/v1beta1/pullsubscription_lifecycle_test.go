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

package v1beta1

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

var (
	availableDeployment = &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	unavailableDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:    appsv1.DeploymentAvailable,
					Status:  corev1.ConditionFalse,
					Reason:  "The status of Deployment is False",
					Message: "False Status",
				},
			},
		},
	}

	unknownDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionUnknown,
				},
			},
		},
	}
)

func TestPullSubscriptionStatusIsReady(t *testing.T) {
	tests := []struct {
		name                string
		s                   *PullSubscriptionStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
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
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSubscribed("subID")
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink and deployed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name: "mark sink and deployed and subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkSubscribed("subID")
			return s
		}(),
		wantConditionStatus: corev1.ConditionTrue,
		want:                true,
	},
		{
			name: "mark sink and unavailable deployment and subscribed",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(apis.HTTP("example"))
				s.PropagateDeploymentAvailability(unavailableDeployment)
				s.MarkSubscribed("subID")
				return s
			}(),
			wantConditionStatus: corev1.ConditionFalse,
			want:                false,
		}, {
			name: "mark sink and unknown deployment and subscribed",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(apis.HTTP("example"))
				s.PropagateDeploymentAvailability(unknownDeployment)
				s.MarkSubscribed("subID")
				return s
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
			want:                false,
		}, {
			name: "mark sink and not deployed and subscribed",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(apis.HTTP("example"))
				s.PropagateDeploymentAvailability(&appsv1.Deployment{})
				s.MarkSubscribed("subID")
				return s
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
			want:                false,
		}, {
			name: "mark sink and deployed and subscribed, then no sink",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(apis.HTTP("example"))
				s.PropagateDeploymentAvailability(availableDeployment)
				s.MarkSubscribed("subID")
				s.MarkNoSink("Testing", "")
				return s
			}(),
			wantConditionStatus: corev1.ConditionFalse,
			want:                false,
		}, {
			name: "mark sink and deployed and subscribed then not deployed",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(apis.HTTP("example"))
				s.PropagateDeploymentAvailability(availableDeployment)
				s.MarkSubscribed("subID")
				s.PropagateDeploymentAvailability(unavailableDeployment)
				return s
			}(),
			wantConditionStatus: corev1.ConditionFalse,
			want:                false,
		}, {
			name: "mark sink and subscribed and not deployed then deployed",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(apis.HTTP("example"))
				s.MarkSubscribed("subID")
				s.PropagateDeploymentAvailability(unavailableDeployment)
				s.PropagateDeploymentAvailability(availableDeployment)
				return s
			}(),
			wantConditionStatus: corev1.ConditionTrue,
			want:                true,
		}, {
			name: "mark sink nil and deployed and subscribed",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(nil)
				s.PropagateDeploymentAvailability(availableDeployment)
				s.MarkSubscribed("subID")
				return s
			}(),
			wantConditionStatus: corev1.ConditionUnknown,
			want:                false,
		}, {
			name: "mark sink nil and deployed and subscribed then sink",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(nil)
				s.PropagateDeploymentAvailability(availableDeployment)
				s.MarkSubscribed("subID")
				s.MarkSink(apis.HTTP("example"))
				return s
			}(),
			wantConditionStatus: corev1.ConditionTrue,
			want:                true,
		}, {
			name: "mark sink empty and deployed and subscribed then sink",
			s: func() *PullSubscriptionStatus {
				s := &PullSubscriptionStatus{}
				s.InitializeConditions()
				s.MarkSink(&apis.URL{})
				s.PropagateDeploymentAvailability(availableDeployment)
				s.MarkSubscribed("subID")
				s.MarkSink(apis.HTTP("example"))
				return s
			}(),
			wantConditionStatus: corev1.ConditionTrue,
			want:                true,
		}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.wantConditionStatus != "" {
				gotConditionStatus := test.s.GetTopLevelCondition().Status
				if gotConditionStatus != test.wantConditionStatus {
					t.Errorf("unexpected condition status: want %v, got %v", test.wantConditionStatus, gotConditionStatus)
				}
			}
			got := test.s.IsReady()
			if got != test.want {
				t.Errorf("unexpected readiness: want %v, got %v", test.want, got)
			}
		})
	}
}

func TestPullSubscriptionStatusGetCondition(t *testing.T) {
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
			s.PropagateDeploymentAvailability(availableDeployment)
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
			s.MarkSink(apis.HTTP("example"))
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
			s.MarkTransformer(apis.HTTP("url"))
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
			s.MarkTransformer(nil)
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
			s.MarkSink(apis.HTTP("example"))
			s.PropagateDeploymentAvailability(availableDeployment)
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
			s.MarkSink(apis.HTTP("example"))
			s.PropagateDeploymentAvailability(availableDeployment)
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
			s.MarkSink(apis.HTTP("example"))
			s.PropagateDeploymentAvailability(availableDeployment)
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
			s.MarkSink(apis.HTTP("example"))
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkSubscribed("subID")
			s.PropagateDeploymentAvailability(unavailableDeployment)
			return s
		}(),
		condQuery: PullSubscriptionConditionReady,
		want: &apis.Condition{
			Type:    PullSubscriptionConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "The status of Deployment is False",
			Message: "False Status",
		},
	}, {
		name: "mark sink nil and deployed and subscribed",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.PropagateDeploymentAvailability(availableDeployment)
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
		name: "mark sink nil and deployed and subscribed then sink",
		s: func() *PullSubscriptionStatus {
			s := &PullSubscriptionStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkSubscribed("subID")
			s.MarkSink(apis.HTTP("example"))
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
