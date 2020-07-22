/*
Copyright 2019 The Knative Authors

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
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	gcpduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var condReady = apis.Condition{
	Type:   ChannelConditionReady,
	Status: corev1.ConditionTrue,
}

var ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
	apis.Condition{},
	"LastTransitionTime", "Message", "Reason", "Severity")

func TestChannelGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *ChannelStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cs: &ChannelStatus{
			IdentityStatus: gcpduckv1.IdentityStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						condReady,
					},
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &condReady,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		cs   *ChannelStatus
		want *ChannelStatus
	}{{
		name: "empty",
		cs:   &ChannelStatus{},
		want: &ChannelStatus{
			IdentityStatus: gcpduckv1.IdentityStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionTopicReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one false",
		cs: &ChannelStatus{
			IdentityStatus: gcpduckv1.IdentityStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionFalse,
					}},
				},
			},
		},
		want: &ChannelStatus{
			IdentityStatus: gcpduckv1.IdentityStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionFalse,
					}, {
						Type:   ChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionTopicReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}, {
		name: "one true",
		cs: &ChannelStatus{
			IdentityStatus: gcpduckv1.IdentityStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
		want: &ChannelStatus{
			IdentityStatus: gcpduckv1.IdentityStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   ChannelConditionAddressable,
						Status: corev1.ConditionTrue,
					}, {
						Type:   ChannelConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   ChannelConditionTopicReady,
						Status: corev1.ConditionUnknown,
					}},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sort.Slice(test.want.Conditions, func(i, j int) bool { return test.want.Conditions[i].Type < test.want.Conditions[j].Type })
			test.cs.InitializeConditions()
			if diff := cmp.Diff(test.want, test.cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestChannelIsReady(t *testing.T) {
	tests := []struct {
		name                string
		setAddress          bool
		topicStatus         *v1beta1.TopicStatus
		wantConditionStatus corev1.ConditionStatus
		want                bool
	}{{
		name:                "all happy",
		setAddress:          true,
		topicStatus:         ReadyTopicStatus(),
		wantConditionStatus: corev1.ConditionTrue,
		want:                true,
	}, {
		name:                "address not set",
		setAddress:          false,
		topicStatus:         ReadyTopicStatus(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}, {
		name:                "the status of topic is false",
		setAddress:          true,
		topicStatus:         FalseTopicStatus(),
		wantConditionStatus: corev1.ConditionFalse,
		want:                false,
	}, {
		name:                "the status of topic is unknown",
		setAddress:          true,
		topicStatus:         UnknownTopicStatus(),
		wantConditionStatus: corev1.ConditionUnknown,
		want:                false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.InitializeConditions()
			if test.setAddress {
				cs.SetAddress(&apis.URL{Scheme: "http", Host: "foo.bar"})
			}
			cs.PropagateTopicStatus(test.topicStatus)
			gotConditionStatus := cs.GetTopLevelCondition().Status
			if test.wantConditionStatus != gotConditionStatus {
				t.Errorf("unexpected condition status: want %v, got %v", test.wantConditionStatus, gotConditionStatus)
			}
			got := cs.IsReady()
			if got != test.want {
				t.Errorf("unexpected readiness: want %v, got %v", test.want, got)
			}
		})
	}
}

func TestPubSubChannelStatus_SetAddressable(t *testing.T) {
	testCases := map[string]struct {
		url  *apis.URL
		want *ChannelStatus
	}{
		"empty string": {
			want: &ChannelStatus{
				IdentityStatus: gcpduckv1.IdentityStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   ChannelConditionAddressable,
								Status: corev1.ConditionFalse,
							},
							// Note that Ready is here because when the condition is marked False, duck
							// automatically sets Ready to false.
							{
								Type:   ChannelConditionReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				},
				AddressStatus: duckv1.AddressStatus{Address: &duckv1.Addressable{}},
			},
		},
		"has domain": {
			url: &apis.URL{Scheme: "http", Host: "test-domain"},
			want: &ChannelStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: &duckv1.Addressable{
						URL: &apis.URL{
							Scheme: "http",
							Host:   "test-domain",
						},
					},
				},
				IdentityStatus: gcpduckv1.IdentityStatus{
					Status: duckv1.Status{
						Conditions: []apis.Condition{
							{
								Type:   ChannelConditionAddressable,
								Status: corev1.ConditionTrue,
							},
							{
								Type:   ChannelConditionReady,
								Status: corev1.ConditionUnknown,
							},
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &ChannelStatus{}
			cs.SetAddress(tc.url)
			if diff := cmp.Diff(tc.want, cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func ReadyTopicStatus() *v1beta1.TopicStatus {
	ts := &v1beta1.TopicStatus{}
	ts.InitializeConditions()
	ts.MarkPublisherDeployed()
	ts.MarkTopicReady()
	url, _ := apis.ParseURL("http://testchannel.mynamespace.svc.cluster.local/")
	ts.SetAddress(url)
	ts.TopicID = "test"
	return ts
}

func FalseTopicStatus() *v1beta1.TopicStatus {
	ts := &v1beta1.TopicStatus{}
	ts.InitializeConditions()
	ts.MarkNoTopic("TopicNotFound", "topic not found")
	return ts
}

func UnknownTopicStatus() *v1beta1.TopicStatus {
	ts := &v1beta1.TopicStatus{}
	ts.InitializeConditions()
	return ts
}
