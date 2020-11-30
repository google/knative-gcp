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
)

/*

The following steps should be taken when implementing a new type of probe.

1. Define a forward probe handler struct which implements the Handler interface,
  and define a constructor for that probe Handler object.
2. Define a receiver probe handler struct which implements the Handler interface,
	and define a constructor for that probe Handler object.
3. Ensure that the ChannelID for paired forward/receive probe Handlers are equal.
4. Add the mapping between forward probe type and forward Handler constructor to
	the forwardProbeConstructors map in probe/helper.go
5. Add the mapping between receiver probe type and receiver Handler constructor to
  the receiveProbeConstructors map in probe/helper.go

*/

// Handler is the interface which probe objects should implement.
type Handler interface {
	// ChannelID represents the key of the receiver channel which the probe helper
	// should wait on after forwarding a probe request. You should ensure that
	// each distinct probe request maps to a unique ChannelID, and that when a
	// forward probe is matched with a receiver probe, that both handlers produce
	// the same ChannelID.
	ChannelID() string

	// Handle is the handler function which is called for probe forwarders before
	// waiting on the receiver channel. Typically, Handle should forward an event
	// to some other service.
	Handle(context.Context) error
}

// ShouldWaitOnReceiver determines if the given probe handler should wait on a
// receiver channel. By convention, whenever the probe's channel ID is empty, we
// do not wait on a receiver channel.
func ShouldWaitOnReceiver(p Handler) bool {
	return p.ChannelID() != ""
}
