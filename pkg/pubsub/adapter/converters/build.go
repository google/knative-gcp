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
	"errors"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	. "github.com/cloudevents/sdk-go/v1/cloudevents"
	cepubsub "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/v1/cloudevents/transport/pubsub/context"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
)

const (
	CloudBuildConverter = "com.google.cloud.build"
	buildSchemaUrl      = "https://raw.githubusercontent.com/google/knative-gcp/master/schemas/build/schema.json"
)

func convertCloudBuild(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	tx := pubsubcontext.TransportContextFrom(ctx)
	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(tx.ID)
	event.SetTime(tx.PublishTime)
	// We do not know the content type and we do not want to inspect the payload,
	// thus we set this generic one.
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetType(v1alpha1.CloudBuildSourceEvent)
	event.SetDataSchema(buildSchemaUrl)

	// Set the source and subject if it comes as an attribute.
	if buildId, ok := msg.Attributes[v1alpha1.CloudBuildSourceBuildId]; !ok {
		return nil, errors.New("received event did not have buildId")
	} else {
		event.SetSource(v1alpha1.CloudBuildSourceEventSource(tx.Project, buildId))
	}
	if buildStatus, ok := msg.Attributes[v1alpha1.CloudBuildSourceBuildStatus]; !ok {
		return nil, errors.New("received event did not have build status")
	} else {
		event.SetSubject(buildStatus)
	}

	// Set the mode to be an extension attribute.
	event.SetExtension("knativecemode", string(sendMode))
	event.Data = msg.Data
	event.DataEncoded = true
	// Attributes are extensions.
	if msg.Attributes != nil && len(msg.Attributes) > 0 {
		for k, v := range msg.Attributes {
			// CloudEvents v1.0 attributes MUST consist of lower-case letters ('a' to 'z') or digits ('0' to '9') as per
			// the spec. It's not even possible for a conformant transport to allow non-base36 characters.
			// Note `SetExtension` will make it lowercase so only `IsAlphaNumeric` needs to be checked here.
			if IsAlphaNumeric(k) {
				event.SetExtension(k, v)
			}
		}
	}
	return &event, nil
}
