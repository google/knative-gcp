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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

//TODO verify topic and sub name can't be longer than 255 chars

const testUID = "11186600-4003-4ad6-90e7-22780053debf"

func TestGenerateDecouplingTopicName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_default_default_%s", testUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_with-dashes_more-dashes_%s", testUID),
	}}

	for _, tc := range testCases {
		got := GenerateDecouplingTopicName(broker(tc.ns, tc.n, tc.uid))
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (want, +got) = %v", diff)
		}
	}
}

func TestGenerateDecouplingSubscriptionName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_default_default_%s", testUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_with-dashes_more-dashes_%s", testUID),
	}}

	for _, tc := range testCases {
		got := GenerateDecouplingSubscriptionName(broker(tc.ns, tc.n, tc.uid))
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (want, +got) = %v", diff)
		}
	}
}

func TestGenerateRetryTopicName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_default_default_%s", testUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_with-dashes_more-dashes_%s", testUID),
	}}

	for _, tc := range testCases {
		got := GenerateRetryTopicName(trigger(tc.ns, tc.n, tc.uid))
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (want, +got) = %v", diff)
		}
	}
}

func TestGenerateRetrySubscriptionName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_default_default_%s", testUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_with-dashes_more-dashes_%s", testUID),
	}}

	for _, tc := range testCases {
		got := GenerateRetryTopicName(trigger(tc.ns, tc.n, tc.uid))
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (want, +got) = %v", diff)
		}
	}
}

func broker(ns, n, uid string) *brokerv1beta1.Broker {
	return &brokerv1beta1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      n,
			UID:       types.UID(uid),
		},
	}
}

func trigger(ns, n, uid string) *brokerv1beta1.Trigger {
	return &brokerv1beta1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      n,
			UID:       types.UID(uid),
		},
	}
}
