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
	CloudSchedulerSourceProbeEventType           = "cloudschedulersource-probe"

	cloudSchedulerSourceProbeChannelID = "cloudschedulersource-probe-channel-id"
)

type healthChecker struct {
	lastProbeEventTimestamp    eventTimestamp
	lastReceiverEventTimestamp eventTimestamp
	maxStaleDuration           time.Duration
	port                       int
}

func (t *eventTimestamp) setNow() {
	t.Lock()
	defer t.Unlock()
	t.time = time.Now()
}

func (t *eventTimestamp) getTime() time.Time {
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
	if (now.Sub(c.lastProbeEventTimestamp.getTime()) > c.maxStaleDuration) ||
		(now.Sub(c.lastReceiverEventTimestamp.getTime()) > c.maxStaleDuration) {
		w.WriteHeader(nethttp.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(nethttp.StatusOK)
}

func (c *healthChecker) start(ctx context.Context) {
	c.lastProbeEventTimestamp.setNow()
	c.lastReceiverEventTimestamp.setNow()

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

func (ph *ProbeHelper) forwardFromProbe(ctx context.Context, brokerClient cloudevents.Client, pubsubClient cloudevents.Client, bucket *storage.BucketHandle, receivedEvents *receivedEventsMap) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var channelID string
		var err error
		var receiverChannel chan bool
		logging.FromContext(ctx).Infof("Received probe request: %+v \n", event)
		ph.healthChecker.lastProbeEventTimestamp.setNow()

		ctx, cancel := context.WithTimeout(ctx, ph.timeoutDuration)
		defer cancel()
		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			channelID = event.ID()
			receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
			if err != nil {
				return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
			}
			defer receivedEvents.deleteReceiverChannel(channelID)

			// The broker client forwards the event to the broker.
			if res := brokerClient.Send(ctx, event); !cloudevents.IsACK(res) {
				return logNACK(ctx, "Error when sending event %v to broker: %+v \n", event.ID(), res)
			}
		case CloudPubSubSourceProbeEventType:
			channelID = event.ID()
			receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
			if err != nil {
				return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
			}
			defer receivedEvents.deleteReceiverChannel(channelID)

			// The pubsub client forwards the event as a message to a pubsub topic.
			if res := pubsubClient.Send(ctx, event); !cloudevents.IsACK(res) {
				return logNACK(ctx, "Error when publishing event %v to pubsub topic: %+v \n", event.ID(), res)
			}
		case CloudStorageSourceProbeEventType:
			obj := bucket.Object(event.ID())
			switch event.Subject() {
			case CloudStorageSourceProbeCreateSubject:
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

				// The storage client writes an object named as the event ID.
				if err := obj.NewWriter(ctx).Close(); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error closing storage writer for object %v: %v", event.ID(), err)
				}
			case CloudStorageSourceProbeUpdateMetadataSubject:
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

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
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

				// The storage client updates the object's storage class to ARCHIVE.
				w := obj.NewWriter(ctx)
				w.ObjectAttrs.StorageClass = "ARCHIVE"
				if err := w.Close(); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error closing storage writer for object %v: %v", event.ID(), err)
				}
			case CloudStorageSourceProbeDeleteSubject:
				channelID = event.ID() + "-" + event.Subject()
				receiverChannel, err = receivedEvents.createReceiverChannel(channelID)
				if err != nil {
					return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
				}
				defer receivedEvents.deleteReceiverChannel(channelID)

				// Deleting a specific version of an object deletes it forever.
				objectAttrs, err := obj.Attrs(ctx)
				if err != nil {
					return logNACK(ctx, "Probe forwarding failed, error getting attributes for object %v: %v", event.ID(), err)
				}
				if err := obj.Generation(objectAttrs.Generation).Delete(ctx); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error deleting object %v: %v", event.ID(), err)
				}
			default:
				return logNACK(ctx, "Probe forwarding failed, unrecognized cloud storage probe subject: %v", event.Subject())
			}
		case CloudSchedulerSourceProbeEventType:
			// Fail if the delay since the last scheduled tick is greater than the desired period.
			if delay := time.Now().Sub(ph.lastCloudSchedulerEventTimestamp.getTime()); delay > ph.cloudSchedulerSourcePeriod {
				return logNACK(ctx, "Probe failed, delay between CloudSchedulerSource ticks exceeds period: %s > %s", delay, ph.cloudSchedulerSourcePeriod)
			}
			return cloudevents.ResultACK
		default:
			return logNACK(ctx, "Probe forwarding failed, unrecognized event type, %v", event.Type())
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
		logging.FromContext(ctx).Infof("Received event: %+v \n", event)
		ph.healthChecker.lastReceiverEventTimestamp.setNow()

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
		case schemasv1.CloudSchedulerJobExecutedEventType:
			// Refresh the last received event timestamp from the CloudSchedulerSource.
			ph.lastCloudSchedulerEventTimestamp.setNow()
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

	cloudSchedulerSourcePeriod       time.Duration
	lastCloudSchedulerEventTimestamp eventTimestamp

	probePort    int
	receiverPort int

	timeoutDuration time.Duration

	healthChecker *healthChecker
}

func (ph *ProbeHelper) run(ctx context.Context) {
	logger := logging.FromContext(ctx)

	// initialize the cloud scheduler event timestamp
	ph.lastCloudSchedulerEventTimestamp.setNow()

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
	go sc.StartReceiver(ctx, ph.forwardFromProbe(ctx, sc, psc, bkt, receivedEvents))

	// Receive the event and return the result back to the probe
	logger.Info("Starting event receiver...")
	rc.StartReceiver(ctx, ph.receiveEvent(ctx, receivedEvents))
}
