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

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
	"k8s.io/client-go/tools/record"
)

// CreateFn is a factory function to create a Pub/Sub client. It is copied from gclient/pubsub
// to avoid unnecessary dependency on gpubsub.
type CreateFn func(ctx context.Context, projectID string, opts ...option.ClientOption) (*pubsub.Client, error)

type Reconciler struct {
	client   *pubsub.Client
	recorder record.EventRecorder
}

func NewReconciler(client *pubsub.Client, recorder record.EventRecorder) *Reconciler {
	return &Reconciler{
		client:   client,
		recorder: recorder,
	}
}

// StatusUpdater is an interface which updates resource status based on pubsub reconciliation results.
type StatusUpdater interface {
	MarkTopicFailed(reason, format string, args ...interface{})
	MarkTopicUnknown(reason, format string, args ...interface{})
	MarkTopicReady()
	MarkSubscriptionFailed(reason, format string, args ...interface{})
	MarkSubscriptionUnknown(reason, format string, args ...interface{})
	MarkSubscriptionReady()
}
