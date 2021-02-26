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
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"

	pkgutils "github.com/google/knative-gcp/pkg/utils"
	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/knative-gcp/test/test_images/probe_helper/probe"
)

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
		 the object, and waits to be notified that the object has been deleted by a
		 CloudStorageSource.

4. CloudSchedulerSource Probe

		This probe is unlike the others in that it does not measure e2e delivery
		by sending and receiving uniquely identifiable events. Instead, it depends
		on an existing CloudSchedulerSource which sinks an event to the Probe Helper
		receiver every minute.

		The Probe Helper receives an event of type `cloudschedulersource-probe`, and
		compares the delay between the current time and the last observed Cloud Scheduler
		tick. The probe fails if the delay exceeds a threshold.

5. CloudAuditLogsSource Probe

	The Probe Helper receives an event, creates a Pub/Sub topic named after it,
	and waits to observe its creation having been logged by a CloudAuditLogsSource.

6. PingSource Probe

	This is similar to the CloudSchedulerSource Probe.

	The Probe Helper receives an event of type `pingsource-probe`, and
	compares the delay between the current time and the last observed PingSource
	tick. The probe fails if the delay exceeds a threshold.

*/

type envConfig struct {
	probe.EnvConfig
	// Environment variable containing the port which listens to the probe to forward events
	ProbePort probe.ForwardPort `envconfig:"PROBE_PORT" default:"8070"`
	// Environment variable containing the port to receive delivered events
	ReceiverPort probe.ReceivePort `envconfig:"RECEIVER_PORT" default:"8080"`
	// Environment variable containing the base URL for the brokercell ingress, used in the broker e2e delivery probe
	BrokerCellIngressBaseURL string `envconfig:"BROKER_CELL_INGRESS_BASE_URL" default:"http://default-brokercell-ingress.cloud-run-events.svc.cluster.local"`
	// Environment variable containing the maximum tolerated staleness duration for Cloud Scheduler job / PingSource ticks before they are discarded
	CronStaleDuration time.Duration `envconfig:"CRON_STALE_DURATION" default:"3m"`
}

func main() {
	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		panic(fmt.Sprintf("Failed to process env var: %s", err))
	}

	// Create the logger and attach it to the context
	loggingConfig, err := logging.NewConfigFromMap(map[string]string{
		"loglevel.probe-helper": "debug",
	})
	if err != nil {
		// If this fails, there is no recovering.
		panic(err)
	}
	logger, _ := logging.NewLoggerFromConfig(loggingConfig, "probe-helper")
	ctx := logging.WithLogger(signals.NewContext(), logger)

	// Get the default project ID
	projectID, err := pkgutils.ProjectIDOrDefault("")
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to get the default project ID", zap.Error(err))
	}

	ph, err := InitializeProbeHelper(ctx, env.BrokerCellIngressBaseURL, clients.ProjectID(projectID), env.CronStaleDuration, env.EnvConfig, env.ProbePort, env.ReceiverPort)
	if err != nil {
		logging.FromContext(ctx).Fatal("Failed to initialize probe helper", zap.Error(err))
	}
	ph.Run(ctx)
}
