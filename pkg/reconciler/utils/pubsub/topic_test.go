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

package pubsub

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"

	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	utilspubsubtesting "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub/testing"
)

const (
	project = "test-project"
	topic   = "test-topic"
)

var (
	// obj can be anything that implements runtime.Object. In real reconcilers this should be the object being reconciled.
	obj         = &corev1.Namespace{}
	su          = &utilspubsubtesting.StatusUpdater{}
	topicConfig = pubsub.TopicConfig{}
)

func TestReconcileTopic(t *testing.T) {
	tests := []testCase{
		{
			name:               "new topic created",
			wantEvents:         []string{`Normal TopicCreated Created PubSub topic "test-topic"`},
			wantTopicCondition: apis.Condition{Status: corev1.ConditionTrue},
		},
		{
			name:               "topic already exists",
			pre:                []reconcilertesting.PubsubAction{reconcilertesting.Topic(topic)},
			wantTopicCondition: apis.Condition{Status: corev1.ConditionTrue},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr, cleanup := newTestRunner(t, tc)
			defer cleanup()
			r := NewReconciler(tr.client, tr.recorder)
			su := &utilspubsubtesting.StatusUpdater{}
			res, err := r.ReconcileTopic(context.Background(), topic, &topicConfig, obj, su)

			tr.verify(t, tc, su, err)
			verifyTopic(t, res)
		})
	}

}

func TestDeleteTopic(t *testing.T) {
	tests := []testCase{
		{
			name:       "topic deleted",
			pre:        []reconcilertesting.PubsubAction{reconcilertesting.Topic(topic)},
			wantEvents: []string{`Normal TopicDeleted Deleted PubSub topic "test-topic"`},
		},
		{
			name: "nothing to delete",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr, cleanup := newTestRunner(t, tc)
			defer cleanup()
			r := NewReconciler(tr.client, tr.recorder)
			su := &utilspubsubtesting.StatusUpdater{}
			err := r.DeleteTopic(context.Background(), topic, obj, su)
			tr.verify(t, tc, su, err)
			exists, err := tr.client.Topic(topic).Exists(context.Background())
			if err != nil {
				t.Fatalf("Failed to verify topic exists: %v", err)
			}
			if exists {
				t.Errorf("Topic still exists")
			}
		})
	}

}

func verifyTopic(t *testing.T, got *pubsub.Topic) {
	want := fmt.Sprintf("projects/%s/topics/%s", project, topic)
	if got.String() != want {
		t.Errorf("Unexpected topic, got: %v, want:%v", got.String(), want)
	}
	gotConfig, err := got.Config(context.Background())
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	if diff := cmp.Diff(gotConfig, topicConfig); diff != "" {
		t.Errorf("Unexpected config, got:%+v, want: %+v", gotConfig, topicConfig)
	}
}

type testCase struct {
	name               string
	pre                []reconcilertesting.PubsubAction
	wantSubConfig      *pubsub.SubscriptionConfig
	wantEvents         []string
	wantTopicCondition apis.Condition
	wantSubCondition   apis.Condition
}

// testRunner helps to setup resources such as pubsub client, as well as verify the common test case.
type testRunner struct {
	client   *pubsub.Client
	close    func()
	recorder *record.FakeRecorder
}

func newTestRunner(t *testing.T, tc testCase) (*testRunner, func()) {
	client, close := reconcilertesting.TestPubsubClient(context.Background(), project)
	for _, action := range tc.pre {
		action(context.Background(), t, client)
	}
	return &testRunner{
		client:   client,
		recorder: record.NewFakeRecorder(len(tc.wantEvents)),
	}, close
}

func (r *testRunner) verify(t *testing.T, tc testCase, su *utilspubsubtesting.StatusUpdater, err error) {
	for _, event := range tc.wantEvents {
		got := <-r.recorder.Events
		if got != event {
			t.Errorf("Unexpected event recorded, got: %v, want: %v", got, event)
		}
	}

	if diff := cmp.Diff(tc.wantTopicCondition, su.TopicCondition); diff != "" {
		t.Errorf("Unexpected topic condition, diff: %s", diff)
	}

	if diff := cmp.Diff(tc.wantSubCondition, su.SubCondition); diff != "" {
		t.Errorf("Unexpected subscription condition, diff: %s", diff)
	}
}
