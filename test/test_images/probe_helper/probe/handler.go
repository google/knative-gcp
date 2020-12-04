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

package probe

import (
	"context"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

/*

The following steps should be taken when implementing a new type of probe.

1. Define a static probe handler which implements the Handler interface.
2. Declare a singleton Handler object of this kind in the probe helper initializerHandlers method.
3. Add the mapping between forward probe type and the object from step 2 to the forwardProbeHandlers map in initializerHandlers.
4. Add the mapping between receiver probe type and the object from step 2 to the receiveProbeHandlers map in initializerHandlers.

*/

// Handler is the interface which static probe objects should implement.
type Handler interface {
	// Forward is the handler function which is called for probe requests which come
	// from PROBE_PORT.
	Forward(context.Context, cloudevents.Event) error
	// Receive is the handler function which is called for probe requests which come
	// from RECEIVER_PORT.
	Receive(context.Context, cloudevents.Event) error
}

func channelID(prefix, eventID string) string {
	return fmt.Sprintf("%s/%s", prefix, eventID)
}
