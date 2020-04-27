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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	brokerv1beta1 "github.com/google/knative-gcp/pkg/apis/broker/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const testUID = "11186600-4003-4ad6-90e7-22780053debf"

var maxName = strings.Repeat("n", k8sNameMax)

// fmt.Sprintf("%x", md5.Sum([]byte(maxName)))
var maxNameHash = "1c6d61b118caf1b3e5c8c4404f34b4a2"

var maxNamespace = strings.Repeat("s", k8sNamespaceMax)

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
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_%s_%s%s_%s", maxNamespace, strings.Repeat("n", 146-md5Len), maxNameHash, testUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_default_%s%s_%s", strings.Repeat("n", 146-md5Len+(k8sNamespaceMax-7)), maxNameHash, testUID),
	}}

	for _, tc := range testCases {
		got := GenerateDecouplingTopicName(broker(tc.ns, tc.n, tc.uid))
		if len(got) > pubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), pubsubMax)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
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
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_%s_%s%s_%s", maxNamespace, strings.Repeat("n", 146-md5Len), maxNameHash, testUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-bkr_default_%s%s_%s", strings.Repeat("n", 146-md5Len+(k8sNamespaceMax-7)), maxNameHash, testUID),
	}}

	for _, tc := range testCases {
		got := GenerateDecouplingSubscriptionName(broker(tc.ns, tc.n, tc.uid))
		if len(got) > pubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), pubsubMax)
		}
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
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_%s_%s%s_%s", maxNamespace, strings.Repeat("n", 146-md5Len), maxNameHash, testUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_default_%s%s_%s", strings.Repeat("n", 146-md5Len+(k8sNamespaceMax-7)), maxNameHash, testUID),
	}}

	for _, tc := range testCases {
		got := GenerateRetryTopicName(trigger(tc.ns, tc.n, tc.uid))
		if len(got) > pubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), pubsubMax)
		}
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
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_%s_%s%s_%s", maxNamespace, strings.Repeat("n", 146-md5Len), maxNameHash, testUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-tgr_default_%s%s_%s", strings.Repeat("n", 146-md5Len+(k8sNamespaceMax-7)), maxNameHash, testUID),
	}}

	for _, tc := range testCases {
		got := GenerateRetrySubscriptionName(trigger(tc.ns, tc.n, tc.uid))
		if len(got) > pubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), pubsubMax)
		}
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

func checkPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	f()
}

func Test_hashedTruncate(t *testing.T) {
	want := "nnn" + maxNameHash
	if got := hashedTruncate(maxName, 35); got != want {
		t.Errorf("Expected %q, got %q", want, got)
	}
}

func Test_hashedTruncatePanic(t *testing.T) {
	// panic when string is longer than max
	checkPanic(t, func() { hashedTruncate("abcde", 4) })

	// panic when string is shorter than max
	checkPanic(t, func() { hashedTruncate("", 31) })

	// no panic when max == 32
	hashedTruncate("", 32)
}
