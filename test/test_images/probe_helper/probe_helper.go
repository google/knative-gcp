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
	"net"
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
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
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
	CloudSchedulerSourceProbeEventType           = "cloudschedulersource-probe"
)

type healthChecker struct {
	lastProbeEventTimestamp    eventTimestamp
	lastReceiverEventTimestamp eventTimestamp
	maxStaleDuration           time.Duration

	// The port through which the health checker accepts liveness probe requests
	port int
	// If a listener is specified instead, the port is ignored
	listener net.Listener
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

	if c.listener == nil {
		var err error
		c.listener, err = net.Listen("tcp", ":"+strconv.Itoa(c.port))
		if err != nil {
			logging.FromContext(ctx).Error("Failed to get probe helper health checker listener", zap.Error(err))
		}
	}
	srv := &nethttp.Server{
		Handler: c,
	}
	go func() {
		logging.FromContext(ctx).Info("Starting the probe helper health checker...")
		if err := srv.Serve(c.listener); err != nil && err != nethttp.ErrServerClosed {
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
			eventSubject == CloudAuditLogsSourceProbeDeleteSubject)) ||
		eventType == CloudSchedulerSourceProbeEventType)
}

func (ph *ProbeHelper) forwardFromProbe(ctx context.Context) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var channelID string
		var err error
		var receiverChannel chan bool
		logging.FromContext(ctx).Info("Received probe request", zap.Any("event", event))
		ph.healthChecker.lastProbeEventTimestamp.setNow()

		if !isValidProbeEvent(event) {
			return logNACK(ctx, "Probe forwarding failed, unrecognized probe event type and subject: type=%v, subject=%v", event.Type(), event.Subject())
		}

		// The CloudSchedulerSource probe is not channel-based.
		if event.Type() != CloudSchedulerSourceProbeEventType {
			// Create channel on which to wait for event to be received once forwarded.
			channelID = event.ID()
			if event.Subject() != "" {
				channelID = channelID + "-" + event.Subject()
			}
			receiverChannel, err = ph.receivedEvents.createReceiverChannel(channelID)
			if err != nil {
				return logNACK(ctx, "Probe forwarding failed, could not create receiver channel: %v", err)
			}
			defer ph.receivedEvents.deleteReceiverChannel(channelID)
		}

		ctx, cancel := context.WithTimeout(ctx, ph.timeoutDuration)
		defer cancel()

		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			// The probe client forwards the event to the broker.
			if res := ph.probeClient.Send(ctx, event); !cloudevents.IsACK(res) {
				return logNACK(ctx, "Error when sending event %v to broker: %+v \n", event.ID(), res)
			}
		case CloudPubSubSourceProbeEventType:
			// The pubsub client forwards the event as a message to a pubsub topic.
			if res := ph.cePubsubClient.Send(ctx, event); !cloudevents.IsACK(res) {
				return logNACK(ctx, "Error when publishing event %v to pubsub topic: %+v \n", event.ID(), res)
			}
		case CloudStorageSourceProbeEventType:
			obj := ph.bucket.Object(event.ID())
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
				if _, err := ph.pubsubClient.CreateTopic(ctx, event.ID()); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error creating pubsub topic %v: %v", event.ID(), err)
				}
			case CloudAuditLogsSourceProbeDeleteSubject:
				// Delete the pubsub topic with the given ID.
				if err := ph.pubsubClient.Topic(event.ID()).Delete(ctx); err != nil {
					return logNACK(ctx, "Probe forwarding failed, error deleting pubsub topic %v: %v", event.ID(), err)
				}
			}
		case CloudSchedulerSourceProbeEventType:
			// Fail if the delay since the last scheduled tick is greater than the desired period.
			if delay := time.Now().Sub(ph.lastCloudSchedulerEventTimestamp.getTime()); delay > ph.cloudSchedulerSourcePeriod {
				return logNACK(ctx, "Probe failed, delay between CloudSchedulerSource ticks exceeds period: %s > %s", delay, ph.cloudSchedulerSourcePeriod)
			}
			return cloudevents.ResultACK
		}

		select {
		case <-receiverChannel:
			return cloudevents.ResultACK
		case <-ctx.Done():
			return logNACK(ctx, "Timed out waiting for the receiver channel: %v \n", channelID)
		}
	}
}

func (ph *ProbeHelper) receiveEvent(ctx context.Context) cloudEventsFunc {
	return func(event cloudevents.Event) protocol.Result {
		var channelID string
		logging.FromContext(ctx).Info("Received event", zap.Any("event", event))
		ph.healthChecker.lastReceiverEventTimestamp.setNow()

		switch event.Type() {
		case BrokerE2EDeliveryProbeEventType:
			// The event is received as sent.
			//
			// Example:
			//   Context Attributes,
			//     specversion: 1.0
			//     type: broker-e2e-delivery-probe
			//     source: probe
			//     id: broker-e2e-delivery-probe-5950a1f1-f128-4c8e-bcdd-a2c96b4e5b78
			//     time: 2020-09-14T18:49:01.455945852Z
			//     datacontenttype: application/json
			//   Extensions,
			//     knativearrivaltime: 2020-09-14T18:49:01.455947424Z
			//     traceparent: 00-82b13494f5bcddc7b3007a7cd7668267-64e23f1193ceb1b7-00
			//   Data,
			//     { ... }
			channelID = event.ID()
		case schemasv1.CloudPubSubMessagePublishedEventType:
			// The original event is wrapped into a pubsub Message by the CloudEvents
			// pubsub sender client, and encoded as data in a CloudEvent by the CloudPubSubSource.
			//
			// Example:
			//   Context Attributes,
			//     specversion: 1.0
			//     type: google.cloud.pubsub.topic.v1.messagePublished
			//     source: //pubsub.googleapis.com/projects/project-id/topics/cloudpubsubsource-topic
			//     id: 1529309436535525
			//     time: 2020-09-14T17:06:46.363Z
			//     datacontenttype: application/json
			//   Data,
			//     {
			//       "subscription": "cre-src_cloud-run-events-probe_cloudpubsubsource_02f88763-1df6-4944-883f-010ebac27dd2",
			//       "message": {
			//         "messageId": "1529309436535525",
			//         "data": "eydtc2cnOidQcm9iZSBDbG91ZCBSdW4gRXZlbnRzISd9",
			//         "attributes": {
			//           "Content-Type": "application/json",
			//           "ce-id": "cloudpubsubsource-probe-294119a9-98e2-44ec-a2b2-28a98cf40eee",
			//           "ce-source": "probe",
			//           "ce-specversion": "1.0",
			//           "ce-type": "cloudpubsubsource-probe"
			//         },
			//         "publishTime": "2020-09-14T17:06:46.363Z"
			//       }
			//     }
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
			//
			// Example:
			//   Context Attributes,
			//     specversion: 1.0
			//     type: google.cloud.storage.object.v1.finalized
			//     source: //storage.googleapis.com/projects/_/buckets/cloudstoragesource-bucket
			//     subject: objects/cloudstoragesource-probe-fc2638d1-fcae-4889-9fa1-14a08cb05fc4
			//     id: 1529343217463053
			//     time: 2020-09-14T17:18:40.984Z
			//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
			//     datacontenttype: application/json
			//   Data,
			//     { ... }
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeCreateSubject
		case schemasv1.CloudStorageObjectMetadataUpdatedEventType:
			// The original event is written as an identifiable object to a bucket.
			//
			// Example:
			//   Context Attributes,
			//     specversion: 1.0
			//     type: google.cloud.storage.object.v1.metadataUpdated
			//     source: //storage.googleapis.com/projects/_/buckets/cloudstoragesource-bucket
			//     subject: objects/cloudstoragesource-probe-fc2638d1-fcae-4889-9fa1-14a08cb05fc4
			//     id: 1529343267626759
			//     time: 2020-09-14T17:18:42.296Z
			//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
			//     datacontenttype: application/json
			//   Data,
			//     { ... }
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeUpdateMetadataSubject
		case schemasv1.CloudStorageObjectArchivedEventType:
			// The original event is written as an identifiable object to a bucket.
			//
			// Example:
			//   Context Attributes,
			//     specversion: 1.0
			//     type: google.cloud.storage.object.v1.archived
			//     source: //storage.googleapis.com/projects/_/buckets/cloudstoragesource-bucket
			//     subject: objects/cloudstoragesource-probe-fc2638d1-fcae-4889-9fa1-14a08cb05fc4
			//     id: 1529346856916356
			//     time: 2020-09-14T17:18:43.872Z
			//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
			//     datacontenttype: application/json
			//   Data,
			//     { ... }
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeArchiveSubject
		case schemasv1.CloudStorageObjectDeletedEventType:
			// The original event is written as an identifiable object to a bucket.
			//
			// Example:
			//   Context Attributes,
			//     specversion: 1.0
			//     type: google.cloud.storage.object.v1.deleted
			//     source: //storage.googleapis.com/projects/_/buckets/cloudstoragesource-bucket
			//     subject: objects/cloudstoragesource-probe-fc2638d1-fcae-4889-9fa1-14a08cb05fc4
			//     id: 1529347481207133
			//     time: 2020-09-14T17:18:45.146Z
			//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/storage/v1/data.proto
			//     datacontenttype: application/json
			//   Data,
			//     { ... }
			if _, err := fmt.Sscanf(event.Subject(), "objects/%s", &channelID); err != nil {
				return logACK(ctx, "Failed to extract event ID from object name: %v", err)
			}
			channelID = channelID + "-" + CloudStorageSourceProbeDeleteSubject
		case schemasv1.CloudAuditLogsLogWrittenEventType:
			// The logged event type is held in the methodname extension. For creation
			// and deletion of pubsub topics, the topic ID can be extracted from the
			// event subject.
			if _, ok := event.Extensions()["methodname"]; !ok {
				return logACK(ctx, "CloudEvent does not have extension methodname: %v", event)
			}
			sepSub := strings.Split(event.Subject(), "/")
			if len(sepSub) != 5 || sepSub[0] != "pubsub.googleapis.com" || sepSub[1] != "projects" || sepSub[2] != ph.projectID || sepSub[3] != "topics" {
				return logACK(ctx, "Unexpected Cloud AuditLogs event subject: %v", event.Subject())
			}
			switch event.Extensions()["methodname"] {
			case "google.pubsub.v1.Publisher.CreateTopic":
				// Example:
				//   Context Attributes,
				//     specversion: 1.0
				//     type: google.cloud.audit.log.v1.written
				//     source: //cloudaudit.googleapis.com/projects/project-id/logs/activity
				//     subject: pubsub.googleapis.com/projects/project-id/topics/cloudauditlogssource-probe-914e5946-5e27-4bde-a455-7cfbae1c8539
				//     id: d2ad1359483fc13c8056c430545fd217
				//     time: 2020-09-14T18:44:18.636961725Z
				//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/audit/v1/data.proto
				//     datacontenttype: application/json
				//   Extensions,
				//     methodname: google.pubsub.v1.Publisher.CreateTopic
				//     resourcename: projects/project-id/topics/cloudauditlogssource-probe-914e5946-5e27-4bde-a455-7cfbae1c8539
				//     servicename: pubsub.googleapis.com
				//   Data,
				//     { ... }
				channelID = sepSub[4] + "-" + CloudAuditLogsSourceProbeCreateSubject
			case "google.pubsub.v1.Publisher.DeleteTopic":
				// Example:
				//   Context Attributes,
				//     specversion: 1.0
				//     type: google.cloud.audit.log.v1.written
				//     source: //cloudaudit.googleapis.com/projects/project-id/logs/activity
				//     subject: pubsub.googleapis.com/projects/project-id/topics/cloudauditlogssource-probe-914e5946-5e27-4bde-a455-7cfbae1c8539
				//     id: 5e3ecfb9fa807b4f0beb8844a0c31b65
				//     time: 2020-09-14T18:44:21.941097939Z
				//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/audit/v1/data.proto
				//     datacontenttype: application/json
				//   Extensions,
				//     methodname: google.pubsub.v1.Publisher.DeleteTopic
				//     resourcename: projects/project-id/topics/cloudauditlogssource-probe-914e5946-5e27-4bde-a455-7cfbae1c8539
				//     servicename: pubsub.googleapis.com
				//   Data,
				//     { ... }
				channelID = sepSub[4] + "-" + CloudAuditLogsSourceProbeDeleteSubject
			default:
				return logACK(ctx, "Unrecognized CloudEvent methodname extension: %v", event.Extensions()["methodname"])
			}
		case schemasv1.CloudSchedulerJobExecutedEventType:
			// Refresh the last received event timestamp from the CloudSchedulerSource.
			//
			// Example:
			//   Context Attributes,
			//     specversion: 1.0
			//     type: google.cloud.scheduler.job.v1.executed
			//     source: //cloudscheduler.googleapis.com/projects/project-id/locations/location/jobs/cre-scheduler-9af24c86-8ba9-4688-80d0-e527678a6a63
			//     id: 1533039115503825
			//     time: 2020-09-15T20:12:00.14Z
			//     dataschema: https://raw.githubusercontent.com/googleapis/google-cloudevents/master/proto/google/events/cloud/scheduler/v1/data.proto
			//     datacontenttype: application/json
			//   Data,
			//     { ... }
			ph.lastCloudSchedulerEventTimestamp.setNow()
		default:
			return logACK(ctx, "Unrecognized event type: %v", event.Type())
		}

		ph.receivedEvents.RLock()
		defer ph.receivedEvents.RUnlock()
		receiver, ok := ph.receivedEvents.channels[channelID]
		if !ok {
			return logACK(ctx, "This event is not received by the probe receiver client: %v \n", channelID)
		}
		receiver <- true
		return cloudevents.ResultACK
	}
}

type ProbeHelper struct {
	// The project ID
	projectID string

	// The URL endpoint for the Broker ingress
	brokerURL string

	// The client responsible for handling probe requests and forwarding events to the Broker
	probeClient cloudevents.Client

	// The client responsible for receiving events from sources
	receiverClient cloudevents.Client

	// The topic ID used in the CloudPubSubSource
	cloudPubSubSourceTopicID string

	// The pubsub client wrapped by a CloudEvents client for the CloudPubSubSource
	// probe and used natively for the CloudAuditLogsSource probe
	pubsubClient *pubsub.Client

	// The CloudEvents client responsible for forwarding events as messages to a
	// topic for the CloudPubSubSource probe.
	cePubsubClient cloudevents.Client

	// The bucket ID used in the CloudStorageSource
	cloudStorageSourceBucketID string

	// The storage client used in the CloudStorageSource
	storageClient *storage.Client

	// Handle for the bucket used in the CloudStorageSource probe
	bucket *storage.BucketHandle

	// The tolerated period between observed CloudSchedulerSource tickets
	cloudSchedulerSourcePeriod time.Duration

	// Timestamp of the last observed tick from the CloudSchedulerSource
	lastCloudSchedulerEventTimestamp eventTimestamp

	// The port through which the probe helper receives probe requests
	probePort int
	// If a listener is specified instead, the port is ignored
	probeListener net.Listener

	// The port through which the probe helper receives source events
	receiverPort int
	// If a listener is specified instead, the port is ignored
	receiverListener net.Listener

	// The duration after which the probe helper times out after forwarding an event
	timeoutDuration time.Duration

	// The map of received events to be tracked by the probe and receiver clients
	receivedEvents *receivedEventsMap

	// The health checker invoked in the liveness probe
	healthChecker *healthChecker
}

func (ph *ProbeHelper) run(ctx context.Context) {
	var err error
	logger := logging.FromContext(ctx)

	// initialize the cloud scheduler event timestamp
	ph.lastCloudSchedulerEventTimestamp.setNow()

	// create pubsub client
	if ph.pubsubClient == nil {
		ph.pubsubClient, err = pubsub.NewClient(ctx, ph.projectID)
		if err != nil {
			logger.Fatal("Failed to create cloud pubsub client", zap.Error(err))
		}
	}
	if ph.cePubsubClient == nil {
		pst, err := cepubsub.New(ctx,
			cepubsub.WithClient(ph.pubsubClient),
			cepubsub.WithProjectID(ph.projectID),
			cepubsub.WithTopicID(ph.cloudPubSubSourceTopicID))
		if err != nil {
			logger.Fatal("Failed to create pubsub transport", zap.Error(err))
		}
		ph.cePubsubClient, err = cloudevents.NewClient(pst)
		if err != nil {
			logger.Fatal("Failed to create CloudEvents pubsub client", zap.Error(err))
		}
	}

	// create cloud storage client
	if ph.storageClient == nil {
		ph.storageClient, err = storage.NewClient(ctx)
		if err != nil {
			logger.Fatal("Failed to create cloud storage client", zap.Error(err))
		}
	}
	if ph.bucket == nil {
		ph.bucket = ph.storageClient.Bucket(ph.cloudStorageSourceBucketID)
	}

	// create sender client
	if ph.probeClient == nil {
		spOpts := []cehttp.Option{cloudevents.WithTarget(ph.brokerURL)}
		if ph.probeListener != nil {
			spOpts = append(spOpts, cloudevents.WithListener(ph.probeListener))
		} else {
			spOpts = append(spOpts, cloudevents.WithPort(ph.probePort))
		}
		sp, err := cloudevents.NewHTTP(spOpts...)
		if err != nil {
			logger.Fatal("Failed to create sender transport", zap.Error(err))
		}
		ph.probeClient, err = cloudevents.NewClient(sp)
		if err != nil {
			logger.Fatal("Failed to create sender client", zap.Error(err))
		}
	}

	// create receiver client
	if ph.receiverClient == nil {
		rpOpts := []cehttp.Option{}
		if ph.probeListener != nil {
			rpOpts = append(rpOpts, cloudevents.WithListener(ph.receiverListener))
		} else {
			rpOpts = append(rpOpts, cloudevents.WithPort(ph.receiverPort))
		}
		rp, err := cloudevents.NewHTTP(rpOpts...)
		if err != nil {
			logger.Fatal("Failed to create receiver transport", zap.Error(err))
		}
		ph.receiverClient, err = cloudevents.NewClient(rp)
		if err != nil {
			logger.Fatal("Failed to create receiver client", zap.Error(err))
		}
	}

	// start the health checker
	if ph.healthChecker == nil {
		logger.Fatal("Unspecified health checker")
	}
	go ph.healthChecker.start(ctx)

	// make a map to store the channel for each event
	if ph.receivedEvents == nil {
		ph.receivedEvents = &receivedEventsMap{
			channels: make(map[string]chan bool),
		}
	}

	// start a goroutine to receive the event from probe and forward it appropriately
	logger.Info("Starting Probe Helper server...")
	go ph.probeClient.StartReceiver(ctx, ph.forwardFromProbe(ctx))

	// Receive the event and return the result back to the probe
	logger.Info("Starting event receiver...")
	ph.receiverClient.StartReceiver(ctx, ph.receiveEvent(ctx))
}
