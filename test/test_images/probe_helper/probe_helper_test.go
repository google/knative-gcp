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

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"cloud.google.com/go/storage"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"knative.dev/pkg/logging"
	logtest "knative.dev/pkg/logging/testing"

	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
	"github.com/google/knative-gcp/pkg/pubsub/adapter/converters"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

const (
	// the port that the dummy broker listens to
	dummyBrokerPort = 9999
	// the port that the probe helper listens to
	probeHelperPort = 8070
	// the port that the probe receiver component listens to
	probeReceiverPort = 8080
	//
	testProjectID = "test-project-id"
	//
	testTopicID = "cloudpubsubsource-topic"
	//
	testStorageBucket = "cloudstoragesource-bucket"
	//
	testSubscriptionID = "cre-src-test-subscription-id"
)

var (
	probeReceiverURL = fmt.Sprintf("http://localhost:%d", probeReceiverPort)
	probeHelperURL   = fmt.Sprintf("http://localhost:%d", probeHelperPort)
	dummyBrokerURL   = fmt.Sprintf("http://localhost:%d", dummyBrokerPort)

	testStorageUploadRequest      = fmt.Sprintf("/upload/storage/v1/b/%s/o?alt=json&name=cloudstoragesource-probe-1234567890&prettyPrint=false&projection=full&uploadType=multipart", testStorageBucket)
	testStorageRequest            = fmt.Sprintf("/storage/v1/b/%s/o/cloudstoragesource-probe-1234567890?alt=json&prettyPrint=false&projection=full", testStorageBucket)
	testStorageGenerationRequest  = fmt.Sprintf("/storage/v1/b/%s/o/cloudstoragesource-probe-1234567890?alt=json&generation=0&prettyPrint=false", testStorageBucket)
	testStorageCreateBody         = fmt.Sprintf(`{"bucket":"%s","name":"cloudstoragesource-probe-1234567890"}`, testStorageBucket)
	testStorageUpdateMetadataBody = fmt.Sprintf(`{"bucket":"%s","metadata":{"some-key":"Metadata updated!"}}`, testStorageBucket)
	testStorageArchiveBody        = fmt.Sprintf(`{"bucket":"%s","name":"cloudstoragesource-probe-1234567890","storageClass":"ARCHIVE"}`, testStorageBucket)
)

// A helper function that starts a dummy broker which receives events forwarded by the probe helper and delivers the events
// back to the probe helper's receive port
func runDummyBroker(ctx context.Context, t *testing.T) {
	logger := logging.FromContext(ctx)
	bp, err := cloudevents.NewHTTP(cloudevents.WithPort(dummyBrokerPort), cloudevents.WithTarget(probeReceiverURL))
	if err != nil {
		logger.Fatalf("Failed to create http protocol of the dummy Broker, %v", err)
	}
	bc, err := cloudevents.NewClient(bp)
	if err != nil {
		logger.Fatalf("Failed to create the dummy Broker client, ", err)
	}
	bc.StartReceiver(ctx, func(event cloudevents.Event) {
		if res := bc.Send(ctx, event); !cloudevents.IsACK(res) {
			logger.Fatalf("Failed to send CloudEvent from the dummy Broker: %v", res)
		}
	})
}

func runDummyCloudPubSubSource(ctx context.Context, sub *pubsub.Subscription, converter converters.Converter, t *testing.T) {
	logger := logging.FromContext(ctx)
	cp, err := cloudevents.NewHTTP(cloudevents.WithTarget(probeReceiverURL))
	if err != nil {
		logger.Fatalf("Failed to create http protocol of the dummy CloudPubSubSource, %v", err)
	}
	c, err := cloudevents.NewClient(cp)
	if err != nil {
		logger.Fatalf("Failed to create the dummy CloudPubSubSource client, ", err)
	}
	msgHandler := func(ctx context.Context, msg *pubsub.Message) {
		event, err := converter.Convert(ctx, msg, converters.CloudPubSub)
		if err != nil {
			logger.Fatalf("Could not convert message to CloudEvent: %v", err)
		}
		if res := c.Send(ctx, *event); !cloudevents.IsACK(res) {
			logger.Fatalf("Failed to send CloudEvent from the dummy CloudPubSubSource: %v", err)
		}
	}
	if err := sub.Receive(ctx, msgHandler); err != nil {
		logger.Fatalf("Could not receive from subscription: %v", err)
	}
}

func runDummyCloudStorageSource(ctx context.Context, gotRequest chan requestData, converter converters.Converter, t *testing.T) {
	logger := logging.FromContext(ctx)
	cp, err := cloudevents.NewHTTP(cloudevents.WithTarget(probeReceiverURL))
	if err != nil {
		logger.Fatalf("Failed to create http protocol of the dummy CloudPubSubSource, %v", err)
	}
	c, err := cloudevents.NewClient(cp)
	if err != nil {
		logger.Fatalf("Failed to create the dummy CloudPubSubSource client, ", err)
	}
	for {
		select {
		case req := <-gotRequest:
			if req.method == "POST" && req.url == testStorageUploadRequest && strings.Contains(req.body, testStorageCreateBody) {
				finalizeEvent := newCloudEvent(
					cloudEvent{
						CeID:      "1234567890",
						CeSubject: schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"),
						CeType:    schemasv1.CloudStorageObjectFinalizedEventType,
						CeSource:  schemasv1.CloudStorageEventSource(testStorageBucket),
					},
				)
				if res := c.Send(ctx, *finalizeEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object finalized CloudEvent from the dummy CloudStorageSource: %v", res)
				}
			} else if req.method == "PATCH" && req.url == testStorageRequest && strings.Contains(req.body, testStorageUpdateMetadataBody) {
				updateMetadataEvent := newCloudEvent(
					cloudEvent{
						CeID:      "1234567890",
						CeSubject: schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"),
						CeType:    schemasv1.CloudStorageObjectMetadataUpdatedEventType,
						CeSource:  schemasv1.CloudStorageEventSource(testStorageBucket),
					},
				)
				if res := c.Send(ctx, *updateMetadataEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object metadata updated CloudEvent from the dummy CloudStorageSource: %v", res)
				}
			} else if req.method == "POST" && req.url == testStorageUploadRequest && strings.Contains(req.body, testStorageArchiveBody) {
				archivedEvent := newCloudEvent(
					cloudEvent{
						CeID:      "1234567890",
						CeSubject: schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"),
						CeType:    schemasv1.CloudStorageObjectArchivedEventType,
						CeSource:  schemasv1.CloudStorageEventSource(testStorageBucket),
					},
				)
				if res := c.Send(ctx, *archivedEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object archived CloudEvent from the dummy CloudStorageSource: %v", res)
				}
			} else if req.method == "DELETE" && req.url == testStorageGenerationRequest {
				deletedEvent := newCloudEvent(
					cloudEvent{
						CeID:      "1234567890",
						CeSubject: schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"),
						CeType:    schemasv1.CloudStorageObjectDeletedEventType,
						CeSource:  schemasv1.CloudStorageEventSource(testStorageBucket),
					},
				)
				if res := c.Send(ctx, *deletedEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object deleted CloudEvent from the dummy CloudStorageSource: %v", res)
				}
			}
		}
	}
}

type cloudEvent struct {
	CeID      string
	CeSubject string
	CeSource  string
	CeType    string
}

func newCloudEvent(e cloudEvent) *cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(e.CeID)
	event.SetSubject(e.CeSubject)
	event.SetType(e.CeType)
	event.SetSource(e.CeSource)
	event.SetTime(time.Time{})
	return &event
}

func probeEvent(name, subject string) *cloudevents.Event {
	return newCloudEvent(
		cloudEvent{
			CeID:      name + "-1234567890",
			CeSubject: subject,
			CeSource:  "probe-helper-test",
			CeType:    name,
		},
	)
}

func newTestServer(handler func(w http.ResponseWriter, r *http.Request)) (*http.Client, func()) {
	ts := httptest.NewTLSServer(http.HandlerFunc(handler))
	tlsConf := &tls.Config{InsecureSkipVerify: true}
	tr := &http.Transport{
		TLSClientConfig: tlsConf,
		DialTLS: func(netw, addr string) (net.Conn, error) {
			return tls.Dial("tcp", ts.Listener.Addr().String(), tlsConf)
		},
	}
	return &http.Client{Transport: tr}, func() {
		tr.CloseIdleConnections()
		ts.Close()
	}
}

func testPubsubClient(ctx context.Context, t *testing.T, projectID string) (*pubsub.Client, func()) {
	srv := pstest.NewServer()
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial test pubsub connection: %v", err)
	}
	close := func() {
		srv.Close()
		conn.Close()
	}
	c, err := pubsub.NewClient(ctx, projectID, option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("Failed to create test pubsub client: %v", err)
	}
	return c, close
}

type eventAndResult struct {
	event      *cloudevents.Event
	wantResult protocol.Result
}

type requestData struct {
	method string
	url    string
	body   string
}

func TestProbeHelper(t *testing.T) {
	t.Skip("Skip this test from running on Prow as it is only for local development.")

	os.Setenv("K_SINK", dummyBrokerURL)
	os.Setenv("PROJECT_ID", testProjectID)
	os.Setenv("CLOUDPUBSUBSOURCE_TOPIC_ID", testTopicID)
	os.Setenv("CLOUDSTORAGESOURCE_BUCKET_ID", testStorageBucket)

	cases := []struct {
		name  string
		steps []eventAndResult
	}{{
		name: "Broker E2E delivery probe",
		steps: []eventAndResult{
			{
				event:      probeEvent("broker-e2e-delivery-probe", ""),
				wantResult: cloudevents.ResultACK,
			},
		},
	}, {
		name: "CloudPubSubSource probe",
		steps: []eventAndResult{
			{
				event:      probeEvent("cloudpubsubsource-probe", ""),
				wantResult: cloudevents.ResultACK,
			},
		},
	}, {
		name: "CloudStorageSource probe",
		steps: []eventAndResult{
			{
				event:      probeEvent("cloudstoragesource-probe", "create"),
				wantResult: cloudevents.ResultACK,
			},
			{
				event:      probeEvent("cloudstoragesource-probe", "update-metadata"),
				wantResult: cloudevents.ResultACK,
			},
			{
				event:      probeEvent("cloudstoragesource-probe", "archive"),
				wantResult: cloudevents.ResultACK,
			},
			{
				event:      probeEvent("cloudstoragesource-probe", "delete"),
				wantResult: cloudevents.ResultACK,
			},
		},
	}, {
		name: "Unrecognized probe event type",
		steps: []eventAndResult{
			{
				event:      probeEvent("unrecognized-probe-type", ""),
				wantResult: cloudevents.ResultNACK,
			},
		},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := logtest.TestContextWithLogger(t)
			ctx = WithProjectKey(ctx, testProjectID)
			ctx = WithTopicKey(ctx, testTopicID)
			ctx = WithSubscriptionKey(ctx, testSubscriptionID)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			converter := converters.NewPubSubConverter()

			// set up the resources for testing the CloudPubSubSource
			pubsubClient, cancel := testPubsubClient(ctx, t, testProjectID)
			defer cancel()
			topic, err := pubsubClient.CreateTopic(ctx, testTopicID)
			if err != nil {
				t.Fatalf("Failed to create test topic: %v", err)
			}
			sub, err := pubsubClient.CreateSubscription(ctx, testSubscriptionID, pubsub.SubscriptionConfig{
				Topic: topic,
			})
			if err != nil {
				t.Fatalf("Failed to create test subscription: %v", err)
			}

			// set up resources for testing the CloudStorageSource
			gotRequest := make(chan requestData, 1)
			hc, close := newTestServer(func(w http.ResponseWriter, r *http.Request) {
				body, _ := ioutil.ReadAll(r.Body)
				gotRequest <- requestData{
					method: r.Method,
					url:    r.URL.String(),
					body:   string(body),
				}
				w.Write([]byte("{}"))
			})
			defer close()
			storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(hc))

			// start a goroutine to run the dummy probe helper
			go runProbeHelper(ctx, pubsubClient, storageClient)

			// start a goroutine to run the dummy CloudPubSubSource
			go runDummyCloudPubSubSource(ctx, sub, converter, t)

			// start a goroutine to run the dummy CloudStorageSource
			go runDummyCloudStorageSource(ctx, gotRequest, converter, t)

			// start a goroutine to run the dummy Broker for testing Broker E2E delivery
			go runDummyBroker(ctx, t)

			p, err := cloudevents.NewHTTP(cloudevents.WithTarget(probeHelperURL))
			if err != nil {
				t.Fatalf("Failed to create HTTP protocol of the testing client: %s", err.Error())
			}
			c, err := cloudevents.NewClient(p)
			if err != nil {
				t.Fatalf("Failed to create testing client: %s", err.Error())
			}

			for _, step := range tc.steps {
				if result := c.Send(ctx, *step.event); !errors.Is(result, step.wantResult) {
					t.Fatalf("wanted result %+v, got %+v", step.wantResult, result)
				}
			}
		})
	}
}
