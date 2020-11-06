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

package broker

import (
	"testing"

	"github.com/google/knative-gcp/pkg/apis/configs/brokerdelivery"
	"github.com/google/knative-gcp/pkg/apis/configs/dataresidency"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/system"
	tracingconfig "knative.dev/pkg/tracing/config"

	// Fake injection informers
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/broker/v1beta1/broker/fake"
	_ "github.com/google/knative-gcp/pkg/client/injection/informers/intevents/v1alpha1/brokercell/fake"
)

func TestNew(t *testing.T) {
	ctx, _ := SetupFakeContext(t)

	c := NewConstructor(&brokerdelivery.StoreSingleton{}, &dataresidency.StoreSingleton{})(ctx, configmap.NewStaticWatcher(
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
		NewBrokerDeliveryConfigMapFromDeliverySpec(nil),
		NewDataresidencyConfigMapFromRegions([]string{}),
	))

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}
