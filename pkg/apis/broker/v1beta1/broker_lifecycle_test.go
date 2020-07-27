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
	brokerConditionReady = apis.Condition{
		Type:   eventingv1beta1.BrokerConditionReady,
		Status: corev1.ConditionTrue,
	}

	brokerConditionAddressable = apis.Condition{
		Type:   eventingv1beta1.BrokerConditionAddressable,
		Status: corev1.ConditionTrue,
	}

	brokerConditionBrokerCell = apis.Condition{
		Type:   BrokerConditionBrokerCell,
		Status: corev1.ConditionTrue,
	}

	brokerConditionBrokerCellFalse = apis.Condition{
		Type:   BrokerConditionBrokerCell,
		Status: corev1.ConditionFalse,
	}

	brokerConditionTopic = apis.Condition{
		Type:   BrokerConditionTopic,
		Status: corev1.ConditionTrue,
	}

	brokerConditionSubscription = apis.Condition{
		Type:   BrokerConditionSubscription,
		Status: corev1.ConditionTrue,
	}
)

func TestBrokerGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ts        *BrokerStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ts: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						brokerConditionReady,
					},
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &brokerConditionReady,
	}, {
		name: "multiple conditions",
		ts: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						brokerConditionAddressable,
						brokerConditionBrokerCell,
						brokerConditionTopic,
					},
				},
			},
		},
		condQuery: BrokerConditionBrokerCell,
		want:      &brokerConditionBrokerCell,
	}, {
		name: "multiple conditions, condition false",
		ts: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						brokerConditionAddressable,
						brokerConditionBrokerCellFalse,
						brokerConditionTopic,
					},
				},
			},
		},
		condQuery: BrokerConditionBrokerCell,
		want:      &brokerConditionBrokerCellFalse,
	}, {
		name: "unknown condition",
		ts: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						brokerConditionAddressable,
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

func TestBrokerInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ts   *BrokerStatus
		want *BrokerStatus
	}{{
		name: "empty",
		ts:   &BrokerStatus{},
		want: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.BrokerConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   BrokerConditionBrokerCell,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.BrokerConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   BrokerConditionSubscription,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   BrokerConditionTopic,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one false",
		ts: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.BrokerConditionAddressable,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
		want: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.BrokerConditionAddressable,
						Status: corev1.ConditionFalse,
					}, {
						Type:   BrokerConditionBrokerCell,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.BrokerConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   BrokerConditionSubscription,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   BrokerConditionTopic,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one true",
		ts: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.BrokerConditionAddressable,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		want: &BrokerStatus{
			BrokerStatus: eventingv1beta1.BrokerStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   eventingv1beta1.BrokerConditionAddressable,
						Status: corev1.ConditionTrue,
					}, {
						Type:   BrokerConditionBrokerCell,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   eventingv1beta1.BrokerConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   BrokerConditionSubscription,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   BrokerConditionTopic,
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

func TestBrokerConditionStatus(t *testing.T) {
	tests := []struct {
		name                string
		addressStatus       bool
		brokerCellStatus    corev1.ConditionStatus
		subscriptionStatus  corev1.ConditionStatus
		topicStatus         corev1.ConditionStatus
		configStatus        corev1.ConditionStatus
		wantConditionStatus corev1.ConditionStatus
	}{{
		name:                "all happy",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionTrue,
		subscriptionStatus:  corev1.ConditionTrue,
		topicStatus:         corev1.ConditionTrue,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionTrue,
	}, {
		name:                "subscription sad",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionTrue,
		subscriptionStatus:  corev1.ConditionFalse,
		topicStatus:         corev1.ConditionTrue,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "subscription unknown",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionTrue,
		subscriptionStatus:  corev1.ConditionUnknown,
		topicStatus:         corev1.ConditionTrue,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionUnknown,
	}, {
		name:                "topic sad",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionTrue,
		subscriptionStatus:  corev1.ConditionTrue,
		topicStatus:         corev1.ConditionFalse,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "topic unknown",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionTrue,
		subscriptionStatus:  corev1.ConditionTrue,
		topicStatus:         corev1.ConditionUnknown,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionUnknown,
	}, {
		name:                "address missing",
		addressStatus:       false,
		brokerCellStatus:    corev1.ConditionTrue,
		subscriptionStatus:  corev1.ConditionTrue,
		topicStatus:         corev1.ConditionTrue,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "ingress false",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionFalse,
		subscriptionStatus:  corev1.ConditionTrue,
		topicStatus:         corev1.ConditionTrue,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "brokerCell false",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionFalse,
		subscriptionStatus:  corev1.ConditionTrue,
		topicStatus:         corev1.ConditionTrue,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "brokerCell unknown",
		addressStatus:       true,
		brokerCellStatus:    corev1.ConditionUnknown,
		subscriptionStatus:  corev1.ConditionTrue,
		topicStatus:         corev1.ConditionTrue,
		configStatus:        corev1.ConditionTrue,
		wantConditionStatus: corev1.ConditionUnknown,
	}, {
		name:                "all sad",
		addressStatus:       false,
		brokerCellStatus:    corev1.ConditionFalse,
		subscriptionStatus:  corev1.ConditionFalse,
		topicStatus:         corev1.ConditionFalse,
		configStatus:        corev1.ConditionFalse,
		wantConditionStatus: corev1.ConditionFalse,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bs := &BrokerStatus{}
			if test.addressStatus {
				bs.SetAddress(apis.HTTP("example.com"))
			} else {
				bs.SetAddress(nil)
			}
			if test.brokerCellStatus == corev1.ConditionTrue {
				bs.MarkBrokerCellReady()
			} else if test.brokerCellStatus == corev1.ConditionFalse {
				bs.MarkBrokerCellFailed("Unable to create brokercell", "induced failure")
			} else {
				bs.MarkBrokerCellUnknown("Unable to create brokercell", "induced unknown")
			}
			if test.subscriptionStatus == corev1.ConditionTrue {
				bs.MarkSubscriptionReady()
			} else if test.subscriptionStatus == corev1.ConditionFalse {
				bs.MarkSubscriptionFailed("Unable to create PubSub subscription", "induced failure")
			} else {
				bs.MarkSubscriptionUnknown("Unable to create PubSub subscription", "induced unknown")
			}
			if test.topicStatus == corev1.ConditionTrue {
				bs.MarkTopicReady()
			} else if test.topicStatus == corev1.ConditionFalse {
				bs.MarkTopicFailed("Unable to create PubSub topic", "induced failure")
			} else {
				bs.MarkTopicUnknown("Unable to create PubSub topic", "induced unknown")
			}

			got := bs.GetTopLevelCondition().Status
			if test.wantConditionStatus != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantConditionStatus, got)
			}
			happy := bs.IsReady()
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
