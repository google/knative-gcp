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
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"knative.dev/pkg/test/zipkin"

	messagingv1alpha1 "github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/common"
	"knative.dev/eventing/test/conformance/helpers"
	"knative.dev/pkg/test/logstream"
)

var packages = []string{
	"github.com/google/knative-gcp/test/cmd/target",
	"github.com/google/knative-gcp/test/cmd/storage_target",
	"github.com/google/knative-gcp/test/cmd/sender",
	"github.com/google/knative-gcp/test/cmd/reciever",
}

var packageToImageConfig = map[string]string{}
var packageToImageConfigDone bool

func TestMain(m *testing.M) {
	for _, pack := range packages {
		image, err := KoPublish(pack)
		if err != nil {
			fmt.Printf("error attempting to ko publish: %s\n", err)
			panic(err)
		}
		i := strings.Split(pack, "/")
		packageToImageConfig[i[len(i)-1]+"Image"] = image
	}
	packageToImageConfigDone = true

	// Any tests may SetupZipkinTracing, it will only actually be done once. This should be the ONLY
	// place that cleans it up. If an individual test calls this instead, then it will break other
	// tests that need the tracing in place.
	defer zipkin.CleanupZipkinTracingSetup(log.Printf)

	os.Exit(m.Run())
}

// This test is more for debugging the ko publish process.
func TestKoPublish(t *testing.T) {
	for k, v := range packageToImageConfig {
		t.Log(k, "-->", v)
	}
}

// Rest of e2e tests go below:

// TestSmoke makes sure we can run tests.
func TestSmokeChannel(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeTestChannelImpl(t)
}

func TestChannelTracing(t *testing.T) {
	t.Skip("Skipping until https://github.com/knative/eventing/issues/2046 is fixed")
	cancel := logstream.Start(t)
	defer cancel()
	helpers.ChannelTracingTestHelper(t, metav1.TypeMeta{
		APIVersion: messagingv1alpha1.SchemeGroupVersion.String(),
		Kind:       "Channel",
	}, func(client *common.Client) error {
		// This test is running based on code in knative/eventing, so it does not use the same
		// Client that tests in this repo use. Therefore, we need to duplicate the logic from this
		// repo's Setup() here. See test/e2e/lifecycle.go's Setup() for the function used in this
		// repo whose functionality we need to copy here.

		// Copy the secret from the default namespace to the namespace used in the test.
		return copySecret(client)
	})
}

func copySecret(client *common.Client) error {
	secret, err := client.Kube.Kube.CoreV1().Secrets("default").Get("google-cloud-key", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("could not get secret: %v", err)
	}
	newSecret, err := client.Kube.Kube.CoreV1().Secrets(client.Namespace).Create(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secret.Name,
			Labels:      secret.Labels,
			Annotations: secret.Annotations,
		},
		Type:       secret.Type,
		Data:       secret.Data,
		StringData: secret.StringData,
	})
	if err != nil {
		return fmt.Errorf("could not create secret: %v", err)
	}
	client.Tracker.Add(newSecret.GroupVersionKind().Group, newSecret.GroupVersionKind().Version, "secrets", newSecret.Namespace, newSecret.Name)
	return nil
}

// TestSmokePullSubscription makes sure we can run tests on PullSubscriptions.
func TestSmokePullSubscription(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokePullSubscriptionTestImpl(t)
}

// TestPullSubscriptionWithTarget tests we can knock down a target.
func TestPullSubscriptionWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	PullSubscriptionWithTargetTestImpl(t, packageToImageConfig)
}

// TestSmokePubSub makes sure we can run tests on PubSubs.
func TestSmokePubSub(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokePubSubTestImpl(t)
}

// TestPubSubWithTarget tests we can knock down a target from a PubSub.
func TestPubSubWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	PubSubWithTargetTestImpl(t, packageToImageConfig, false /*assertMetrics */)
}

// TestPubSubStackDriverMetrics tests we can knock down a target from a PubSub and that we send metrics to StackDriver.
func TestPubSubStackDriverMetrics(t *testing.T) {
	t.Skip("See issues https://github.com/google/knative-gcp/issues/317 and https://github.com/cloudevents/sdk-go/pull/234")
	cancel := logstream.Start(t)
	defer cancel()
	PubSubWithTargetTestImpl(t, packageToImageConfig, true /*assertMetrics */)
}

// TestPubSubWithBroker tests we can knock a knative service from a broker with PubSub channel.
func TestPubSubWithBroker(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	PubSubWithBrokerTestImpl(t, packageToImageConfig, false /*assertMetrics */)
}

// TestStorage tests we can knock down a target from a Storage.
func TestStorage(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	StorageWithTestImpl(t, packageToImageConfig, false /*assertMetrics */)
}

// TestStorageStackDriverMetrics tests we can knock down a target from a Storage and that we send metrics to StackDriver.
func TestStorageStackDriverMetrics(t *testing.T) {
	t.Skip("See issue https://github.com/google/knative-gcp/issues/317")
	cancel := logstream.Start(t)
	defer cancel()
	StorageWithTestImpl(t, packageToImageConfig, true /*assertMetrics */)
}
