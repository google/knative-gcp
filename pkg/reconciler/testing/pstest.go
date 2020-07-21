/*
Copyright 2020 Google LLC.

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

package testing

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/sets"
	rtesting "knative.dev/pkg/reconciler/testing"
)

type PubsubAction func(context.Context, *testing.T, *pubsub.Client)

func Topic(id string) PubsubAction {
	return func(ctx context.Context, t *testing.T, c *pubsub.Client) {
		_, err := c.CreateTopic(ctx, id)
		if err != nil {
			t.Fatalf("Error creating topic %q: %v", id, err)
		}
		t.Logf("Created topic %q", id)
	}
}

func SubscriptionWithTopic(id string, tid string) PubsubAction {
	return func(ctx context.Context, t *testing.T, c *pubsub.Client) {
		_, err := c.CreateSubscription(ctx, id, pubsub.SubscriptionConfig{Topic: c.Topic(tid)})
		if err != nil {
			t.Fatalf("Error creating subscription %q: %v", id, err)
		}
		t.Logf("Created subscription %q", id)
	}
}

func TopicAndSub(tid, sid string) PubsubAction {
	return func(ctx context.Context, t *testing.T, c *pubsub.Client) {
		Topic(tid)(ctx, t, c)
		SubscriptionWithTopic(sid, tid)(ctx, t, c)
	}
}

func TopicExists(id string) func(*testing.T, *rtesting.TableRow) {
	return func(t *testing.T, r *rtesting.TableRow) {
		c := getPubsubClient(r)
		exist, err := c.Topic(id).Exists(context.Background())
		if err != nil {
			t.Errorf("Error checking topic existence: %v", err)
		} else if !exist {
			t.Errorf("Expected topic %q to exist", id)
		}
	}
}

func OnlyTopics(ids ...string) func(*testing.T, *rtesting.TableRow) {
	return func(t *testing.T, r *rtesting.TableRow) {
		c := getPubsubClient(r)
		it := c.Topics(context.Background())
		want := sets.NewString(ids...)
		got := sets.NewString()
		for {
			topic, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Errorf("Error iterating topics: %v", err)
				return
			}
			got.Insert(topic.ID())
		}
		if extra := got.Difference(want); extra.Len() != 0 {
			t.Errorf("Unexpected topics: %v", extra.List())
		}
		if missing := want.Difference(got); missing.Len() != 0 {
			t.Errorf("Missing expected topics: %v", missing.List())
		}
	}
}

func NoTopicsExist() func(*testing.T, *rtesting.TableRow) {
	return OnlyTopics()
}

func SubscriptionExists(id string) func(*testing.T, *rtesting.TableRow) {
	return func(t *testing.T, r *rtesting.TableRow) {
		c := getPubsubClient(r)
		exist, err := c.Subscription(id).Exists(context.Background())
		if err != nil {
			t.Errorf("Error checking subscription existence: %v", err)
		} else if !exist {
			t.Errorf("Expected subscription %q to exist", id)
		}
	}
}

func SubscriptionHasRetryPolicy(id string, wantPolicy *pubsub.RetryPolicy) func(*testing.T, *rtesting.TableRow) {
	return func(t *testing.T, r *rtesting.TableRow) {
		c := getPubsubClient(r)
		sub := c.Subscription(id)
		cfg, err := sub.Config(context.Background())
		if err != nil {
			t.Errorf("Error getting pubsub config: %v", err)
		}
		if diff := cmp.Diff(wantPolicy, cfg.RetryPolicy); diff != "" {
			t.Errorf("Pubsub config retry policy (-want,+got): %v", diff)
		}
	}
}

func OnlySubscriptions(ids ...string) func(*testing.T, *rtesting.TableRow) {
	return func(t *testing.T, r *rtesting.TableRow) {
		c := getPubsubClient(r)
		it := c.Subscriptions(context.Background())
		want := sets.NewString(ids...)
		got := sets.NewString()
		for {
			subscription, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Errorf("Error iterating subscriptions: %v", err)
				return
			}
			got.Insert(subscription.ID())
		}
		if extra := got.Difference(want); extra.Len() != 0 {
			t.Errorf("Unexpected subscriptions: %v", extra.List())
		}
		if missing := want.Difference(got); missing.Len() != 0 {
			t.Errorf("Missing expected subscriptions: %v", missing.List())
		}
	}
}

func NoSubscriptionsExist() func(*testing.T, *rtesting.TableRow) {
	return OnlySubscriptions()
}

func InjectPubsubClient(td map[string]interface{}, c *pubsub.Client) {
	td["_psclient"] = c
}

func getPubsubClient(r *rtesting.TableRow) *pubsub.Client {
	return r.OtherTestData["_psclient"].(*pubsub.Client)
}

func TestPubsubClient(ctx context.Context, projectID string) (*pubsub.Client, func()) {
	srv := pstest.NewServer()
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		panic(fmt.Errorf("failed to dial test pubsub connection: %v", err))
	}
	close := func() {
		srv.Close()
		conn.Close()
	}
	c, err := pubsub.NewClient(context.Background(), projectID, option.WithGRPCConn(conn))
	if err != nil {
		panic(fmt.Errorf("failed to create test pubsub client: %v", err))
	}
	return c, close
}
