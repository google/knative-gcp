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

package logadmin

import (
	"context"

	"cloud.google.com/go/logging/logadmin"
)

// Client matches the interface exposed by logadmin.Client
// see https://godoc.org/cloud.google.com/go/logging/logadmin#Client
type Client interface {
	// Close: https://godoc.org/cloud.google.com/go/logging/logadmin#Client.Close
	Close() error
	// CreateSink: https://godoc.org/cloud.google.com/go/logging/logadmin#Client.CreateSink
	CreateSink(ctx context.Context, sink *logadmin.Sink) (*logadmin.Sink, error)
	// CreateSinkOpt: https://godoc.org/cloud.google.com/go/logging/logadmin#Client.CreateSinkOpt
	CreateSinkOpt(ctx context.Context, sink *logadmin.Sink, opts logadmin.SinkOptions) (*logadmin.Sink, error)
	// DeleteSink: https://godoc.org/cloud.google.com/go/logging/logadmin#Client.DeleteSink
	DeleteSink(ctx context.Context, sinkID string) error
	// Sink: https://godoc.org/cloud.google.com/go/logging/logadmin#Client.Sink
	Sink(ctx context.Context, sinkID string) (*logadmin.Sink, error)
}
