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

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	duckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	sourcev1 "github.com/google/knative-gcp/pkg/apis/events/v1"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/test/lib"
	"github.com/google/uuid"
	cloudlogging "google.golang.org/api/logging/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/test/helpers"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	iso8601 = "2006-01-02T15:04:05.999Z"
)

var (
	trueVal  = true
	falseVal = false
)

func CloudLoggingGCPControlPlaneTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	startTimestamp := time.Now()

	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	topicName := helpers.AppendRandomString("test-e2e-gcp-control-plane-")
	randomString := uuid.New().String()

	topic := &v1.Topic{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "internal.events.cloud.google.com/v1",
			Kind:       "Topic",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cloud-logging-test-topic",
			Annotations: map[string]string{
				// This will cause the controller and webhook to write randomString to their logs,
				// which we will check for using the Cloud Logging API below.
				v1.LoggingE2ETestAnnotation: randomString,
			},
		},
		Spec: v1.TopicSpec{
			Topic:           topicName,
			EnablePublisher: &falseVal,
		},
	}
	client.CreateTopicOrFail(topic)
	err := duck.WaitForResourceReady(client.Core.Dynamic, &resources.MetaResource{
		TypeMeta:   topic.TypeMeta,
		ObjectMeta: topic.ObjectMeta,
	})
	if err != nil {
		t.Fatalf("Topic did not become ready: %v", err)
	}

	// Sleep so that the logging API has a chance to receive the logs lines and process them before
	// we try to retrieve them.
	time.Sleep(2 * time.Minute)

	// Read from the Cloud Logging API.
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	verifyLogEntryInCloudLogging(t, ctx, containerHierarchy{
		gcpProject: project,
		namespace:  "cloud-run-events",
		container:  "controller",
	}, startTimestamp, randomString)
	verifyLogEntryInCloudLogging(t, ctx, containerHierarchy{
		gcpProject: project,
		namespace:  "cloud-run-events",
		container:  "webhook",
	}, startTimestamp, randomString)
}

func CloudLoggingCloudPubSubSourceTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	startTimestamp := time.Now()

	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	topicName, deleteTopic := lib.MakeTopicOrDie(t)
	defer deleteTopic()
	randomString := uuid.New().String()

	pss := &sourcev1.CloudPubSubSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "events.cloud.google.com/v1",
			Kind:       "CloudPubSubSource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: client.Namespace,
			Name:      "cloud-logging-test-source",
			Annotations: map[string]string{
				// This will cause the controller and webhook to write randomString to their logs,
				// which we will check for using the Cloud Logging API below.
				v1.LoggingE2ETestAnnotation: randomString,
			},
		},
		Spec: sourcev1.CloudPubSubSourceSpec{
			Topic: topicName,
			PubSubSpec: duckv1.PubSubSpec{
				SourceSpec: pkgduckv1.SourceSpec{
					Sink: pkgduckv1.Destination{
						URI: &apis.URL{
							Scheme: "http",
							Host:   "example.com",
						},
					},
				},
			},
		},
	}
	client.CreatePubSubOrFail(pss)
	err := duck.WaitForResourceReady(client.Core.Dynamic, &resources.MetaResource{
		TypeMeta:   pss.TypeMeta,
		ObjectMeta: pss.ObjectMeta,
	})
	if err != nil {
		t.Fatalf("CloudPubSubSource did not become ready: %v", err)
	}

	// Sleep so that the logging API has a chance to receive the logs lines and process them before
	// we try to retrieve them. Two minutes was chosen arbitrarily.
	time.Sleep(2 * time.Minute)

	// Read from the Cloud Logging API.
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	verifyLogEntryInCloudLogging(t, ctx, containerHierarchy{
		gcpProject: project,
		namespace:  client.Namespace,
		container:  "receive-adapter",
	}, startTimestamp, randomString)
}

func CloudLoggingTopicTestImpl(t *testing.T, authConfig lib.AuthConfig) {
	ctx := context.Background()
	startTimestamp := time.Now()

	client := lib.Setup(ctx, t, true, authConfig.WorkloadIdentity)
	defer lib.TearDown(ctx, client)

	topicName := helpers.AppendRandomString("test-e2e-gcp-control-plane-")
	randomString := uuid.New().String()

	topic := &v1.Topic{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "internal.events.cloud.google.com/v1",
			Kind:       "Topic",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: client.Namespace,
			Name:      "cloud-logging-test-topic",
			Annotations: map[string]string{
				// This will cause the data plane Pod to write randomString to its logs, which we
				// will check for using the Cloud Logging API below.
				v1.LoggingE2ETestAnnotation: randomString,
			},
		},
		Spec: v1.TopicSpec{
			Topic:           topicName,
			EnablePublisher: &trueVal,
		},
	}
	client.CreateTopicOrFail(topic)
	err := duck.WaitForResourceReady(client.Core.Dynamic, &resources.MetaResource{
		TypeMeta:   topic.TypeMeta,
		ObjectMeta: topic.ObjectMeta,
	})
	if err != nil {
		t.Fatalf("Topic did not become ready: %v", err)
	}

	// Sleep so that the logging API has a chance to receive the logs lines and process them before
	// we try to retrieve them.
	time.Sleep(2 * time.Minute)

	// Read from the Cloud Logging API.
	project := lib.GetEnvOrFail(t, lib.ProwProjectKey)
	verifyLogEntryInCloudLogging(t, ctx, containerHierarchy{
		gcpProject: project,
		namespace:  client.Namespace,
		container:  "user-container",
	}, startTimestamp, randomString)
}

type containerHierarchy struct {
	gcpProject string
	namespace  string
	container  string
}

func verifyLogEntryInCloudLogging(t *testing.T, ctx context.Context, ch containerHierarchy, startTimestamp time.Time, toSearchFor string) {
	present, err := readFromCloudLogging(ctx, ch, startTimestamp, toSearchFor)
	if err != nil {
		t.Errorf("Error reading from cloud logging API: %v", err)
	}
	if !present {
		t.Errorf("Unable to find %s's logs in cloud logging API", ch.container)
	}
}

func readFromCloudLogging(ctx context.Context, ch containerHierarchy, startTimestamp time.Time, toSearchFor string) (bool, error) {
	loggingService, err := cloudlogging.NewService(ctx)
	if err != nil {
		return false, err
	}
	// Generate the filter we will send to Cloud Logging.
	filter := strings.Builder{}
	// Only get entries after the start of the test. We subtract five minutes, just in case there is
	// significant clock skew between the local machine running the tests and the GKE pods.
	startTimestamp = startTimestamp.Add(-5 * time.Minute).UTC()
	filter.WriteString(fmt.Sprintf("timestamp >=\"%v\"", startTimestamp.Format(iso8601)))
	filter.WriteString(" AND resource.type=k8s_container")
	filter.WriteString(fmt.Sprintf(" AND resource.labels.namespace_name=%s", ch.namespace))
	filter.WriteString(fmt.Sprintf(" AND resource.labels.container_name=%s", ch.container))
	// The string to search for is written as the LoggingE2EFieldName JSON field.
	filter.WriteString(fmt.Sprintf(" AND jsonPayload.%s=\"%s\"", v1.LoggingE2EFieldName, toSearchFor))

	resp, err := loggingService.Entries.List(&cloudlogging.ListLogEntriesRequest{
		Filter:  filter.String(),
		OrderBy: "timestamp desc",
		// If we see any entries, we know this was a success.
		PageSize: 1,
		ResourceNames: []string{
			// Limit it to just this project.
			fmt.Sprintf("projects/%s", ch.gcpProject),
		},
	}).Do()
	if err != nil {
		return false, err
	}
	return len(resp.Entries) > 0, nil
}
