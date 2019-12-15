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

// Package testing provides a fake logadmin client for test purposes.
package testing

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"cloud.google.com/go/logging/logadmin"
	glogadmin "github.com/google/knative-gcp/pkg/gclient/logging/logadmin"
)

var (
	errClientClosed = errors.New("client is closed")
)

// TestClientCreator returns a logadmin.CreateFn used to construct the test logadmin client.
func TestClientCreator(value interface{}) glogadmin.CreateFn {
	var data TestClientConfiguration
	var ok bool
	if data, ok = value.(TestClientConfiguration); !ok {
		data = TestClientConfiguration{}
	}
	if data.CreateClientErr != nil {
		return func(_ context.Context, _ string, _ ...option.ClientOption) (glogadmin.Client, error) {
			return nil, data.CreateClientErr
		}
	}
	parents := make(map[string]*sinkMap)
	var lock sync.Mutex
	return func(_ context.Context, parent string, _ ...option.ClientOption) (glogadmin.Client, error) {
		lock.Lock()
		defer lock.Unlock()
		sinks, ok := parents[parent]
		if !ok {
			sinks = &sinkMap{
				sinks: make(map[string]logadmin.Sink),
			}
			parents[parent] = sinks
		}
		return &testClient{
			sinks: sinks,
			data:  data,
		}, nil
	}
}

// TestClientConfiguration is the data used to configure the fake logadmin client.
type TestClientConfiguration struct {
	CreateClientErr error
	CloseErr        error
	CreateSinkErr   error
	DeleteSinkErr   error
	SinkErr         error
}

type sinkMap struct {
	lock  sync.RWMutex
	sinks map[string]logadmin.Sink
}

// testClient is the test Scheduler client.
type testClient struct {
	sinks  *sinkMap
	data   TestClientConfiguration
	closed bool
}

// Verify that it satisfies the scheduler.Client interface.
var _ glogadmin.Client = &testClient{}

// Close implements client.Close
func (c *testClient) Close() error {
	if c.closed {
		return nil
	}
	if c.data.CloseErr == nil {
		c.closed = true
	}
	return c.data.CloseErr
}

func (c *testClient) CreateSink(ctx context.Context, sink *logadmin.Sink) (*logadmin.Sink, error) {
	return c.CreateSinkOpt(ctx, sink, logadmin.SinkOptions{})
}

func (c *testClient) CreateSinkOpt(ctx context.Context, sink *logadmin.Sink, opts logadmin.SinkOptions) (*logadmin.Sink, error) {
	if c.closed {
		return nil, errClientClosed
	}
	if c.data.CreateSinkErr != nil {
		return nil, c.data.CreateSinkErr
	}
	c.sinks.lock.Lock()
	defer c.sinks.lock.Unlock()
	if _, ok := c.sinks.sinks[sink.ID]; ok {
		return nil, status.Errorf(codes.AlreadyExists, "sink %s already exists", sink.ID)
	}
	newSink := *sink
	if opts.UniqueWriterIdentity {
		newSink.WriterIdentity = fmt.Sprintf("writer-identity-%s", sink.ID)
	} else {
		newSink.WriterIdentity = "writer-identity"
	}
	c.sinks.sinks[sink.ID] = newSink
	return &newSink, nil
}

func (c *testClient) DeleteSink(ctx context.Context, sinkID string) error {
	if c.closed {
		return errClientClosed
	}
	if c.data.DeleteSinkErr != nil {
		return c.data.DeleteSinkErr
	}
	c.sinks.lock.Lock()
	defer c.sinks.lock.Unlock()
	if _, ok := c.sinks.sinks[sinkID]; !ok {
		return status.Errorf(codes.NotFound, "sink %s not found", sinkID)
	}
	delete(c.sinks.sinks, sinkID)
	return nil
}

func (c *testClient) Sink(ctx context.Context, sinkID string) (*logadmin.Sink, error) {
	if c.closed {
		return nil, errClientClosed
	}
	if c.data.SinkErr != nil {
		return nil, c.data.SinkErr
	}
	c.sinks.lock.RLock()
	defer c.sinks.lock.RUnlock()
	if sink, ok := c.sinks.sinks[sinkID]; ok {
		return &sink, nil
	}
	return nil, status.Errorf(codes.NotFound, "sink %s not found", sinkID)
}
