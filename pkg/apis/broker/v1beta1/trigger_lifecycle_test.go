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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	triggerConditionReady = apis.Condition{
		Type:   eventingv1beta1.TriggerConditionReady,
		Status: corev1.ConditionTrue,
	}

	triggerConditionBroker = apis.Condition{
		Type:   eventingv1beta1.TriggerConditionBroker,
		Status: corev1.ConditionTrue,
	}

	triggerConditionDependency = apis.Condition{
		Type:   eventingv1beta1.TriggerConditionDependency,
		Status: corev1.ConditionTrue,
	}

	triggerConditionSubscriberResolved = apis.Condition{
		Type:   eventingv1beta1.TriggerConditionSubscriberResolved,
		Status: corev1.ConditionTrue,
	}

	triggerConditionTopic = apis.Condition{
		Type:   TriggerConditionTopic,
		Status: corev1.ConditionTrue,
	}

	triggerConditionSubscription = apis.Condition{
		Type:   TriggerConditionSubscription,
		Status: corev1.ConditionTrue,
	}
)

func TestTriggerGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ts        *TriggerStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ts: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						triggerConditionReady,
					},
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &triggerConditionReady,
	}, {
		name: "multiple conditions",
		ts: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						triggerConditionBroker,
						triggerConditionDependency,
						triggerConditionSubscriberResolved,
					},
				},
			},
		},
		condQuery: eventingv1beta1.TriggerConditionDependency,
		want:      &triggerConditionDependency,
	}, {
		name: "multiple conditions, condition false",
		ts: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						triggerConditionBroker,
						triggerConditionDependency,
						triggerConditionSubscriberResolved,
					},
				},
			},
		},
		condQuery: eventingv1beta1.TriggerConditionDependency,
		want:      &triggerConditionDependency,
	}, {
		name: "unknown condition",
		ts: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						triggerConditionBroker,
					},
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ts.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestTriggerInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ts   *TriggerStatus
		want *TriggerStatus
	}{{
		name: "empty",
		ts:   &TriggerStatus{},
		want: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.TriggerConditionBroker,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.TriggerConditionDependency,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.TriggerConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.TriggerConditionSubscriberResolved,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   TriggerConditionSubscription,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   TriggerConditionTopic,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one false",
		ts: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.TriggerConditionBroker,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
		want: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.TriggerConditionBroker,
						Status: corev1.ConditionFalse,
					}, {
						Type:   eventingv1beta1.TriggerConditionDependency,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.TriggerConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.TriggerConditionSubscriberResolved,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   TriggerConditionSubscription,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   TriggerConditionTopic,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one true",
		ts: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.TriggerConditionBroker,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		want: &TriggerStatus{
			TriggerStatus: eventingv1beta1.TriggerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.TriggerConditionBroker,
						Status: corev1.ConditionTrue,
					}, {
						Type:   eventingv1beta1.TriggerConditionDependency,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.TriggerConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.TriggerConditionSubscriberResolved,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   TriggerConditionSubscription,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   TriggerConditionTopic,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}}

	ignoreAllButTypeAndStatus := cmpopts.IgnoreFields(
		apis.Condition{},
		"LastTransitionTime", "Message", "Reason", "Severity")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ts.InitializeConditions()
			if diff := cmp.Diff(test.want, test.ts, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestTriggerConditionStatus(t *testing.T) {
	tests := []struct {
		name                     string
		brokerStatus             *BrokerStatus
		topicStatus              corev1.ConditionStatus
		subscriptionStatus       corev1.ConditionStatus
		subscriberResolvedStatus corev1.ConditionStatus
		dependencyStatus         *duckv1.Source
		wantConditionStatus      corev1.ConditionStatus
	}{{
		name:                     "all happy",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionTrue,
	}, {
		name:                     "broker status unconfigured",
		brokerStatus:             TestHelper.UnconfiguredBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionUnknown,
	}, {
		name:                     "broker status unconfigured",
		brokerStatus:             TestHelper.UnknownBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionUnknown,
	}, {
		name:                     "broker status false",
		brokerStatus:             TestHelper.FalseBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionFalse,
	}, {
		name:                     "subscription sad",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionFalse,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionFalse,
	}, {
		name:                     "subscription unknown",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionUnknown,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionUnknown,
	}, {
		name:                     "topic sad",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionFalse,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionFalse,
	}, {
		name:                     "topic unknown",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionUnknown,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         nil,
		wantConditionStatus:      corev1.ConditionUnknown,
	}, {
		name:                     "failed to resolve subscriber",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionFalse,
		dependencyStatus:         TestHelper.ReadyDependencyStatus(),
		wantConditionStatus:      corev1.ConditionFalse,
	}, {
		name:                     "resolve subscriber unknown",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionUnknown,
		dependencyStatus:         TestHelper.ReadyDependencyStatus(),
		wantConditionStatus:      corev1.ConditionUnknown,
	}, {
		name:                     "dependency unconfigured",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         TestHelper.UnconfiguredDependencyStatus(),
		wantConditionStatus:      corev1.ConditionUnknown,
	}, {
		name:                     "dependency unknown",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         TestHelper.UnknownDependencyStatus(),
		wantConditionStatus:      corev1.ConditionUnknown,
	}, {
		name:                     "dependency false",
		brokerStatus:             TestHelper.ReadyBrokerStatus(),
		subscriptionStatus:       corev1.ConditionTrue,
		topicStatus:              corev1.ConditionTrue,
		subscriberResolvedStatus: corev1.ConditionTrue,
		dependencyStatus:         TestHelper.FalseDependencyStatus(),
		wantConditionStatus:      corev1.ConditionFalse,
	}, {
		name:                     "all sad",
		brokerStatus:             TestHelper.FalseBrokerStatus(),
		subscriptionStatus:       corev1.ConditionFalse,
		topicStatus:              corev1.ConditionFalse,
		subscriberResolvedStatus: corev1.ConditionFalse,
		dependencyStatus:         TestHelper.FalseDependencyStatus(),
		wantConditionStatus:      corev1.ConditionFalse,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := &TriggerStatus{}
			if test.brokerStatus != nil {
				ts.PropagateBrokerStatus(test.brokerStatus)
			}
			if test.subscriptionStatus == corev1.ConditionTrue {
				ts.MarkSubscriptionReady()
			} else if test.subscriptionStatus == corev1.ConditionFalse {
				ts.MarkSubscriptionFailed("Unable to create PubSub subscription", "induced failure")
			} else {
				ts.MarkSubscriptionUnknown("Unable to create PubSub subscription", "induced unknown")
			}
			if test.topicStatus == corev1.ConditionTrue {
				ts.MarkTopicReady()
			} else if test.topicStatus == corev1.ConditionFalse {
				ts.MarkTopicFailed("Unable to create PubSub topic", "induced failure")
			} else {
				ts.MarkTopicUnknown("Unable to create PubSub topic", "induced unknown")
			}
			if test.subscriberResolvedStatus == corev1.ConditionTrue {
				ts.MarkSubscriberResolvedSucceeded()
			} else if test.subscriberResolvedStatus == corev1.ConditionFalse {
				ts.MarkSubscriberResolvedFailed("Unable to get the Subscriber's URI", "subscriber not found")
			} else {
				ts.MarkSubscriberResolvedUnknown("Status of Subscriber URI is unknown", "induced failure")
			}
			if test.dependencyStatus == nil {
				ts.MarkDependencySucceeded()
			} else {
				ts.PropagateDependencyStatus(test.dependencyStatus)
			}
			got := ts.GetTopLevelCondition().Status
			if test.wantConditionStatus != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantConditionStatus, got)
			}
			happy := ts.IsReady()
			switch test.wantConditionStatus {
			case corev1.ConditionTrue:
				if !happy {
					t.Error("expected happy true, got false")
				}
			case corev1.ConditionFalse, corev1.ConditionUnknown:
				if happy {
					t.Error("expected happy false, got true")
				}
			}
		})
	}
}
