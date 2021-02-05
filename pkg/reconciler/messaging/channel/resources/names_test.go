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
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/messaging/v1beta1"
	"github.com/google/knative-gcp/pkg/utils/naming"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	testUID = "11186600-4003-4ad6-90e7-22780053debf"

	// truncatedNameMaxForSub is the length that the name will be truncated for Subscription based
	// names, because they use the key 'sub'.
	truncatedNameMaxForSub = 146

	// truncatedNameMaxForCh is the length that the name will be truncated for Channel based names.
	// It is distinct from truncatedNameMaxForSub because the key used for Channels is 'ch',
	// which is one shorter than 'sub'.
	truncatedNameMaxForCh = truncatedNameMaxForSub + 1

	subscriptionUID = "a50142ff-4dc7-4254-9f15-fc6bc0597b4a"
)

var maxName = strings.Repeat("n", naming.K8sNameMax)

var maxNamespace = strings.Repeat("s", naming.K8sNamespaceMax)

func TestGenerateChannelDecouplingTopicName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_default_default_%s", testUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_with-dashes_more-dashes_%s", testUID),
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_%s_%s_%s", maxNamespace, strings.Repeat("n", truncatedNameMaxForCh), testUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_default_%s_%s", strings.Repeat("n", truncatedNameMaxForCh+(naming.K8sNamespaceMax-7)), testUID),
	}}

	for _, tc := range testCases {
		got := GenerateDecouplingTopicName(channel(tc.ns, tc.n, tc.uid))
		if len(got) > naming.PubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), naming.PubsubMax)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
		}
	}
}

func TestGenerateChannelDecouplingSubscriptionName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_default_default_%s", testUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_with-dashes_more-dashes_%s", testUID),
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_%s_%s_%s", maxNamespace, strings.Repeat("n", truncatedNameMaxForCh), testUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-ch_default_%s_%s", strings.Repeat("n", truncatedNameMaxForCh+(naming.K8sNamespaceMax-7)), testUID),
	}}

	for _, tc := range testCases {
		got := GenerateDecouplingSubscriptionName(channel(tc.ns, tc.n, tc.uid))
		if len(got) > naming.PubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), naming.PubsubMax)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (want, +got) = %v", diff)
		}
	}
}

func TestGenerateSubscriberRetryTopicName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_default_default_%s", subscriptionUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_with-dashes_more-dashes_%s", subscriptionUID),
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_%s_%s_%s", maxNamespace, strings.Repeat("n", truncatedNameMaxForSub), subscriptionUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_default_%s_%s", strings.Repeat("n", truncatedNameMaxForSub+(naming.K8sNamespaceMax-7)), subscriptionUID),
	}}

	for _, tc := range testCases {
		got := GenerateSubscriberRetryTopicName(channel(tc.ns, tc.n, tc.uid), subscriptionUID)
		if len(got) > naming.PubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), naming.PubsubMax)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (want, +got) = %v", diff)
		}
	}
}

func TestGenerateSubscriberRetrySubscriptionName(t *testing.T) {
	testCases := []struct {
		ns   string
		n    string
		uid  string
		want string
	}{{
		ns:   "default",
		n:    "default",
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_default_default_%s", subscriptionUID),
	}, {
		ns:   "with-dashes",
		n:    "more-dashes",
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_with-dashes_more-dashes_%s", subscriptionUID),
	}, {
		ns:   maxNamespace,
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_%s_%s_%s", maxNamespace, strings.Repeat("n", truncatedNameMaxForSub), subscriptionUID),
	}, {
		ns:   "default",
		n:    maxName,
		uid:  testUID,
		want: fmt.Sprintf("cre-sub_default_%s_%s", strings.Repeat("n", truncatedNameMaxForSub+(naming.K8sNamespaceMax-7)), subscriptionUID),
	}}

	for _, tc := range testCases {
		got := GenerateSubscriberRetrySubscriptionName(channel(tc.ns, tc.n, tc.uid), subscriptionUID)
		if len(got) > naming.PubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), naming.PubsubMax)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (want, +got) = %v", diff)
		}
	}
}

func channel(ns, n, uid string) *v1beta1.Channel {
	return &v1beta1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      n,
			UID:       types.UID(uid),
		},
	}
}
