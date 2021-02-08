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

package probe

import (
	"context"
	"net/http"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/wire"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/knative-gcp/test/test_images/probe_helper/probe/handlers"
	"github.com/google/knative-gcp/test/test_images/probe_helper/utils"
)

var HelperSet wire.ProviderSet = wire.NewSet(
	NewHelper,
	NewPubSubClient,
	NewCePubSubClient,
	NewK8sClient,
	NewStorageClient,
	NewCeForwardClient,
	NewCeReceiverClient,
	NewCeReceiverClientOptions,
	NewCeForwardClientOptions,
)

func NewHelper(env EnvConfig, handler handlers.Interface, ceForwardClient handlers.CeForwardClient, ceReceiveClient handlers.CeReceiveClient, livenessCheker *utils.LivenessChecker) *Helper {
	ph := &Helper{
		env:             env,
		probeHandler:    handler,
		ceForwardClient: ceForwardClient,
		ceReceiveClient: ceReceiveClient,
		livenessChecker: livenessCheker,
	}
	ph.lastForwardEventTime.SetNow()
	ph.lastReceiverEventTime.SetNow()
	ph.livenessChecker.AddActionFunc(ph.CheckLastEventTimes())
	return ph
}

func NewCeReceiverClient(ctx context.Context, livenessChecker *utils.LivenessChecker, opts ReceiveClientOptions) (handlers.CeReceiveClient, error) {
	injectReceiverPath := cloudevents.WithMiddleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			req.Header.Set(utils.ProbeEventReceiverPathHeader, req.URL.Path)
			next.ServeHTTP(rw, req)
		})
	})
	livenessCheck := cloudevents.WithGetHandlerFunc(livenessChecker.LivenessHandlerFunc(ctx))
	opts = append(opts, injectReceiverPath)
	opts = append(opts, livenessCheck)
	rp, err := cloudevents.NewHTTP(opts...)
	if err != nil {
		return nil, err
	}
	return cloudevents.NewClient(rp)
}

func NewCeForwardClient(opts ForwardClientOptions) (handlers.CeForwardClient, error) {
	sp, err := cloudevents.NewHTTP(opts...)
	if err != nil {
		return nil, err
	}
	return cloudevents.NewClient(sp)
}

func NewCeReceiverClientOptions(port ReceivePort) ReceiveClientOptions {
	opts := []cehttp.Option{cloudevents.WithPort(int(port))}
	return opts
}

func NewCeForwardClientOptions(port ForwardPort) ForwardClientOptions {
	opts := []cehttp.Option{cloudevents.WithPort(int(port))}
	return opts
}

func NewCePubSubClient(ctx context.Context, pc *pubsub.Client) (handlers.CePubSubClient, error) {
	pst, err := cepubsub.New(ctx, cepubsub.WithClient(pc))
	if err != nil {
		return nil, err
	}
	return cloudevents.NewClient(pst)
}

func NewPubSubClient(ctx context.Context, projectID clients.ProjectID) (c *pubsub.Client, err error) {
	return pubsub.NewClient(ctx, string(projectID))
}

func NewStorageClient(ctx context.Context) (c *storage.Client, err error) {
	return storage.NewClient(ctx)
}

func NewK8sClient(ctx context.Context) (c kubernetes.Interface, err error) {
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, err
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	return kubernetes.Interface(k8sClient), err
}

type ForwardPort int
type ReceivePort int
type ForwardClientOptions []cehttp.Option
type ReceiveClientOptions []cehttp.Option
