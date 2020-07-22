// +build e2e

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

package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/knative-gcp/test"
	"github.com/google/knative-gcp/test/e2e/lib"
	"github.com/google/knative-gcp/test/e2e/lib/metrics"
	"github.com/google/knative-gcp/test/e2e/lib/resources"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingtest "knative.dev/eventing/test"
	eventingtestlib "knative.dev/eventing/test/lib"
	"knative.dev/pkg/test/zipkin"
)

const knativeCustomMetricPrefix = "custom.googleapis.com/knative.dev/"

var (
	channelTestRunner eventingtestlib.ComponentsTestRunner
	authConfig        lib.AuthConfig
	projectID         string
)

func TestMain(m *testing.M) {
	os.Exit(func() int {
		test.InitializeFlags()
		eventingtest.InitializeEventingFlags()
		projectID = os.Getenv(lib.ProwProjectKey)
		channelTestRunner = eventingtestlib.ComponentsTestRunner{
			// ChannelFeatureMap saves the channel-features mapping.
			// Each pair means the channel support the given list of features.
			ComponentFeatureMap: map[metav1.TypeMeta][]eventingtestlib.Feature{
				{
					APIVersion: resources.MessagingAPIVersion,
					Kind:       "Channel",
				}: {
					eventingtestlib.FeatureBasic,
					eventingtestlib.FeatureRedelivery,
					eventingtestlib.FeaturePersistence,
				},
				{
					APIVersion: resources.MessagingV1beta1APIVersion,
					Kind:       "Channel",
				}: {
					eventingtestlib.FeatureBasic,
					eventingtestlib.FeatureRedelivery,
					eventingtestlib.FeaturePersistence,
				},
			},
			ComponentsToTest: eventingtest.EventingFlags.Channels,
		}
		authConfig.WorkloadIdentity = test.Flags.WorkloadIdentity
		// The format of a Google Cloud Service Account is: service-account-name@project-id.iam.gserviceaccount.com.
		if authConfig.WorkloadIdentity {
			authConfig.ServiceAccountName = test.Flags.ServiceAccountName
		}
		// Any tests may SetupZipkinTracing, it will only actually be done once. This should be the ONLY
		// place that cleans it up. If an individual test calls this instead, then it will break other
		// tests that need the tracing in place.
		defer zipkin.CleanupZipkinTracingSetup(log.Printf)

		if code := m.Run(); code > 0 {
			return code
		}

		// The knative/pkg base controller emits custom metrics, plus some other components can
		// potentially emit custom metrics. By default custom metrics should not be published to
		// Stackdriver.
		// After running all tests, we verify that no custom metrics have been emitted in the past
		// 30 min to cover roughly how long the e2e tests run.
		if err := verifyNoCustomMetrics(projectID, time.Duration(-30)*time.Minute); err != nil {
			log.Fatalf("Failed to verify that no custom metrics are emitted: %v", err)
		}

		return 0
	}())
}

func verifyNoCustomMetrics(projectID string, duration time.Duration) error {
	client, err := monitoring.NewMetricClient(context.TODO())
	if err != nil {
		return fmt.Errorf("Failed to create metrics client: %w\n", err)
	}
	defer client.Close()
	// Get the metric descriptors with custom metric prefix.
	req := &monitoringpb.ListMetricDescriptorsRequest{
		Name:   fmt.Sprintf("projects/%s", projectID),
		Filter: fmt.Sprintf(`metric.type=starts_with("%s")`, knativeCustomMetricPrefix),
	}
	descriptors, err := metrics.ListMetricDescriptors(context.TODO(), client, req)
	if err != nil {
		return fmt.Errorf("Failed to list custom metrics: %w\n", err)
	}

	// Check no data points of custom metrics in the past duration.
	fmt.Printf("Got %d custom metric definitions with prefix %s\n", len(descriptors), knativeCustomMetricPrefix)
	for _, d := range descriptors {
		if err := verifyNoPoints(client, projectID, d.GetType(), duration); err != nil {
			return err
		}
	}
	return nil
}

// verifyNoPoints verifies no metrics data points are found for the given metric type.
func verifyNoPoints(client *monitoring.MetricClient, projectID string, mt string, duration time.Duration) error {
	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", projectID),
		Filter: fmt.Sprintf(`metric.type="%s"`, mt),
		Interval: &monitoringpb.TimeInterval{
			StartTime: &timestamp.Timestamp{Seconds: time.Now().Add(duration).Unix()},
			EndTime:   &timestamp.Timestamp{Seconds: time.Now().Unix()},
		},
	}
	tss, err := metrics.ListTimeSeries(context.TODO(), client, req)
	if err != nil {
		return fmt.Errorf("Failed to fetch data points of custom metric %s: %w\n", mt, err)
	}
	for _, ts := range tss {
		if len(ts.GetPoints()) > 0 {
			return fmt.Errorf("Found data points of custom metric %s, which should be disabled", mt)
		}
	}
	return nil
}
