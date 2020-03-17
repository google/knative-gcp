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

package converters

import (
	"context"
	"errors"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	. "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents"
	cepubsub "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/pubsub"
	pubsubcontext "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/pubsub/context"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler/events/scheduler/resources"
)

const (
	CloudSchedulerConverter = "com.google.cloud.scheduler"
)

func convertCloudScheduler(ctx context.Context, msg *cepubsub.Message, sendMode ModeType) (*cloudevents.Event, error) {
	tx := pubsubcontext.TransportContextFrom(ctx)
	// Make a new event and convert the message payload.
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(tx.ID)
	event.SetTime(tx.PublishTime)
	// We do not know the content type and we do not want to inspect the payload,
	// thus we set this generic one.
	event.SetDataContentType("application/octet-stream")
	event.SetType(v1alpha1.CloudSchedulerSourceExecute)
	// Set the schema if it comes as an attribute.
	if val, ok := msg.Attributes["schema"]; ok {
		delete(msg.Attributes, "schema")
		event.SetDataSchema(val)
	}
	// Set the source and subject if it comes as an attribute.
	jobName, ok := msg.Attributes[v1alpha1.CloudSchedulerSourceJobName]
	if ok {
		delete(msg.Attributes, v1alpha1.CloudSchedulerSourceJobName)
	} else {
		return nil, errors.New("received event did not have jobName")
	}
	schedulerName, ok := msg.Attributes[v1alpha1.CloudSchedulerSourceName]
	if ok {
		delete(msg.Attributes, v1alpha1.CloudSchedulerSourceName)
	} else {
		return nil, errors.New("received event did not have schedulerName")
	}
	parentName := resources.ExtractParentName(jobName)
	event.SetSource(v1alpha1.CloudSchedulerSourceEventSource(parentName, schedulerName))
	event.SetSubject(resources.ExtractJobID(jobName))

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
