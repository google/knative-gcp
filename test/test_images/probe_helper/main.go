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

/*

The Probe Helper implements the logic of a container image that converts
asynchronous event delivery to a synchronous call. An external probe can call
this to get e2e event delivery success rate and latency. Concretely, the Probe
Helper is able to send and receive events. It exposes an HTTP endpoint which
forwards probe requests to different destinations, and waits for events to be
delivered back to it.

The Probe Helper can handle multiple different types of probes.

1. Broker E2E Delivery Probe

	The Probe Helper receives an event, forwards it to a Broker, and waits for it
	to be delivered back.

                                                      4. (event)
                                   ----------------------------------------------------
                                  |                                                   |
                                  v                                                   |
	Probe ---(event)-----> ProbeHelper ----(event)-----> Broker ------> trigger ------
							 1.                           2.             3. (blackbox)

2. CloudPubSubSource Probe

	The Probe Helper receives an event, publishes it as a message to a Cloud
	Pub/Sub topic, and waits for it to be delivered back wrapped in a CloudEvent
	from a CloudPubSubSource.

3. CloudStorageSource Probe

	This probe involves multiple steps executed in sequence which are intended to
	test all of the different Cloud Storage events which the CloudStorageSource is
	triggered on.

	1. The Probe Helper receives an event with a given ID, writes an object
		 named with that ID to a Cloud Storage bucket, and waits to be notified of
		 the object having been finalized by a CloudStorageSource.
	2. The Probe Helper receives an event with the same ID as in step 1, modifies
		 the same object's metadata, and waits to be notified of the object's
		 metadata having been updated by a CloudStorageSource.
	3. The Probe Helper receives an event with the same ID as in step 1, archives
		 the object, and waits to be notified that the object has been archived by a
		 CloudStorageSource.
	4. The Probe Helper receives an event with the same ID as in step 1, deletes
		 the object, and waits to tbe notified that the object has been deleted by a
		 CloudStorageSource.

4. CloudSchedulerSource Probe

		This probe is unlike the others in that it does not measure e2e delivery
		by sending and receiving uniquely identifiable events. Instead, it depends
		on an existing CloudSchedulerSource which sinks an event to the Probe Helper
		receiver every minute.

		The Probe Helper receives an event of type `cloudschedulersource-probe`,
		and examines the time since the last tick observed from the CloudSchedulerSource.
		If this duration is greater than 1 minute, the probe fails, and otherwise,
		the probe succeeds.

5. CloudAuditLogsSource Probe

	The Probe Helper receives an event, creates a Pub/Sub topic named after it,
	and waits to observe its creation having been logged by a CloudAuditLogsSource.

*/

package main

import (
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"

	metadataClient "github.com/google/knative-gcp/pkg/gclient/metadata"
	"github.com/google/knative-gcp/pkg/utils"
)

type envConfig struct {
	// Environment variable containing the project ID
	ProjectID string `envconfig:"PROJECT_ID"`

	// Environment variable containing the sink URL (broker URL) that the event will be forwarded to by the probeHelper for the e2e delivery probe
	BrokerURL string `envconfig:"K_SINK" default:"http://default-brokercell-ingress.cloud-run-events.svc.cluster.local/cloud-run-events-probe/default"`

	// Environment variable containing the CloudPubSubSource Topic ID that the event will be forwarded to by the probeHelper for the CloudPubSubSource probe
	CloudPubSubSourceTopicID string `envconfig:"CLOUDPUBSUBSOURCE_TOPIC_ID" default:"cloudpubsubsource-topic"`

	// Environment variable containing the CloudStorageSource Bucket ID that objects will be written to by the probeHelper for the CloudStorageSource probe
	CloudStorageSourceBucketID string `envconfig:"CLOUDSTORAGESOURCE_BUCKET_ID" default:"cloudstoragesource-bucket"`

	// Environment variable containing an upper bound on the duration between events emitted by the CloudSchedulerSource
	CloudSchedulerSourcePeriod time.Duration `envconfig:"CLOUDSCHEDULERSOURCE_PERIOD" default:"90s"`

	// Environment variable containing the port which listens to the probe to deliver the event
	ProbePort int `envconfig:"PROBE_PORT" default:"8070"`

	// Environment variable containing the port to receive the event from the trigger
	ReceiverPort int `envconfig:"RECEIVER_PORT" default:"8080"`

	// Environment variable containing the port to send health checks to
	HealthCheckerPort int `envconfig:"HEALTH_CHECKER_PORT" default:"8060"`

	// Environment variable containing the maximum tolerated staleness duration
	MaxStaleDuration time.Duration `envconfig:"MAX_STALE_DURATION" default:"5m"`

	// Environment variable containing the timeout duration to wait for an event to be delivered back
	TimeoutDuration time.Duration `envconfig:"TIMEOUT_DURATION" default:"30m"`
}

func main() {
	ctx := signals.NewContext()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		logging.FromContext(ctx).Fatal("Failed to process env var", zap.Error(err))
	}
	projectID, err := utils.ProjectID(env.ProjectID, metadataClient.NewDefaultMetadataClient())
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to get the default project ID", zap.Error(err))
	}
	ph := &ProbeHelper{
		projectID:                  projectID,
		brokerURL:                  env.BrokerURL,
		probePort:                  env.ProbePort,
		receiverPort:               env.ReceiverPort,
		cloudPubSubSourceTopicID:   env.CloudPubSubSourceTopicID,
		cloudStorageSourceBucketID: env.CloudStorageSourceBucketID,
		cloudSchedulerSourcePeriod: env.CloudSchedulerSourcePeriod,
		timeoutDuration:            env.TimeoutDuration,
		healthChecker: &healthChecker{
			port:             env.HealthCheckerPort,
			maxStaleDuration: env.MaxStaleDuration,
		},
	}
	ph.run(ctx)
}
