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
The Probe Helper implements the logic of a container image that converts asynchronous event delivery
to a synchronous call. An external probe can call this to get e2e event delivery success rate and latency.
Concretely, the ProbeHelper is able to send and receive events. It exposes an HTTP endpoints which forwards
probe requests to the broker, and waits for the event to be delivered back to the it,
and returns the e2e latency as the response.
                                                      4. (event)
                                   ----------------------------------------------------
                                  |                                                   |
                                  v                                                   |
	Probe ---(event)-----> ProbeHelper ----(event)-----> Broker ------> trigger ------
               1.                           2.             3. (blackbox)
*/

package main

func main() {
	runProbeHelper()
}
