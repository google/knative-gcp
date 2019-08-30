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

package v1alpha1

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestDecoratorGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *DecoratorStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cs: &DecoratorStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					condReady,
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

func TestDecoratorInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		cs   *DecoratorStatus
		want *DecoratorStatus
	}{{
		name: "empty",
		cs:   &DecoratorStatus{},
		want: &DecoratorStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   DecoratorConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   DecoratorConditionAddressable,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   DecoratorConditionSinkProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   DecoratorConditionServiceReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		cs: &DecoratorStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   DecoratorConditionAddressable,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &DecoratorStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   DecoratorConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   DecoratorConditionAddressable,
					Status: corev1.ConditionFalse,
				}, {
					Type:   DecoratorConditionSinkProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   DecoratorConditionServiceReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		cs: &DecoratorStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   DecoratorConditionAddressable,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &DecoratorStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   DecoratorConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   DecoratorConditionAddressable,
					Status: corev1.ConditionTrue,
				}, {
					Type:   DecoratorConditionSinkProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   DecoratorConditionServiceReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}}

	var ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
		apis.Condition{},
		"LastTransitionTime", "Message", "Reason", "Severity")

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

func TestDecoratorIsReady(t *testing.T) {
	tests := []struct {
		name        string
		setAddress  bool
		markService bool
		markSink    bool
		wantReady   bool
	}{{
		name:        "all happy",
		setAddress:  true,
		markService: true,
		markSink:    true,
		wantReady:   true,
	}, {
		name:        "address not set",
		setAddress:  false,
		markService: true,
		markSink:    true,
		wantReady:   false,
	}, {
		name:        "service not ready",
		setAddress:  true,
		markService: false,
		markSink:    true,
		wantReady:   false,
	}, {
		name:        "no sink",
		setAddress:  true,
		markService: false,
		markSink:    true,
		wantReady:   false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := &DecoratorStatus{}
			cs.InitializeConditions()
			if test.setAddress {
				cs.SetAddress(&apis.URL{Scheme: "http", Host: "foo.bar"})
			}
			if test.markService {
				cs.MarkServiceReady()
			} else {
				cs.MarkNoService("NoService", "UnitTest")
			}
			if test.markSink {
				cs.MarkSink(&apis.URL{Scheme: "http", Host: "foo.bar"})
			} else {
				cs.MarkNoSink("NoSink", "UnitTest")
			}
			got := cs.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestPubSubDecoratorStatus_SetAddressable(t *testing.T) {
	testCases := map[string]struct {
		url  *apis.URL
		want *DecoratorStatus
	}{
		"empty string": {
			want: &DecoratorStatus{
				Status: duckv1.Status{
					Conditions: []apis.Condition{
						{
							Type:   DecoratorConditionAddressable,
							Status: corev1.ConditionFalse,
						},
						// Note that Ready is here because when the condition is marked False, duck
						// automatically sets Ready to false.
						{
							Type:   DecoratorConditionReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
				AddressStatus: duckv1.AddressStatus{Address: &duckv1.Addressable{}},
			},
		},
		"has domain": {
			url: &apis.URL{Scheme: "http", Host: "test-domain"},
			want: &DecoratorStatus{
				AddressStatus: duckv1.AddressStatus{
					Address: &duckv1.Addressable{
						URL: &apis.URL{
							Scheme: "http",
							Host:   "test-domain",
						},
					},
				},
				Status: duckv1.Status{
					Conditions: []apis.Condition{{
						Type:   DecoratorConditionAddressable,
						Status: corev1.ConditionTrue,
					}},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			cs := &DecoratorStatus{}
			cs.SetAddress(tc.url)
			if diff := cmp.Diff(tc.want, cs, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}
