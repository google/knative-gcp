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

package intevents

import (
	"context"

	"knative.dev/pkg/configmap"

	pubsubClient "github.com/google/knative-gcp/pkg/client/injection/client"
	gpubsub "github.com/google/knative-gcp/pkg/gclient/pubsub"
	"github.com/google/knative-gcp/pkg/reconciler"
)

type PubSubBaseArgs struct {
	ControllerAgentName  string
	ReceiveAdapterName   string
	ReceiveAdapterType   string
	ConfigWatcher        configmap.Watcher
	// TODO remove after 0.16 cut.
	PubsubClientProvider gpubsub.CreateFn
}

func NewPubSubBase(ctx context.Context, args *PubSubBaseArgs) *PubSubBase {
	return &PubSubBase{
		Base:               reconciler.NewBase(ctx, args.ControllerAgentName, args.ConfigWatcher),
		pubsubClient:       pubsubClient.Get(ctx),
		receiveAdapterName: args.ReceiveAdapterName,
		receiveAdapterType: args.ReceiveAdapterType,
		// TODO remove after 0.16 cut.
		pubsubClientProvider: args.PubsubClientProvider,
	}
}
