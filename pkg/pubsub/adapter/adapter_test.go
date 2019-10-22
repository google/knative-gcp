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

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"

	cloudevents "github.com/cloudevents/sdk-go"
	cepubsub "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/pubsub/context"
)

type mockStatsReporter struct {
	gotArgs *ReportArgs
	gotCode int
}

func (r *mockStatsReporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	r.gotArgs = args
	r.gotCode = responseCode
	return nil
}

func TestStartAdapter(t *testing.T) {
	t.Skipf("need to fix the error from call to newPubSubClient: %s", `pubsub: google: could not find default credentials. See https://developers.google.com/accounts/docs/application-default-credentials for more information.`)
	a := Adapter{
		Project:          "proj",
		Topic:            "top",
		Subscription:     "sub",
		Sink:             "http://localhost:8081",
		Transformer:      "http://localhost:8080",
		ExtensionsBase64: "eyJrZXkxIjoidmFsdWUxIiwia2V5MiI6InZhbHVlMiJ9Cg==",
	}
	// This test only does sanity checks to see if all fields are
	// initialized.
	// In reality, Start should be a blocking function. Here, it's not
	// blocking because we expect it to fail as it shouldn't be able to
	// connect to pubsub.
	if err := a.Start(context.Background()); err == nil {
		t.Fatal("adapter.Start got nil want error")
	}

	if a.SendMode == "" {
		t.Errorf("adapter.SendMode got %q want %q", a.SendMode, converters.DefaultSendMode)
	}
	if a.reporter == nil {
		t.Error("adapter.reporter got nil want a StatsReporter")
	}
	if a.inbound == nil {
		t.Error("adapter.inbound got nil want a cloudevents.Client")
	}
	if a.outbound == nil {
		t.Error("adapter.outbound got nil want a cloudevents.Client")
	}
	if a.transformer == nil {
		t.Error("adapter.transformer got nil want a cloudevents.Client")
	}
	wantExt := map[string]string{"key1": "value1", "key2": "value2"}
	if !cmp.Equal(wantExt, a.extensions) {
		t.Errorf("adapter.extensions got %v want %v", a.extensions, wantExt)
	}
}

func TestInboundConvert(t *testing.T) {
	cases := []struct {
		name          string
		ctx           context.Context
		message       *cepubsub.Message
		wantMessageFn func() *cloudevents.Event
		wantErr       bool
	}{{
		name: "pubsub event",
		ctx: pubsubcontext.WithTransportContext(
			context.Background(),
			pubsubcontext.NewTransportContext(
				"proj", "topic", "sub", "test",
				&pubsub.Message{ID: "abc"},
			),
		),
		message: &cepubsub.Message{
			Data: []byte("some data"),
			Attributes: map[string]string{
				"schema": "http://example.com",
				"key1":   "value1",
			},
		},
		wantMessageFn: func() *cloudevents.Event {
			e := cloudevents.NewEvent(cloudevents.VersionV03)
			e.SetID("abc")
			e.SetSource(v1alpha1.PubSubEventSource("proj", "topic"))
			e.SetDataContentType(*cloudevents.StringOfApplicationJSON())
			e.SetType(v1alpha1.PubSubPublish)
			e.SetDataSchema("http://example.com")
			e.SetExtension("knativecemode", string(converters.DefaultSendMode))
			e.Data = []byte("some data")
			e.DataEncoded = true
			e.SetExtension("key1", "value1")
			return &e
		},
	}, {
		name: "storage event",
		ctx: pubsubcontext.WithTransportContext(
			context.Background(),
			pubsubcontext.NewTransportContext(
				"proj", "topic", "sub", "test",
				&pubsub.Message{ID: "abc"},
			),
		),
		message: &cepubsub.Message{
			Data: []byte("some data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.storage",
				"bucketId":    "my-bucket",
				"objectId":    "my-obj",
				"key1":        "value1",
				"eventType":   "OBJECT_FINALIZE",
			},
		},
		wantMessageFn: func() *cloudevents.Event {
			e := cloudevents.NewEvent(cloudevents.VersionV03)
			e.SetID("abc")
			e.SetSource(v1alpha1.StorageEventSource("my-bucket"))
			e.SetSubject("my-obj")
			e.SetDataContentType(*cloudevents.StringOfApplicationJSON())
			e.SetType("com.google.cloud.storage.object.finalize")
			e.SetDataSchema("https://raw.githubusercontent.com/google/knative-gcp/master/schemas/storage/schema.json")
			e.Data = []byte("some data")
			e.DataEncoded = true
			e.SetExtension("key1", "value1")
			return &e
		},
	}, {
		name: "invalid storage event",
		ctx: pubsubcontext.WithTransportContext(
			context.Background(),
			pubsubcontext.NewTransportContext(
				"proj", "topic", "sub", "test",
				&pubsub.Message{ID: "abc"},
			),
		),
		message: &cepubsub.Message{
			Data: []byte("some data"),
			Attributes: map[string]string{
				"knative-gcp": "com.google.cloud.storage",
				"key1":        "value1",
				"eventType":   "OBJECT_FINALIZE",
			},
		},
		wantMessageFn: func() *cloudevents.Event { return nil },
		wantErr:       true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := Adapter{
				Project:      "proj",
				Topic:        "top",
				Subscription: "sub",
				SendMode:     converters.DefaultSendMode,
			}
			var err error
			gotEvent, err := a.convert(tc.ctx, tc.message, err)
			if (err != nil) != tc.wantErr {
				t.Errorf("adapter.convert got error %v want error=%v", err, tc.wantErr)
			}
			if diff := cmp.Diff(tc.wantMessageFn(), gotEvent); diff != "" {
				t.Errorf("adapter.convert got unexpeceted cloudevents.Event (-want +got) %s", diff)
			}
		})
	}
}

func TestReceive(t *testing.T) {
	cases := []struct {
		name           string
		eventFn        func() cloudevents.Event
		returnStatus   int
		returnHeader   http.Header
		returnBody     []byte
		wantHeader     http.Header
		wantBody       []byte
		wantStatus     int
		wantEventFn    func() *cloudevents.Event
		wantReportArgs *ReportArgs
		wantReportCode int
		wantErr        bool
	}{{
		name: "success without responding event",
		eventFn: func() cloudevents.Event {
			e := cloudevents.NewEvent()
			e.SetSource("source")
			e.SetType("unit.testing")
			e.SetID("abc")
			e.SetDataContentType("application/json")
			e.Data = []byte(`{"key":"value"}`)
			return e
		},
		returnStatus: http.StatusOK,
		wantHeader: map[string][]string{
			"Ce-Id":          []string{"abc"},
			"Ce-Source":      []string{"source"},
			"Ce-Specversion": []string{"0.2"},
			"Ce-Type":        []string{"unit.testing"},
			"Content-Length": []string{"15"},
			"Content-Type":   []string{"application/json"},
		},
		wantBody:    []byte(`{"key":"value"}`),
		wantEventFn: func() *cloudevents.Event { return nil },
		wantReportArgs: &ReportArgs{
			EventSource: "source",
			EventType:   "unit.testing",
		},
		wantReportCode: 200,
	}, {
		name: "success with responding event",
		eventFn: func() cloudevents.Event {
			e := cloudevents.NewEvent()
			e.SetSource("source")
			e.SetType("unit.testing")
			e.SetID("abc")
			e.SetDataContentType("application/json")
			e.Data = []byte(`{"key":"value"}`)
			return e
		},
		returnStatus: http.StatusOK,
		returnHeader: map[string][]string{
			"Ce-Id":          []string{"def"},
			"Ce-Source":      []string{"reply-source"},
			"Ce-Specversion": []string{"0.2"},
			"Ce-Type":        []string{"unit.testing.reply"},
			"Content-Type":   []string{"application/json"},
		},
		returnBody: []byte(`{"key2":"value2"}`),
		wantHeader: map[string][]string{
			"Ce-Id":          []string{"abc"},
			"Ce-Source":      []string{"source"},
			"Ce-Specversion": []string{"0.2"},
			"Ce-Type":        []string{"unit.testing"},
			"Content-Length": []string{"15"},
			"Content-Type":   []string{"application/json"},
		},
		wantBody: []byte(`{"key":"value"}`),
		wantEventFn: func() *cloudevents.Event {
			e := cloudevents.NewEvent()
			e.SetSource("reply-source")
			e.SetType("unit.testing.reply")
			e.SetID("def")
			e.SetDataContentType("application/json")
			e.Data = []byte(`{"key2":"value2"}`)
			e.DataEncoded = true
			return &e
		},
		wantStatus: 200,
		wantReportArgs: &ReportArgs{
			EventSource: "source",
			EventType:   "unit.testing",
		},
		wantReportCode: 200,
	}, {
		name: "receiver internal error",
		eventFn: func() cloudevents.Event {
			e := cloudevents.NewEvent()
			e.SetSource("source")
			e.SetType("unit.testing")
			e.SetID("abc")
			e.SetDataContentType("application/json")
			e.Data = []byte(`{"key":"value"}`)
			return e
		},
		returnStatus: http.StatusInternalServerError,
		wantHeader: map[string][]string{
			"Ce-Id":          []string{"abc"},
			"Ce-Source":      []string{"source"},
			"Ce-Specversion": []string{"0.2"},
			"Ce-Type":        []string{"unit.testing"},
			"Content-Length": []string{"15"},
			"Content-Type":   []string{"application/json"},
		},
		wantBody:    []byte(`{"key":"value"}`),
		wantEventFn: func() *cloudevents.Event { return nil },
		wantReportArgs: &ReportArgs{
			EventSource: "source",
			EventType:   "unit.testing",
		},
		wantReportCode: 500,
		wantErr:        true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotHeader http.Header
			var gotBody []byte
			handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				gotHeader = req.Header
				gotHeader.Del("Accept-Encoding")
				gotHeader.Del("User-Agent")
				b := bytes.NewBuffer(gotBody)
				defer req.Body.Close()
				io.Copy(b, req.Body)
				gotBody = b.Bytes()

				for k, vs := range tc.returnHeader {
					for _, v := range vs {
						w.Header().Add(k, v)
					}
				}
				w.WriteHeader(tc.returnStatus)
				w.Write(tc.returnBody)
			})
			server := httptest.NewServer(handler)

			r := &mockStatsReporter{}
			a := Adapter{
				Project:      "proj",
				Topic:        "topic",
				Subscription: "sub",
				SendMode:     converters.Binary,
				reporter:     r,
			}

			var err error
			if a.outbound, err = a.newHTTPClient(context.Background(), server.URL); err != nil {
				t.Fatalf("failed to to set adapter outbound to receive events: %v", err)
			}

			var resp cloudevents.EventResponse
			err = a.receive(context.Background(), tc.eventFn(), &resp)

			if (err != nil) != tc.wantErr {
				t.Errorf("adapter.receiver got error %v want error %v", err, tc.wantErr)
			}

			if diff := cmp.Diff(tc.wantHeader, gotHeader); diff != "" {
				t.Errorf("recevier got unexpected HTTP header (-want +got): %s", diff)
			}
			if !bytes.Equal(tc.wantBody, gotBody) {
				t.Errorf("receiver got HTTP body %v want %v", string(gotBody), string(tc.wantBody))
			}
			if resp.Status != tc.wantStatus {
				t.Errorf("adapter.receiver got resp status %d want %d", resp.Status, tc.wantStatus)
			}
			if diff := cmp.Diff(tc.wantEventFn(), resp.Event); diff != "" {
				t.Errorf("adapter.receiver got unexpected resp event (-want +got): %s", diff)
			}
			if diff := cmp.Diff(tc.wantReportArgs, r.gotArgs); diff != "" {
				t.Errorf("stats reporter got unexpected args (-want +got): %s", diff)
			}
			if r.gotCode != tc.wantReportCode {
				t.Errorf("stats reporter got status code %d want %d", r.gotCode, tc.wantReportCode)
			}
		})
	}
}
