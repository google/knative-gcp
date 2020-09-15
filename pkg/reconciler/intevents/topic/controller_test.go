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

package topic

import (
	"os"
	"testing"

	reconcilertesting "github.com/google/knative-gcp/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/pkg/configmap"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers

	_ "knative.dev/pkg/client/injection/kube/informers/batch/v1/job/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
	_ "knative.dev/serving/pkg/client/injection/informers/serving/v1/service/fake"

	_ "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1/topic/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	_ = os.Setenv("PUBSUB_PUBLISHER_IMAGE", "PUBSUB_PUBLISHER_IMAGE")
	cmw := configmap.NewStaticWatcher(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tracingconfig.ConfigName,
				Namespace: system.Namespace(),
			},
			Data: map[string]string{},
		})
	c := newController(ctx, cmw, reconcilertesting.NoopIAMPolicyManager, reconcilertesting.NewGCPAuthTestStore(t, nil), reconcilertesting.NewDataresidencyTestStore(t, nil))

	if c == nil {
		t.Fatal("Expected newControllerWithIAMPolicyManager to return a non-nil value")
	}
}
