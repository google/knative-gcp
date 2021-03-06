/*
Copyright 2019 Google LLC

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

package converters

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/event"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
	schemasv1 "github.com/google/knative-gcp/pkg/schemas/v1"
)

func convertPubSubPull(ctx context.Context, msg *pubsub.Message) (*cev2.Event, error) {
	event := cev2.NewEvent(cev2.VersionV1)
	event.SetID(msg.ID)
	event.SetTime(msg.PublishTime)

	project, err := GetProjectKey(ctx)
	if err != nil {
		return nil, err
	}
	topic, err := GetTopicKey(ctx)
	if err != nil {
		return nil, err
	}

	event.SetSource(schemasv1.CloudPubSubEventSource(project, topic))
	event.SetType(schemasv1.CloudPubSubMessagePublishedEventType)

	// We promote attributes to extensions. If there is at least one attribute that cannot be promoted, we fail.
	if msg.Attributes != nil && len(msg.Attributes) > 0 {
		for k, v := range msg.Attributes {
			// CloudEvents v1.0 attributes MUST consist of lower-case letters ('a' to 'z') or digits ('0' to '9') as per
			// the spec. It's not even possible for a conformant transport to allow non-base36 characters.
			// Note `SetExtension` will make it lowercase so only `IsAlphaNumeric` needs to be checked here.
			if !IsExtensionNameValid(k) {
				return nil, fmt.Errorf("cannot convert attribute %q to extension", k)
			}
			event.SetExtension(k, v)
		}
	}
	// We do not know the content type and we do not want to inspect the payload,
	// thus we set this generic one.
	if err := event.SetData("application/octet-stream", msg.Data); err != nil {
		return nil, err
	}
	return &event, nil
}
