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
	"bytes"
	"context"
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
	// the port that the test broker listens to
	testBrokerPort = 9999
	// the port that the probe helper listens to
	probeHelperPort = 8070
	// the port that the probe receiver component listens to
	probeReceiverPort = 8080
	// the fake project ID used by the test resources
	testProjectID = "test-project-id"
	// the fake pubsub topic ID used in the test CloudPubSubSource
	testTopicID = "cloudpubsubsource-topic"
	// the fake pubsub subscription ID used in the test CloudPubSubSource
	testSubscriptionID = "cre-src-test-subscription-id"
	// the fake Cloud Storage bucket ID used in the test CloudStorageSource
	testStorageBucket = "cloudstoragesource-bucket"
)

var (
	probeReceiverURL = fmt.Sprintf("http://localhost:%d", probeReceiverPort)
	probeHelperURL   = fmt.Sprintf("http://localhost:%d", probeHelperPort)
	testBrokerURL    = fmt.Sprintf("http://localhost:%d", testBrokerPort)

	testStorageUploadRequest      = fmt.Sprintf("/upload/storage/v1/b/%s/o?alt=json&name=cloudstoragesource-probe-1234567890&prettyPrint=false&projection=full&uploadType=multipart", testStorageBucket)
	testStorageRequest            = fmt.Sprintf("/b/%s/o/cloudstoragesource-probe-1234567890?alt=json&prettyPrint=false&projection=full", testStorageBucket)
	testStorageGenerationRequest  = fmt.Sprintf("/b/%s/o/cloudstoragesource-probe-1234567890?alt=json&generation=0&prettyPrint=false", testStorageBucket)
	testStorageCreateBody         = fmt.Sprintf(`{"bucket":"%s","name":"cloudstoragesource-probe-1234567890"}`, testStorageBucket)
	testStorageUpdateMetadataBody = fmt.Sprintf(`{"bucket":"%s","metadata":{"some-key":"Metadata updated!"}}`, testStorageBucket)
	testStorageArchiveBody        = fmt.Sprintf(`{"bucket":"%s","name":"cloudstoragesource-probe-1234567890","storageClass":"ARCHIVE"}`, testStorageBucket)
)

// A helper function that starts a test Broker which receives events forwarded by
// the probe helper and delivers the events back to the probe helper's receive port.
func runTestBroker(ctx context.Context) {
	logger := logging.FromContext(ctx)
	bp, err := cloudevents.NewHTTP(cloudevents.WithPort(testBrokerPort), cloudevents.WithTarget(probeReceiverURL))
	if err != nil {
		logger.Fatalf("Failed to create http protocol of the test Broker, %v", err)
	}
	bc, err := cloudevents.NewClient(bp)
	if err != nil {
		logger.Fatalf("Failed to create the test Broker client, ", err)
	}
	bc.StartReceiver(ctx, func(event cloudevents.Event) {
		if res := bc.Send(ctx, event); !cloudevents.IsACK(res) {
			logger.Fatalf("Failed to send CloudEvent from the test Broker: %v", res)
		}
	})
}

// A helper function that starts a test CloudPubSubSource which watches a pubsub
// Subscription for messages and delivers them as CloudEvents to the probe
// helper's receive port.
func runTestCloudPubSubSource(ctx context.Context, sub *pubsub.Subscription) {
	logger := logging.FromContext(ctx)
	converter := converters.NewPubSubConverter()
	cp, err := cloudevents.NewHTTP(cloudevents.WithTarget(probeReceiverURL))
	if err != nil {
		logger.Fatalf("Failed to create http protocol of the test CloudPubSubSource, %v", err)
	}
	c, err := cloudevents.NewClient(cp)
	if err != nil {
		logger.Fatalf("Failed to create the test CloudPubSubSource client, ", err)
	}
	msgHandler := func(ctx context.Context, msg *pubsub.Message) {
		event, err := converter.Convert(ctx, msg, converters.CloudPubSub)
		if err != nil {
			logger.Fatalf("Could not convert message to CloudEvent: %v", err)
		}
		if res := c.Send(ctx, *event); !cloudevents.IsACK(res) {
			logger.Fatalf("Failed to send CloudEvent from the test CloudPubSubSource: %v", err)
		}
	}
	for {
		if err := sub.Receive(ctx, msgHandler); err != nil {
			logger.Fatalf("Could not receive from subscription: %v", err)
		}
	}
}

// A helper function that starts a test CloudStorageSource which intercepts
// Cloud Storage HTTP requests and forwards the appropriate notifications as
// CloudEvents to the probe helper's receive port.
func runTestCloudStorageSource(ctx context.Context, gotRequest chan *http.Request) {
	logger := logging.FromContext(ctx)
	cp, err := cloudevents.NewHTTP(cloudevents.WithTarget(probeReceiverURL))
	if err != nil {
		logger.Fatalf("Failed to create http protocol of the test CloudStorageSource, %v", err)
	}
	c, err := cloudevents.NewClient(cp)
	if err != nil {
		logger.Fatalf("Failed to create the test CloudStorageSource client, ", err)
	}
	for {
		select {
		case req := <-gotRequest:
			bodyBytes, err := ioutil.ReadAll(req.Body)
			if err != nil {
				logger.Fatalf("Failed to read request body in test CloudStorageSource, ", err)
			}
			body := string(bodyBytes)
			method := req.Method
			url := req.URL.String()
			if method == "POST" && url == testStorageUploadRequest && strings.Contains(body, testStorageCreateBody) {
				// This request indicates the client's intent to create a new object.
				finalizeEvent := cloudevents.NewEvent()
				finalizeEvent.SetID("1234567890")
				finalizeEvent.SetSubject(schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"))
				finalizeEvent.SetType(schemasv1.CloudStorageObjectFinalizedEventType)
				finalizeEvent.SetSource(schemasv1.CloudStorageEventSource(testStorageBucket))
				if res := c.Send(ctx, finalizeEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object finalized CloudEvent from the test CloudStorageSource: %v", res)
				}
			} else if method == "PATCH" && url == testStorageRequest && strings.Contains(body, testStorageUpdateMetadataBody) {
				// This request indicates the client's intent to update the object's metadata.
				updateMetadataEvent := cloudevents.NewEvent()
				updateMetadataEvent.SetID("1234567890")
				updateMetadataEvent.SetSubject(schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"))
				updateMetadataEvent.SetType(schemasv1.CloudStorageObjectMetadataUpdatedEventType)
				updateMetadataEvent.SetSource(schemasv1.CloudStorageEventSource(testStorageBucket))
				if res := c.Send(ctx, updateMetadataEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object metadata updated CloudEvent from the test CloudStorageSource: %v", res)
				}
			} else if method == "POST" && url == testStorageUploadRequest && strings.Contains(body, testStorageArchiveBody) {
				// This request indicates the client's intent to archive the object.
				archivedEvent := cloudevents.NewEvent()
				archivedEvent.SetID("1234567890")
				archivedEvent.SetSubject(schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"))
				archivedEvent.SetType(schemasv1.CloudStorageObjectArchivedEventType)
				archivedEvent.SetSource(schemasv1.CloudStorageEventSource(testStorageBucket))
				if res := c.Send(ctx, archivedEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object archived CloudEvent from the test CloudStorageSource: %v", res)
				}
			} else if method == "DELETE" && url == testStorageGenerationRequest {
				// This request indicates the client's intent to delete the object.
				deletedEvent := cloudevents.NewEvent()
				deletedEvent.SetID("1234567890")
				deletedEvent.SetSubject(schemasv1.CloudStorageEventSubject("cloudstoragesource-probe-1234567890"))
				deletedEvent.SetType(schemasv1.CloudStorageObjectDeletedEventType)
				deletedEvent.SetSource(schemasv1.CloudStorageEventSource(testStorageBucket))
				if res := c.Send(ctx, deletedEvent); !cloudevents.IsACK(res) {
					logger.Fatalf("Failed to send object deleted CloudEvent from the test CloudStorageSource: %v", res)
				}
			}
		}
	}
}

// Creates a new CloudEvent in the shape of probe events sent to the probe helper.
func probeEvent(name, subject string) *cloudevents.Event {
	event := cloudevents.NewEvent()
	event.SetID(name + "-1234567890")
	event.SetSubject(subject)
	event.SetSource("probe-helper-test")
	event.SetType(name)
	event.SetTime(time.Now())
	return &event
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

func testStorageClient(ctx context.Context, t *testing.T) (*storage.Client, chan *http.Request, func()) {
	gotRequest := make(chan *http.Request, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The test Cloud Storage server forwards the client's generated HTTP requests.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logging.FromContext(ctx).Fatal("Test Cloud Storage server could not read request body.")
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		gotRequest <- r
		w.Write([]byte("{}"))
	}))
	c, err := storage.NewClient(ctx, option.WithEndpoint(srv.URL))
	if err != nil {
		t.Fatalf("Failed to create test storage client: %v", err)
	}
	return c, gotRequest, srv.Close
}

func testBrokerClient(ctx context.Context, t *testing.T) (*storage.Client, chan *http.Request, func()) {
	gotRequest := make(chan *http.Request, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The test Cloud Storage server forwards the client's generated HTTP requests.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logging.FromContext(ctx).Fatal("Test Cloud Storage server could not read request body.")
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		gotRequest <- r
		w.Write([]byte("{}"))
	}))
	c, err := storage.NewClient(ctx, option.WithEndpoint(srv.URL))
	if err != nil {
		t.Fatalf("Failed to create test storage client: %v", err)
	}
	return c, gotRequest, srv.Close
}

type eventAndResult struct {
	event      *cloudevents.Event
	wantResult protocol.Result
}

func TestProbeHelper(t *testing.T) {
	t.Skip("Skip this test from running on Prow as it is only for local development.")

	os.Setenv("K_SINK", testBrokerURL)
	os.Setenv("PROJECT_ID", testProjectID)
	os.Setenv("CLOUDPUBSUBSOURCE_TOPIC_ID", testTopicID)
	os.Setenv("CLOUDSTORAGESOURCE_BUCKET_ID", testStorageBucket)

	ctx := logtest.TestContextWithLogger(t)
	ctx = WithProjectKey(ctx, testProjectID)
	ctx = WithTopicKey(ctx, testTopicID)
	ctx = WithSubscriptionKey(ctx, testSubscriptionID)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up the resources for testing the CloudPubSubSource.
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

	// Set up resources for testing the CloudStorageSource.
	storageClient, gotRequest, cancel := testStorageClient(ctx, t)
	defer cancel()

	// Start a goroutine to run the test probe helper. Listen on the desired ports.
	receiverListener, err := net.Listen("tcp", fmt.Sprintf(":%d", probeReceiverPort))
	if err != nil {
		t.Fatalf("Failed to start receiver listener: %v", err)
	}
	probeListener, err := net.Listen("tcp", fmt.Sprintf(":%d", probeHelperPort))
	if err != nil {
		t.Fatalf("Failed to start probe listener: %v", err)
	}
	go runProbeHelper(ctx, receiverListener, probeListener, pubsubClient, storageClient)

	// Start a goroutine to run the test CloudPubSubSource.
	go runTestCloudPubSubSource(ctx, sub)

	// Start a goroutine to run the test CloudStorageSource.
	go runTestCloudStorageSource(ctx, gotRequest)

	// Start a goroutine to run the test Broker for testing Broker E2E delivery.
	go runTestBroker(ctx)

	// Create a testing client from which to send probe events to the probe helper.
	p, err := cloudevents.NewHTTP(cloudevents.WithTarget(probeHelperURL))
	if err != nil {
		t.Fatalf("Failed to create HTTP protocol of the testing client: %s", err.Error())
	}
	c, err := cloudevents.NewClient(p)
	if err != nil {
		t.Fatalf("Failed to create testing client: %s", err.Error())
	}

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
			for _, step := range tc.steps {
				if result := c.Send(ctx, *step.event); !errors.Is(result, step.wantResult) {
					t.Fatalf("wanted result %+v, got %+v", step.wantResult, result)
				}
			}
		})
	}
}
