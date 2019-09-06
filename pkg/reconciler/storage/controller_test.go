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

package storage

import (
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/system"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/configmap"
	logtesting "knative.dev/pkg/logging/testing"
	_ "knative.dev/pkg/metrics/testing"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "github.com/google/knative-gcp/pkg/client/clientset/versioned/typed/pubsub/v1alpha1/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/client/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/events/v1alpha1/storage/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/pullsubscription/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/pubsub/v1alpha1/topic/fake"
	_ "github.com/google/knative-gcp/pkg/reconciler/testing"
	_ "knative.dev/pkg/injection/informers/kubeinformers/batchv1/job/fake"
)

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	ctx, _ := SetupFakeContext(t)

	_ = os.Setenv("STORAGE_NOTIFICATION_IMAGE", "STORAGE_NOTIFICATION_IMAGE")

	c := NewController(ctx, configmap.NewStaticWatcher(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      logging.ConfigMapName(),
				Namespace: system.Namespace(),
			},
			Data: map[string]string{},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      metrics.ConfigMapName(),
				Namespace: system.Namespace(),
			},
			Data: map[string]string{},
		},
	))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
