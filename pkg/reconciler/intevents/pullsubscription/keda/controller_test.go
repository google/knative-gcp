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

package keda

import (
	"os"
	"testing"

	iamtesting "github.com/google/knative-gcp/pkg/reconciler/testing"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	_ "knative.dev/pkg/metrics/testing"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/system"
	tracingconfig "knative.dev/pkg/tracing/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// Fake injection informers and clients
	_ "github.com/google/knative-gcp/pkg/client/injection/ducks/duck/v1/resource/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1/pullsubscription/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/addressable/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/batch/v1/job/fake"
	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
	_ "knative.dev/pkg/injection/clients/dynamicclient/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	_ = os.Setenv("PUBSUB_RA_IMAGE", "PUBSUB_RA_IMAGE")
	cmw := configmap.NewStaticWatcher(
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
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tracingconfig.ConfigName,
				Namespace: system.Namespace(),
			},
			Data: map[string]string{},
		},
	)
	c := newController(ctx, cmw, iamtesting.NoopIAMPolicyManager, iamtesting.NewGCPAuthTestStore(t, nil))

	if c == nil {
		t.Fatal("Expected newControllerWithIAMPolicyManager to return a non-nil value")
	}
}
