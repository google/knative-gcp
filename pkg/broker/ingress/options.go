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

package ingress

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/broker/config"
	"knative.dev/eventing/pkg/kncloudevents"
)

// HandlerOption is the option to configure ingress handler.
type HandlerOption func(*handler)

// WithDecoupleSink specifies the decouple sink for the ingress to send events to.
func WithDecoupleSink(d DecoupleSink) HandlerOption {
	return func(h *handler) {
		h.decouple = d
	}
}

// WithHttpReceiver specifies the HTTP receiver. It's used in tests to specify a test receiver.
func WithHttpReceiver(receiver HttpMessageReceiver) HandlerOption {
	return func(h *handler) {
		h.httpReceiver = receiver
	}
}

// WithPort specifies the port number that ingress listens on. It will create an HttpMessageReceiver for that port.
func WithPort(port int) HandlerOption {
	return func(h *handler) {
		h.httpReceiver = kncloudevents.NewHttpMessageReceiver(port)
	}
}

// MultiTopicDecoupleSinkOption is the option to configure multiTopicDecoupleSink.
type MultiTopicDecoupleSinkOption func(sink *multiTopicDecoupleSink)

// WithPubsubClient specifies the pubsub client to use.
func WithPubsubClient(client cloudevents.Client) MultiTopicDecoupleSinkOption {
	return func(sink *multiTopicDecoupleSink) {
		sink.client = client
	}
}

// WithBrokerConfig specifies the broker config. It can be created by reading a configmap mount.
func WithBrokerConfig(brokerConfig config.ReadonlyTargets) MultiTopicDecoupleSinkOption {
	return func(sink *multiTopicDecoupleSink) {
		sink.brokerConfig = brokerConfig
	}
}