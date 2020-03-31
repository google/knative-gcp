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
	"github.com/cloudevents/sdk-go/v2/client"
)

type Option func(*handler)

// WithInboundClient specifies the inbound client to receive events.
func WithInboundClient(c client.Client) Option {
	return func(h *handler) {
		h.inbound = c
	}
}

// WithDecoupleSink specifies the decouple sink for the ingress to send events to.
func WithDecoupleSink(d DecoupleSink) Option {
	return func(h *handler) {
		h.decouple = d
	}
}
