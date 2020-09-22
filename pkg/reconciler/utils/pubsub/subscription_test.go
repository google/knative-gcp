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
	"reflect"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	utilspubsubtesting "github.com/google/knative-gcp/pkg/reconciler/utils/pubsub/testing"
)

const (
	sub = "test-sub"
)

func TestReconcileSub(t *testing.T) {
	tests := []testCase{
		{
			name:             "new sub created",
			pre:              []reconcilertesting.PubsubAction{reconcilertesting.Topic(topic)},
			wantEvents:       []string{`Normal SubscriptionCreated Created PubSub subscription "test-sub"`},
			wantSubCondition: apis.Condition{Status: corev1.ConditionTrue},
		},
		{
			name:             "sub already exists",
			pre:              []reconcilertesting.PubsubAction{reconcilertesting.TopicAndSub(topic, sub)},
			wantSubCondition: apis.Condition{Status: corev1.ConditionTrue},
		},
		{
			name: "deleted topic",
			// First create the topic and sub to simulate a stable state. Then delete the topic to simulate manual topic
			// deletion, then recreate it to simulate topic reconciliation before sub reconciliation.
			pre: []reconcilertesting.PubsubAction{reconcilertesting.TopicAndSub(topic, sub), deleteTopic, reconcilertesting.Topic(topic)},
			wantEvents: []string{
				`Warning TopicDeleted Unexpected topic deletion detected for subscription: "test-sub"`,
				`Normal SubscriptionDeleted Deleted PubSub subscription "test-sub"`,
				`Normal SubscriptionCreated Created PubSub subscription "test-sub"`,
			},
			wantSubCondition: apis.Condition{Status: corev1.ConditionTrue},
		},
		{
			name: "sub already exists, modify config",
			pre:  []reconcilertesting.PubsubAction{reconcilertesting.TopicAndSub(topic, sub)},
			wantSubConfig: &pubsub.SubscriptionConfig{
				RetryPolicy: &pubsub.RetryPolicy{
					MinimumBackoff: time.Second,
					MaximumBackoff: time.Second,
				},
				DeadLetterPolicy: &pubsub.DeadLetterPolicy{
					DeadLetterTopic:     "some-topic-id",
					MaxDeliveryAttempts: 10,
				},
			},
			wantEvents: []string{
				`Normal SubscriptionConfigUpdated Updated config for PubSub subscription "test-sub"`,
			},
			wantSubCondition: apis.Condition{Status: corev1.ConditionTrue},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr, cleanup := newTestRunner(t, tc)
			defer cleanup()
			r := NewReconciler(tr.client, tr.recorder)
			su := &utilspubsubtesting.StatusUpdater{}
			subConfig := pubsub.SubscriptionConfig{Topic: tr.client.Topic(topic)}
			if tc.wantSubConfig != nil {
				subConfig.Labels = tc.wantSubConfig.Labels
				subConfig.RetryPolicy = tc.wantSubConfig.RetryPolicy
				subConfig.DeadLetterPolicy = tc.wantSubConfig.DeadLetterPolicy
			}
			res, err := r.ReconcileSubscription(context.Background(), sub, subConfig, obj, su)

			tr.verify(t, tc, su, err)
			if res != nil {
				verifySub(t, res, subConfig)
			}
		})
	}

}

func TestDeleteSub(t *testing.T) {
	tests := []testCase{
		{
			name:       "sub deleted",
			pre:        []reconcilertesting.PubsubAction{reconcilertesting.TopicAndSub(topic, sub)},
			wantEvents: []string{`Normal SubscriptionDeleted Deleted PubSub subscription "test-sub"`},
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
			err := r.DeleteSubscription(context.Background(), sub, obj, su)
			tr.verify(t, tc, su, err)
			exists, err := tr.client.Subscription(sub).Exists(context.Background())
			if err != nil {
				t.Fatalf("Failed to verify sub exists: %v", err)
			}
			if exists {
				t.Errorf("Sub still exists")
			}
		})
	}

}

func deleteTopic(ctx context.Context, t *testing.T, c *pubsub.Client) {
	if err := c.Topic(topic).Delete(ctx); err != nil {
		t.Fatalf("Failed to delete topic: %v", err)
	}
}

func verifySub(t *testing.T, got *pubsub.Subscription, wantConfig pubsub.SubscriptionConfig) {
	want := fmt.Sprintf("projects/%s/subscriptions/%s", project, sub)
	if got.String() != want {
		t.Errorf("Unexpected sub, got: %v, want:%v", got.String(), want)
	}
	gotConfig, err := got.Config(context.Background())
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	if !reflect.DeepEqual(gotConfig.Topic, wantConfig.Topic) {
		t.Errorf("Unexpected topic for sub, got:%+v, want: %+v", gotConfig.Topic, wantConfig.Topic)
	}
	if !reflect.DeepEqual(gotConfig.Labels, wantConfig.Labels) {
		t.Errorf("Unexpected labels in config, got:%+v, want: %+v", gotConfig.Labels, wantConfig.Labels)
	}
	if !reflect.DeepEqual(gotConfig.RetryPolicy, wantConfig.RetryPolicy) {
		t.Errorf("Unexpected retry policy in config, got:%+v, want: %+v", gotConfig.RetryPolicy, wantConfig.RetryPolicy)
	}
	if !reflect.DeepEqual(gotConfig.DeadLetterPolicy, wantConfig.DeadLetterPolicy) {
		t.Errorf("Unexpected dead letter policy in config, got:%+v, want: %+v", gotConfig.DeadLetterPolicy, wantConfig.DeadLetterPolicy)
	}
}
