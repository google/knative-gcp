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

*/

package main

import (
	"knative.dev/pkg/signals"
)

func main() {
	runProbeHelper(signals.NewContext(), nil, nil, nil, nil)
}
