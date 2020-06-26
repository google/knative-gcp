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

	"cloud.google.com/go/pubsub"
	cev2 "github.com/cloudevents/sdk-go/v2"
	"github.com/google/knative-gcp/pkg/apis/events/v1beta1"
	. "github.com/google/knative-gcp/pkg/pubsub/adapter/context"
)

const (
	buildSchemaUrl = "https://raw.githubusercontent.com/google/knative-gcp/master/schemas/build/schema.json"
)

func convertCloudBuild(ctx context.Context, msg *pubsub.Message) (*cev2.Event, error) {
	event := cev2.NewEvent(cev2.VersionV1)
	event.SetID(msg.ID)
	event.SetTime(msg.PublishTime)
	event.SetType(v1beta1.CloudBuildSourceEvent)
	event.SetDataSchema(buildSchemaUrl)

	project, err := GetProjectKey(ctx)
	if err != nil {
		return nil, err
	}

	// Set the source and subject if it comes as an attribute.
	if buildId, ok := msg.Attributes[v1beta1.CloudBuildSourceBuildId]; !ok {
		return nil, errors.New("received event did not have buildId")
	} else {
		event.SetSource(v1beta1.CloudBuildSourceEventSource(project, buildId))
	}
	if buildStatus, ok := msg.Attributes[v1beta1.CloudBuildSourceBuildStatus]; !ok {
		return nil, errors.New("received event did not have build status")
	} else {
		event.SetSubject(buildStatus)
	}

	if err := event.SetData(cev2.ApplicationJSON, msg); err != nil {
		return nil, err
	}

	return &event, nil
}
