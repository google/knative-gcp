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
	"fmt"
	"net"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	cepubsub "github.com/cloudevents/sdk-go/protocol/pubsub/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"

	"knative.dev/pkg/logging"

	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

type probeConstructor func(*ProbeHelper, cloudevents.Event) (ProbeInterface, error)

var forwardProbeConstructors = map[string]probeConstructor{
	"broker-e2e-delivery-probe":                BrokerE2EDeliveryForwardProbeConstructor,
	"cloudpubsubsource-probe":                  CloudPubSubSourceForwardProbeConstructor,
	"cloudstoragesource-probe-create":          CloudStorageSourceCreateForwardProbeConstructor,
	"cloudstoragesource-probe-update-metadata": CloudStorageSourceUpdateMetadataForwardProbeConstructor,
	"cloudstoragesource-probe-archive":         CloudStorageSourceArchiveForwardProbeConstructor,
	"cloudstoragesource-probe-delete":          CloudStorageSourceDeleteForwardProbeConstructor,
	"cloudauditlogssource-probe":               CloudAuditLogsSourceForwardProbeConstructor,
	"cloudschedulersource-probe":               CloudSchedulerSourceForwardProbeConstructor,
}

var receiveProbeConstructors = map[string]probeConstructor{
	"broker-e2e-delivery-probe":                          BrokerE2EDeliveryReceiveProbeConstructor,
	schemasv1.CloudPubSubMessagePublishedEventType:       CloudPubSubSourceReceiveProbeConstructor,
	schemasv1.CloudStorageObjectFinalizedEventType:       CloudStorageSourceReceiveProbeConstructor,
	schemasv1.CloudStorageObjectMetadataUpdatedEventType: CloudStorageSourceReceiveProbeConstructor,
	schemasv1.CloudStorageObjectArchivedEventType:        CloudStorageSourceReceiveProbeConstructor,
	schemasv1.CloudStorageObjectDeletedEventType:         CloudStorageSourceReceiveProbeConstructor,
	schemasv1.CloudAuditLogsLogWrittenEventType:          CloudAuditLogsSourceReceiveProbeConstructor,
	schemasv1.CloudSchedulerJobExecutedEventType:         CloudSchedulerSourceReceiveProbeConstructor,
}

func WithProbeTimeout(ctx context.Context, event cloudevents.Event, defaultTimeout time.Duration, maxTimeout time.Duration) (context.Context, context.CancelFunc) {
	timeout := defaultTimeout
	if _, ok := event.Extensions()[ProbeEventTimeoutExtension]; ok {
		customTimeoutExtension := fmt.Sprint(event.Extensions()[ProbeEventTimeoutExtension])
		if customTimeout, err := time.ParseDuration(customTimeoutExtension); err != nil {
			logging.FromContext(ctx).Warnw("Failed to parse custom timeout extension duration", zap.String("timeout", customTimeoutExtension), zap.Error(err))
		} else {
			timeout = customTimeout
		}
	}
	if timeout.Nanoseconds() > maxTimeout.Nanoseconds() {
		logging.FromContext(ctx).Warnw("Desired timeout exceeds the maximum, clamping to maximum value", zap.Duration("timeout", timeout), zap.Duration("maximumTimeout", maxTimeout))
		timeout = maxTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

func WithProbeEventLoggingContext(ctx context.Context, event cloudevents.Event) context.Context {
	logger := logging.FromContext(ctx)
	logger = logger.With(zap.Any("event", map[string]interface{}{
		"id":          event.ID(),
		"source":      event.Source(),
		"specversion": event.SpecVersion(),
		"type":        event.Type(),
		"extensions":  event.Extensions(),
	}))
	return logging.WithLogger(ctx, logger)
}

func (ph *ProbeHelper) probeObjectFromEvent(event cloudevents.Event, ctrs map[string]probeConstructor) (ProbeInterface, error) {
	ctr, ok := ctrs[event.Type()]
	if !ok {
		return nil, fmt.Errorf("unrecognized probe event type: %s", event.Type())
	}
	probe, err := ctr(ph, event)
	if err != nil {
		return nil, err
	}
	return probe, nil
}

func (ph *ProbeHelper) forwardFromProbe(ctx context.Context) cloudEventsFunc {
	return func(event cloudevents.Event) cloudevents.Result {
		// Attach important metadata about the event to the logging context.
		ctx := WithProbeEventLoggingContext(ctx, event)
		logging.FromContext(ctx).Infow("Received probe request")

		// Construct the probe object based on the event type
		pr, err := ph.probeObjectFromEvent(event, forwardProbeConstructors)
		if err != nil {
			logging.FromContext(ctx).Warnw("Probe forwarding failed", zap.Error(err))
			return cloudevents.ResultNACK
		}

		// Add timeout to the context
		ctx, cancel := WithProbeTimeout(ctx, event, ph.defaultTimeoutDuration, ph.maxTimeoutDuration)
		defer cancel()

		// Create the receiver channel if desired
		var receiverChannel chan bool
		if ShouldWaitOnReceiver(pr) {
			receiverChannel, err = ph.receivedEvents.createReceiverChannel(pr.ChannelID())
			if err != nil {
				logging.FromContext(ctx).Warnw("Probe forwarding failed, could not create receiver channel", zap.String("channelID", pr.ChannelID()), zap.Error(err))
				return cloudevents.ResultNACK
			}
			defer ph.receivedEvents.deleteReceiverChannel(pr.ChannelID())
		}

		// Forward the probe event
		if err := pr.Handle(ctx); err != nil {
			logging.FromContext(ctx).Warnw("Probe forwarding failed", zap.Error(err))
			return cloudevents.ResultNACK
		}

		// Wait on the receiver channel if desired
		if ShouldWaitOnReceiver(pr) {
			select {
			case <-receiverChannel:
				return cloudevents.ResultACK
			case <-ctx.Done():
				logging.FromContext(ctx).Warnw("Timed out on probe waiting for the receiver channel", zap.String("channelID", pr.ChannelID()))
				return cloudevents.ResultNACK
			}
		}
		return cloudevents.ResultACK
	}
}

func (ph *ProbeHelper) receiveEvent(ctx context.Context) cloudEventsFunc {
	return func(event cloudevents.Event) cloudevents.Result {
		// Attach important metadata about the event to the logging context.
		ctx := WithProbeEventLoggingContext(ctx, event)
		logging.FromContext(ctx).Infow("Received event")

		// Construct the probe object based on the event type
		pr, err := ph.probeObjectFromEvent(event, receiveProbeConstructors)
		if err != nil {
			logging.FromContext(ctx).Warnw("Probe receiver failed", zap.Error(err))
			return cloudevents.ResultNACK
		}

		// Close the associated receiver channel if desired
		if pr != nil && ShouldWaitOnReceiver(pr) {
			ph.receivedEvents.RLock()
			defer ph.receivedEvents.RUnlock()
			receiverChannel, ok := ph.receivedEvents.channels[pr.ChannelID()]
			if !ok {
				logging.FromContext(ctx).Warnw("This event is not received by the probe receiver client", zap.String("channelID", pr.ChannelID()))
				return cloudevents.ResultACK
			}
			receiverChannel <- true
		}
		return cloudevents.ResultACK
	}
}

type ProbeHelper struct {
	// The project ID
	projectID string

	// The base URL for the BrokerCell Ingress used in the broker delivery probe
	brokerCellIngressBaseURL string

	// The client responsible for handling forward probe requests
	ceForwardClient cloudevents.Client

	// The client responsible for receiving events
	ceReceiveClient cloudevents.Client

	// The pubsub client wrapped by a CloudEvents client for the CloudPubSubSource
	// probe and used for the CloudAuditLogsSource probe
	pubsubClient *pubsub.Client

	// The CloudEvents client responsible for forwarding events as messages to a
	// topic for the CloudPubSubSource probe.
	cePubsubClient cloudevents.Client

	// The storage client used in the CloudStorageSource
	storageClient *storage.Client

	// Handle for the bucket used in the CloudStorageSource probe
	bucket *storage.BucketHandle

	// Timestamp of the last observed tick from the CloudSchedulerSource
	lastSchedulerEventTimestamp *eventTimestamp

	// The port through which the probe helper receives probe requests
	probePort int
	// If a listener is specified instead, the port is ignored
	probeListener net.Listener

	// The port through which the probe helper receives source events
	receiverPort int
	// If a listener is specified instead, the port is ignored
	receiverListener net.Listener

	// The default duration after which the probe helper times out after forwarding an event, if no custom timeout duration is specified
	defaultTimeoutDuration time.Duration

	// The maximum duration after which the probe helper times out after forwarding an event
	maxTimeoutDuration time.Duration

	// The map of received events to be tracked by the probe and receiver clients
	receivedEvents *receivedEventsMap

	// The probe checker invoked in the liveness probe
	probeChecker *probeChecker
}

func (ph *ProbeHelper) run(ctx context.Context) {
	var err error
	logger := logging.FromContext(ctx)

	// initialize the cloud scheduler event timestamp
	ph.lastSchedulerEventTimestamp = &eventTimestamp{}
	ph.lastSchedulerEventTimestamp.setNow()

	// initialize the probe checker
	if ph.probeChecker == nil {
		logger.Fatalw("Unspecified probe checker")
	}
	ph.probeChecker.lastProbeEventTimestamp.setNow()
	ph.probeChecker.lastReceiverEventTimestamp.setNow()

	// create pubsub client
	if ph.pubsubClient == nil {
		ph.pubsubClient, err = pubsub.NewClient(ctx, ph.projectID)
		if err != nil {
			logger.Fatalw("Failed to create cloud pubsub client", zap.Error(err))
		}
	}
	if ph.cePubsubClient == nil {
		pst, err := cepubsub.New(ctx,
			cepubsub.WithClient(ph.pubsubClient),
			cepubsub.WithProjectID(ph.projectID))
		if err != nil {
			logger.Fatalw("Failed to create pubsub transport", zap.Error(err))
		}
		ph.cePubsubClient, err = cloudevents.NewClient(pst)
		if err != nil {
			logger.Fatalw("Failed to create CloudEvents pubsub client", zap.Error(err))
		}
	}

	// create cloud storage client
	if ph.storageClient == nil {
		ph.storageClient, err = storage.NewClient(ctx)
		if err != nil {
			logger.Fatalw("Failed to create cloud storage client", zap.Error(err))
		}
	}

	// create sender client
	if ph.ceForwardClient == nil {
		spOpts := []cehttp.Option{}
		if ph.probeListener != nil {
			spOpts = append(spOpts, cloudevents.WithListener(ph.probeListener))
		} else {
			spOpts = append(spOpts, cloudevents.WithPort(ph.probePort))
		}
		sp, err := cloudevents.NewHTTP(spOpts...)
		if err != nil {
			logger.Fatalw("Failed to create sender transport", zap.Error(err))
		}
		ph.ceForwardClient, err = cloudevents.NewClient(sp)
		if err != nil {
			logger.Fatalw("Failed to create sender client", zap.Error(err))
		}
	}

	// create receiver client
	if ph.ceReceiveClient == nil {
		rpOpts := []cehttp.Option{
			cloudevents.WithGetHandlerFunc(ph.probeChecker.stalenessHandlerFunc(ctx)),
			cloudevents.WithMiddleware(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					req.Header.Set(ProbeEventRequestHostHeader, req.Host)
					next.ServeHTTP(rw, req)
				})
			}),
		}
		if ph.probeListener != nil {
			rpOpts = append(rpOpts, cloudevents.WithListener(ph.receiverListener))
		} else {
			rpOpts = append(rpOpts, cloudevents.WithPort(ph.receiverPort))
		}
		rp, err := cloudevents.NewHTTP(rpOpts...)
		if err != nil {
			logger.Fatalw("Failed to create receiver transport", zap.Error(err))
		}
		ph.ceReceiveClient, err = cloudevents.NewClient(rp)
		if err != nil {
			logger.Fatalw("Failed to create receiver client", zap.Error(err))
		}
	}

	// make a map to store the channel for each event
	if ph.receivedEvents == nil {
		ph.receivedEvents = &receivedEventsMap{
			channels: make(map[string]chan bool),
		}
	}

	// start a goroutine to receive the event from probe and forward it appropriately
	logger.Infow("Starting event forwarder client...")
	go ph.ceForwardClient.StartReceiver(ctx, ph.forwardFromProbe(ctx))

	// Receive the event and return the result back to the probe
	logger.Infow("Starting event receiver client...")
	ph.ceReceiveClient.StartReceiver(ctx, ph.receiveEvent(ctx))
}
