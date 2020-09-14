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
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

const (
	BrokerE2EDeliveryProbeEventType              = "broker-e2e-delivery-probe"
	CloudPubSubSourceProbeEventType              = "cloudpubsubsource-probe"
	CloudStorageSourceProbeEventType             = "cloudstoragesource-probe"
	CloudStorageSourceProbeCreateSubject         = "create"
	CloudStorageSourceProbeUpdateMetadataSubject = "update-metadata"
	CloudStorageSourceProbeArchiveSubject        = "archive"
	CloudStorageSourceProbeDeleteSubject         = "delete"
	CloudAuditLogsSourceProbeEventType           = "cloudauditlogssource-probe"
	CloudAuditLogsSourceProbeCreateSubject       = "create-topic"
	CloudAuditLogsSourceProbeDeleteSubject       = "delete-topic"
)

type healthChecker struct {
	lastProbeEventTimestamp    eventTimestamp
	lastReceiverEventTimestamp eventTimestamp
	maxStaleDuration           time.Duration
	port                       int
}

func (t *eventTimestamp) reportHealth() {
	t.Lock()
	defer t.Unlock()
	t.time = time.Now()
}

func (t *eventTimestamp) lastTime() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.time
}

func (c *healthChecker) ServeHTTP(w nethttp.ResponseWriter, req *nethttp.Request) {
	if req.URL.Path != "/healthz" {
		w.WriteHeader(nethttp.StatusNotFound)
		return
	}
	now := time.Now()
	if (now.Sub(c.lastProbeEventTimestamp.lastTime()) > c.maxStaleDuration) ||
		(now.Sub(c.lastReceiverEventTimestamp.lastTime()) > c.maxStaleDuration) {
		w.WriteHeader(nethttp.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(nethttp.StatusOK)
}

func (c *healthChecker) start(ctx context.Context) {
	c.lastProbeEventTimestamp.reportHealth()
	c.lastReceiverEventTimestamp.reportHealth()

	srv := &nethttp.Server{
		Addr:    ":" + strconv.Itoa(c.port),
		Handler: c,
	}

	go func() {
		logging.FromContext(ctx).Info("Starting the probe helper health checker...")
		if err := srv.ListenAndServe(); err != nil && err != nethttp.ErrServerClosed {
			logging.FromContext(ctx).Error("The probe helper health checker has stopped unexpectedly", zap.Error(err))
		}
	}()

	<-ctx.Done()
	if err := srv.Shutdown(ctx); err != nil && err != context.Canceled {
		logging.FromContext(ctx).Error("Failed to shutdown the probe helper health checker", zap.Error(err))
	}
}

type cloudEventsFunc func(cloudevents.Event) protocol.Result

type eventTimestamp struct {
	sync.RWMutex
	time time.Time
}

type receivedEventsMap struct {
	sync.RWMutex
	channels map[string]chan bool
}

func (r *receivedEventsMap) createReceiverChannel(channelID string) (chan bool, error) {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.channels[channelID]; ok {
		return nil, fmt.Errorf("Receiver channel already exists for key %v", channelID)
	}
	receiverChannel := make(chan bool, 1)
	r.channels[channelID] = receiverChannel
	return receiverChannel, nil
}

func (r *receivedEventsMap) deleteReceiverChannel(channelID string) {
	r.Lock()
	if ch, ok := r.channels[channelID]; ok {
		close(ch)
		delete(r.channels, channelID)
	}
	r.Unlock()
}

func logNACK(ctx context.Context, format string, args ...interface{}) protocol.Result {
	return logReceive(ctx, false, format, args...)
}

func logACK(ctx context.Context, format string, args ...interface{}) protocol.Result {
	return logReceive(ctx, true, format, args...)
}

func logReceive(ctx context.Context, ack bool, format string, args ...interface{}) protocol.Result {
	logging.FromContext(ctx).Infof(format, args...)
	return cloudevents.NewReceipt(ack, format, args...)
}

func isValidProbeEvent(event cloudevents.Event) bool {
	eventType := event.Type()
	eventSubject := event.Subject()
	return (eventType == BrokerE2EDeliveryProbeEventType ||
		eventType == CloudPubSubSourceProbeEventType ||
		(eventType == CloudStorageSourceProbeEventType && (eventSubject == CloudStorageSourceProbeCreateSubject ||
			eventSubject == CloudStorageSourceProbeUpdateMetadataSubject ||
			eventSubject == CloudStorageSourceProbeArchiveSubject ||
			eventSubject == CloudStorageSourceProbeDeleteSubject)) ||
		(eventType == CloudAuditLogsSourceProbeEventType && (eventSubject == CloudAuditLogsSourceProbeCreateSubject ||
			eventSubject == CloudAuditLogsSourceProbeDeleteSubject)))
}

func (ph *ProbeHelper) forwardFromProbe(ctx context.Context, brokerClient cloudevents.Client, cePubsubClient cloudevents.Client, pubsubClient *pubsub.Client, bucket *storage.BucketHandle, receivedEvents *receivedEventsMap) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var err error
		var receiverChannel chan bool
		logging.FromContext(ctx).Info("Received probe request", zap.Any("event", event))
		ph.healthChecker.lastProbeEventTimestamp.reportHealth()

		if !isValidProbeEvent(event) {
			return logNACK(ctx, "Probe forwarding failed, unrecognized probe event type and subject: type=%v, subject=%v", event.Type(), event.Subject())
		}

		channelID := event.ID()
		if event.Subject() != "" {
			channelID = channelID + "-" + event.Subject()
		}
		receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
		if err != nil {
			return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
		}
		defer receivedEvents.deleteReceiverChannel(channelID)

		ctx, cancel := context.WithTimeout(ctx, ph.timeoutDuration)
		defer cancel()

		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			// The broker client forwards the event to the broker.
			if res := brokerClient.Send(ctx, event); !cloudevents.IsACK(res) {
				return logNACK(ctx, "Error when sending event %v to broker: %+v \n", event.ID(), res)
			}
		case CloudPubSubSourceProbeEventType:
			// The pubsub client forwards the event as a message to a pubsub topic.
			if res := cePubsubClient.Send(ctx, event); !cloudevents.IsACK(res) {
				return logNACK(ctx, "Error when publishing event %v to pubsub topic: %+v \n", event.ID(), res)
			}
		case CloudStorageSourceProbeEventType:
			obj := bucket.Object(event.ID())
			switch event.Subject() {
			case CloudStorageSourceProbeCreateSubject:
				// The storage client writes an object named as the event ID.
				if err := obj.NewWriter(ctx).Close(); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error closing storage writer for object %v: %v", event.ID(), err)
				}
			case CloudStorageSourceProbeUpdateMetadataSubject:
				// The storage client updates the object metadata.
				objectAttrs := storage.ObjectAttrsToUpdate{
					Metadata: map[string]string{
						"some-key": "Metadata updated!",
					},
				}
				if _, err := obj.Update(ctx, objectAttrs); err != nil {
					return logNACK(ctx, "Probe forwarding failed, could not update metadata for object %v: %v", event.ID(), err)
				}
			case CloudStorageSourceProbeArchiveSubject:
				// The storage client updates the object's storage class to ARCHIVE.
				w := obj.NewWriter(ctx)
				w.ObjectAttrs.StorageClass = "ARCHIVE"
				if err := w.Close(); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error closing storage writer for object %v: %v", event.ID(), err)
				}
			case CloudStorageSourceProbeDeleteSubject:
				// Deleting a specific version of an object deletes it forever.
				objectAttrs, err := obj.Attrs(ctx)
				if err != nil {
					return logNACK(ctx, "Probe forwarding failed, error getting attributes for object %v: %v", event.ID(), err)
				}
				if err := obj.Generation(objectAttrs.Generation).Delete(ctx); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error deleting object %v: %v", event.ID(), err)
				}
			}
		case CloudAuditLogsSourceProbeEventType:
			switch event.Subject() {
			case CloudAuditLogsSourceProbeCreateSubject:
				// Create a pubsub topic with the given ID.
				if _, err := pubsubClient.CreateTopic(ctx, event.ID()); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error creating pubsub topic %v: %v", event.ID(), err)
				}
			case CloudAuditLogsSourceProbeDeleteSubject:
				// Delete the pubsub topic with the given ID.
				if err := pubsubClient.Topic(event.ID()).Delete(ctx); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error deleting pubsub topic %v: %v", event.ID(), err)
				}
			}
		}

		select {
		case <-receiverChannel:
			return cloudevents.ResultACK
		case <-ctx.Done():
			return logNACK(ctx, "Timed out waiting for the receiver channel: %v \n", channelID)
		}
	}
}

func (ph *ProbeHelper) receiveEvent(ctx context.Context, receivedEvents *receivedEventsMap) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var channelID string
		logging.FromContext(ctx).Info("Received event", zap.Any("event", event))
		ph.healthChecker.lastReceiverEventTimestamp.reportHealth()

		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			// The event is received as sent.
			channelID = event.ID()
		case schemasv1.CloudPubSubMessagePublishedEventType:
			// The original event is wrapped into a pubsub Message by the CloudEvents
			// pubsub sender client, and encoded as data in a CloudEvent by the CloudPubSubSource.
			msgData := schemasv1.PushMessage{}
			if err := json.Unmarshal(event.Data(), &msgData); err != nil {
				return logACK(ctx, "Failed to unmarshal pubsub message data: %v", err)
			}
			var ok bool
			if channelID, ok = msgData.Message.Attributes["ce-id"]; !ok {
				return logACK(ctx, "Failed to read CloudEvent ID from pubsub message")
			}
		case schemasv1.CloudStorageObjectFinalizedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeCreateSubject
		case schemasv1.CloudStorageObjectMetadataUpdatedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeUpdateMetadataSubject
		case schemasv1.CloudStorageObjectArchivedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeArchiveSubject
		case schemasv1.CloudStorageObjectDeletedEventType:
			// The original event is written as an identifiable object to a bucket.
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeDeleteSubject
		case schemasv1.CloudAuditLogsLogWrittenEventType:
			// The logged event type is held in the methodname extension.
			if _, ok := event.Extensions()["methodname"]; !ok {
				return logACK(ctx, "CloudEvent does not have extension methodname: %v", event)
			}
			// For creation and deletion of pubsub topics, the topic ID can be extracted
			// from the event subject.
			sepSub := strings.Split(event.Subject(), "/")
			if len(sepSub) != 5 || sepSub[0] != "pubsub.googleapis.com" || sepSub[1] != "projects" || sepSub[3] != "topics" {
				return logACK(ctx, "Unexpected Cloud AuditLogs event subject: %v", event.Subject())
			}
			switch event.Extensions()["methodname"] {
			case "google.pubsub.v1.Publisher.CreateTopic":
				channelID = sepSub[4] + "-" + CloudAuditLogsSourceProbeCreateSubject
			case "google.pubsub.v1.Publisher.DeleteTopic":
				channelID = sepSub[4] + "-" + CloudAuditLogsSourceProbeDeleteSubject
			default:
				return logACK(ctx, "Unrecognized CloudEvent methodname extension: %v", event.Extensions()["methodname"])
			}
		default:
			return logACK(ctx, "Unrecognized event type: %v", event.Type())
		}

		receivedEvents.RLock()
		defer receivedEvents.RUnlock()
		receiver, ok := receivedEvents.channels[channelID]
		if !ok {
			return logACK(ctx, "This event is not received by the probe receiver client: %v \n", channelID)
		}
		receiver <- true
		return cloudevents.ResultACK
	}
}

type ProbeHelper struct {
	projectID string

	brokerURL string

	cloudPubSubSourceTopicID string
	pubsubClient             *pubsub.Client

	cloudStorageSourceBucketID string
	storageClient              *storage.Client

	probePort    int
	receiverPort int

	timeoutDuration time.Duration

	healthChecker *healthChecker
}

func (ph *ProbeHelper) run(ctx context.Context) {
	logger := logging.FromContext(ctx)

	// create pubsub client
	pst, err := cepubsub.New(ctx,
		cepubsub.WithClient(ph.pubsubClient),
		cepubsub.WithProjectID(ph.projectID),
		cepubsub.WithTopicID(ph.cloudPubSubSourceTopicID))
	if err != nil {
		logger.Fatal("Failed to create pubsub transport", zap.Error(err))
	}
	psc, err := cloudevents.NewClient(pst)
	if err != nil {
		logger.Fatal("Failed to create CloudEvents pubsub client", zap.Error(err))
	}

	// create cloud storage client
	if ph.storageClient == nil {
		ph.storageClient, err = storage.NewClient(ctx)
		if err != nil {
			logger.Fatal("Failed to create cloud storage client", zap.Error(err))
		}
	}
	bkt := ph.storageClient.Bucket(ph.cloudStorageSourceBucketID)

	// create sender client
	sp, err := cloudevents.NewHTTP(cloudevents.WithTarget(ph.brokerURL), cloudevents.WithPort(ph.probePort))
	if err != nil {
		logger.Fatal("Failed to create sender transport", zap.Error(err))
	}
	sc, err := cloudevents.NewClient(sp)
	if err != nil {
		logger.Fatal("Failed to create sender client", zap.Error(err))
	}

	// create receiver client
	rp, err := cloudevents.NewHTTP(cloudevents.WithPort(ph.receiverPort))
	if err != nil {
		logger.Fatal("Failed to create receiver transport", zap.Error(err))
	}
	rc, err := cloudevents.NewClient(rp)
	if err != nil {
		logger.Fatal("Failed to create receiver client", zap.Error(err))
	}

	// start the health checker
	if ph.healthChecker == nil {
		logger.Fatal("Unspecified health checker")
	}
	go ph.healthChecker.start(ctx)

	// make a map to store the channel for each event
	receivedEvents := &receivedEventsMap{
		channels: make(map[string]chan bool),
	}

	// start a goroutine to receive the event from probe and forward it appropriately
	logger.Info("Starting Probe Helper server...")
	go sc.StartReceiver(ctx, ph.forwardFromProbe(ctx, sc, psc, ph.pubsubClient, bkt, receivedEvents))

	// Receive the event and return the result back to the probe
	logger.Info("Starting event receiver...")
	rc.StartReceiver(ctx, ph.receiveEvent(ctx, receivedEvents))
}
