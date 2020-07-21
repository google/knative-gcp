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

package kncloudevents

import (
	nethttp "net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

func NewDefaultClient(target ...string) (cloudevents.Client, error) {
	tOpts := []http.Option{
		cloudevents.WithRoundTripper(&ochttp.Transport{
			Base:        nethttp.DefaultTransport,
			Propagation: tracecontextb3.TraceContextEgress,
		}),
	}
	if len(target) > 0 && target[0] != "" {
		tOpts = append(tOpts, cloudevents.WithTarget(target[0]))
	}

	// Make an http transport for the CloudEvents client.
	t, err := cloudevents.NewHTTP(tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	c, err := cloudevents.NewClient(t,
		cloudevents.WithUUIDs(),
		cloudevents.WithTimeNow(),
	)

	if err != nil {
		return nil, err
	}
	return c, nil
}
