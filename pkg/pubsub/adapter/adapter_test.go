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

package adapter

// TODO redo this tests in https://github.com/google/knative-gcp/issues/1333

//import (
//	"context"
//	"errors"
//	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
//	cev2 "github.com/cloudevents/sdk-go/v2"
//	"github.com/cloudevents/sdk-go/v2/binding"
//	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
//	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
//	"github.com/google/knative-gcp/pkg/utils/clients"
//	"io"
//	"net/http"
//	"net/http/httptest"
//	"testing"
//	"time"
//
//	"cloud.google.com/go/pubsub"
//	"cloud.google.com/go/pubsub/pstest"
//	"github.com/cloudevents/sdk-go/v2/event"
//	"google.golang.org/api/option"
//	"google.golang.org/grpc"
//)
//
//const (
//	testProjectID     = "test-testProjectID"
//	testTopic         = "test-testTopic"
//	testSub           = "test-testSub"
//	testName          = "test-testName"
//	testNamespace     = "test-testNamespace"
//	testResourceGroup = "test-testResourceGroup"
//	testConverterType = "test-testConverterType"
//)
//
//func testPubsubClient(ctx context.Context, t *testing.T, projectID string) (*pubsub.Client, func()) {
//	t.Helper()
//	srv := pstest.NewServer()
//	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
//	if err != nil {
//		t.Fatalf("failed to dial test pubsub connection: %v", err)
//	}
//	close := func() {
//		srv.Close()
//		conn.Close()
//	}
//	c, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
//	if err != nil {
//		t.Fatalf("failed to create test pubsub client: %v", err)
//	}
//	return c, close
//}
//
//type mockStatsReporter struct {
//	gotArgs *ReportArgs
//	gotCode int
//}
//
//func (r *mockStatsReporter) ReportEventCount(args *ReportArgs, responseCode int) error {
//	r.gotArgs = args
//	r.gotCode = responseCode
//	return nil
//}
//
//type mockConverter struct {
//	wantErr   bool
//	wantEvent *cev2.Event
//}
//
//func (c *mockConverter) Convert(ctx context.Context, msg *pubsub.Message, converterType converters.ConverterType) (*cev2.Event, error) {
//	if c.wantErr {
//		return nil, errors.New("induced error")
//	}
//	return c.wantEvent, nil
//}
//
//func TestAdapter(t *testing.T) {
//	ctx := context.Background()
//	c, close := testPubsubClient(ctx, t, testProjectID)
//	defer close()
//
//	topic, err := c.CreateTopic(ctx, testTopic)
//	if err != nil {
//		t.Fatalf("failed to create topic: %v", err)
//	}
//	sub, err := c.CreateSubscription(ctx, testSub, pubsub.SubscriptionConfig{
//		Topic: topic,
//	})
//	if err != nil {
//		t.Fatalf("failed to create subscription: %v", err)
//	}
//
//	p, err := cepubsub.New(context.Background(),
//		cepubsub.WithClient(c),
//		cepubsub.WithProjectID(testProjectID),
//		cepubsub.WithTopicID(testTopic),
//	)
//	if err != nil {
//		t.Fatalf("failed to create cloudevents pubsub protocol: %v", err)
//	}
//
//	outbound := http.DefaultClient
//
//	sinkClient, err := cehttp.New()
//	if err != nil {
//		t.Fatalf("failed to create sink cloudevents client: %v", err)
//	}
//	sinkSvr := httptest.NewServer(sinkClient)
//	defer sinkSvr.Close()
//
//	converter := &mockConverter{}
//	reporter := &mockStatsReporter{}
//	args := &AdapterArgs{
//		TopicID:       testTopic,
//		SinkURI:       sinkSvr.URL,
//		Extensions:    map[string]string{},
//		ConverterType: converters.ConverterType(testConverterType),
//	}
//	adapter := NewAdapter(ctx,
//		clients.ProjectID(testProjectID),
//		Namespace(testNamespace),
//		Name(testName),
//		ResourceGroup(testResourceGroup),
//		sub,
//		outbound,
//		converter,
//		reporter,
//		args)
//
//	adapter.Start(ctx, func(_ error) {})
//	defer adapter.Stop()
//
//	testEvent := event.New()
//	testEvent.SetID("id")
//	testEvent.SetSource("source")
//	testEvent.SetSubject("subject")
//	testEvent.SetType("type")
//
//	rctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
//	defer cancel()
//
//	go func() {
//		_, err := sinkClient.Receive(rctx)
//		if err != nil && err != io.EOF {
//			t.Errorf("unexpected error from sink receiving event: %v", err)
//		}
//	}()
//
//	if err := p.Send(ctx, binding.ToMessage(&testEvent)); err != nil {
//		t.Fatalf("failed to seed event to pubsub: %v", err)
//	}
//
//	<-rctx.Done()

//res := topic.Publish(context.Background(), &pubsub.Message{ID: "testid"})
//if _, err := res.Get(context.Background()); err != nil {
//	t.Fatalf("Failed to publish a msg to topic: %v", err)
//}
//}

//func TestReceive(t *testing.T) {
//	cases := []struct {
//		name           string
//		eventFn        func() cloudevents.Event
//		returnStatus   int
//		returnHeader   http.Header
//		returnBody     []byte
//		wantHeader     http.Header
//		wantBody       []byte
//		wantStatus     int
//		wantEventFn    func() *cloudevents.Event
//		wantReportArgs *ReportArgs
//		wantReportCode int
//		wantErr        bool
//		isSource       bool
//	}{{
//		name: "success without responding event",
//		eventFn: func() cloudevents.Event {
//			e := cloudevents.NewEvent(cloudevents.VersionV1)
//			e.SetSource("source")
//			e.SetType("unit.testing")
//			e.SetID("abc")
//			e.SetDataContentType("application/json")
//			e.Data = []byte(`{"key":"value"}`)
//			return e
//		},
//		returnStatus: http.StatusOK,
//		wantHeader: map[string][]string{
//			"Ce-Id":          {"abc"},
//			"Ce-Source":      {"source"},
//			"Ce-Specversion": {"1.0"},
//			"Ce-Type":        {"unit.testing"},
//			"Content-Length": {"15"},
//			"Content-Type":   {"application/json"},
//		},
//		wantBody:    []byte(`{"key":"value"}`),
//		wantEventFn: func() *cloudevents.Event { return nil },
//		wantReportArgs: &ReportArgs{
//			EventSource:   "source",
//			EventType:     "unit.testing",
//			ResourceGroup: "channels.messaging.cloud.google.com",
//		},
//		wantReportCode: 200,
//	}, {
//		name: "success without responding event and from source",
//		eventFn: func() cloudevents.Event {
//			e := cloudevents.NewEvent(cloudevents.VersionV1)
//			e.SetSource("source")
//			e.SetType("unit.testing")
//			e.SetID("abc")
//			e.SetDataContentType("application/json")
//			e.Data = []byte(`{"key":"value"}`)
//			return e
//		},
//		returnStatus: http.StatusOK,
//		isSource:     true,
//		wantHeader: map[string][]string{
//			"Ce-Id":          {"abc"},
//			"Ce-Source":      {"source"},
//			"Ce-Specversion": {"1.0"},
//			"Ce-Type":        {"unit.testing"},
//			"Content-Length": {"15"},
//			"Content-Type":   {"application/json"},
//		},
//		wantBody:    []byte(`{"key":"value"}`),
//		wantEventFn: func() *cloudevents.Event { return nil },
//		wantReportArgs: &ReportArgs{
//			EventSource:   "source",
//			EventType:     "unit.testing",
//			ResourceGroup: "pubsub.events.cloud.google.com",
//		},
//		wantReportCode: 200,
//	}, {
//		name: "success with responding event",
//		eventFn: func() cloudevents.Event {
//			e := cloudevents.NewEvent(cloudevents.VersionV1)
//			e.SetSource("source")
//			e.SetType("unit.testing")
//			e.SetID("abc")
//			e.SetDataContentType("application/json")
//			e.Data = []byte(`{"key":"value"}`)
//			return e
//		},
//		returnStatus: http.StatusOK,
//		returnHeader: map[string][]string{
//			"Ce-Id":          {"def"},
//			"Ce-Source":      {"reply-source"},
//			"Ce-Specversion": {"1.0"},
//			"Ce-Type":        {"unit.testing.reply"},
//			"Content-Type":   {"application/json"},
//		},
//		returnBody: []byte(`{"key2":"value2"}`),
//		wantHeader: map[string][]string{
//			"Ce-Id":          {"abc"},
//			"Ce-Source":      {"source"},
//			"Ce-Specversion": {"1.0"},
//			"Ce-Type":        {"unit.testing"},
//			"Content-Length": {"15"},
//			"Content-Type":   {"application/json"},
//		},
//		wantBody: []byte(`{"key":"value"}`),
//		wantEventFn: func() *cloudevents.Event {
//			e := cloudevents.NewEvent(cloudevents.VersionV1)
//			e.SetSource("reply-source")
//			e.SetType("unit.testing.reply")
//			e.SetID("def")
//			e.SetDataContentType("application/json")
//			e.Data = []byte(`{"key2":"value2"}`)
//			e.DataEncoded = true
//			return &e
//		},
//		wantStatus: 200,
//		wantReportArgs: &ReportArgs{
//			EventSource:   "source",
//			EventType:     "unit.testing",
//			ResourceGroup: "channels.messaging.cloud.google.com",
//		},
//		wantReportCode: 200,
//	}, {
//		name: "receiver internal error",
//		eventFn: func() cloudevents.Event {
//			e := cloudevents.NewEvent(cloudevents.VersionV1)
//			e.SetSource("source")
//			e.SetType("unit.testing")
//			e.SetID("abc")
//			e.SetDataContentType("application/json")
//			e.Data = []byte(`{"key":"value"}`)
//			return e
//		},
//		returnStatus: http.StatusInternalServerError,
//		wantHeader: map[string][]string{
//			"Ce-Id":          {"abc"},
//			"Ce-Source":      {"source"},
//			"Ce-Specversion": {"1.0"},
//			"Ce-Type":        {"unit.testing"},
//			"Content-Length": {"15"},
//			"Content-Type":   {"application/json"},
//		},
//		wantBody:    []byte(`{"key":"value"}`),
//		wantEventFn: func() *cloudevents.Event { return nil },
//		wantReportArgs: &ReportArgs{
//			EventSource:   "source",
//			EventType:     "unit.testing",
//			ResourceGroup: "channels.messaging.cloud.google.com",
//		},
//		wantReportCode: 500,
//		wantErr:        true,
//	}}
//
//	for _, tc := range cases {
//		t.Run(tc.name, func(t *testing.T) {
//			var gotHeader http.Header
//			var gotBody []byte
//			handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
//				gotHeader = req.Header
//				gotHeader.Del("Accept-Encoding")
//				gotHeader.Del("User-Agent")
//				b := bytes.NewBuffer(gotBody)
//				defer req.Body.Close()
//				io.Copy(b, req.Body)
//				gotBody = b.Bytes()
//
//				for k, vs := range tc.returnHeader {
//					for _, v := range vs {
//						w.Header().Add(k, v)
//					}
//				}
//				w.WriteHeader(tc.returnStatus)
//				w.Write(tc.returnBody)
//			})
//			server := httptest.NewServer(handler)
//
//			r := &mockStatsReporter{}
//			var resourceGroup string
//			if tc.isSource {
//				resourceGroup = "pubsub.events.cloud.google.com"
//			} else {
//				resourceGroup = "channels.messaging.cloud.google.com"
//			}
//			a := Adapter{
//				Project:       "proj",
//				Topic:         "topic",
//				Subscription:  "sub",
//				SendMode:      converters.Binary,
//				reporter:      r,
//				ResourceGroup: resourceGroup,
//			}
//
//			var err error
//			if a.outbound, err = a.newHTTPClient(context.Background(), server.URL); err != nil {
//				t.Fatalf("failed to to set adapter outbound to receive events: %v", err)
//			}
//
//			var resp cloudevents.EventResponse
//			err = a.receive(context.Background(), tc.eventFn(), &resp)
//
//			if (err != nil) != tc.wantErr {
//				t.Errorf("adapter.receiver got error %v want error %v", err, tc.wantErr)
//			}
//
//			options := make([]cmp.Option, 0)
//			ignoreCeTraceparent := cmpopts.IgnoreMapEntries(func(n string, _ []string) bool {
//				return n == "Ce-Traceparent"
//			})
//			options = append(options, ignoreCeTraceparent)
//			ignoreTraceParent := cmpopts.IgnoreMapEntries(func(n string, _ []string) bool {
//				return n == "Traceparent"
//			})
//			options = append(options, ignoreTraceParent)
//			if diff := cmp.Diff(tc.wantHeader, gotHeader, options...); diff != "" {
//				t.Errorf("receiver got unexpected HTTP header (-want +got): %s", diff)
//			}
//			if !bytes.Equal(tc.wantBody, gotBody) {
//				t.Errorf("receiver got HTTP body %v want %v", string(gotBody), string(tc.wantBody))
//			}
//			if resp.Status != tc.wantStatus {
//				t.Errorf("adapter.receiver got resp status %d want %d", resp.Status, tc.wantStatus)
//			}
//			if diff := cmp.Diff(tc.wantEventFn(), resp.Event); diff != "" {
//				t.Errorf("adapter.receiver got unexpected resp event (-want +got): %s", diff)
//			}
//			if diff := cmp.Diff(tc.wantReportArgs, r.gotArgs); diff != "" {
//				t.Errorf("stats reporter got unexpected args (-want +got): %s", diff)
//			}
//			if r.gotCode != tc.wantReportCode {
//				t.Errorf("stats reporter got status code %d want %d", r.gotCode, tc.wantReportCode)
//			}
//		})
//	}
//}
