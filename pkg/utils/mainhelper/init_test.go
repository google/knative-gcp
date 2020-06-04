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

package mainhelper

import (
	"context"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricstest"
	"knative.dev/pkg/system"
	tracingconfig "knative.dev/pkg/tracing/config"

	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	_ "knative.dev/pkg/metrics/testing"
	_ "knative.dev/pkg/system/testing"
)

const component = "test"

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func TestInit(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		ctx, res := Init(component, WithKubeFakes, WithContext(context.Background()))
		defer res.Cleanup()
		defer unregisterMetrics()
		commonVerification(t, ctx, res)
	})

	t.Run("with envConfig", func(t *testing.T) {
		type testEnvConfig struct {
			Value string `envconfig:"TEST_ENV_KEY" required:"true"`
		}
		var env testEnvConfig
		os.Setenv("TEST_ENV_KEY", "test-value")
		ctx, res := Init(component, WithEnv(&env), WithKubeFakes, WithContext(context.Background()))
		defer res.Cleanup()
		defer unregisterMetrics()

		commonVerification(t, ctx, res)
		if env.Value != "test-value" {
			t.Errorf("EnvConfig not processed, got: %v, want: %v", env.Value, "test-value")
		}
	})

	t.Run("with Context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "key", "value")
		ctx, res := Init(component, WithKubeFakes, WithContext(ctx))
		defer res.Cleanup()
		defer unregisterMetrics()

		commonVerification(t, ctx, res)
		// Verify the initial context keys are preserved.
		if ctx.Value("key") != "value" {
			t.Errorf("Context is not preserved, got: %v, want: %v", ctx.Value("key"), "value")
		}
	})
}

func commonVerification(t *testing.T, ctx context.Context, res *InitRes) {
	if ctx == nil {
		t.Fatalf("Returned nil context")
	}
	if res == nil {
		t.Fatalf("InitRes is nil")
	}
	if res.Logger == nil || res.KubeClient == nil || res.CMPWatcher == nil || res.Cleanup == nil {
		t.Errorf("At least one of the InitRes fields are nil: %+v", res)
	}
}

func setup() {
	// Inject observability configmaps.
	loggingConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      logging.ConfigMapName(),
			Namespace: system.Namespace(),
		},
	}
	observabilityConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metrics.ConfigMapName(),
			Namespace: system.Namespace(),
		},
	}
	tracingConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tracingconfig.ConfigName,
			Namespace: system.Namespace(),
		},
	}
	cj := func(ctx context.Context, config *rest.Config) context.Context {
		return context.WithValue(ctx, kubeclient.Key{}, kubefake.NewSimpleClientset(loggingConfig, observabilityConfig, tracingConfig))
	}
	injection.Fake.RegisterClient(cj)
}

func teardown() {
	// nothing to do for now
}

func unregisterMetrics() {
	metricstest.Unregister("go_alloc", "go_total_alloc", "go_sys", "go_lookups", "go_mallocs", "go_frees", "go_heap_alloc", "go_heap_sys", "go_heap_idle", "go_heap_in_use", "go_heap_released", "go_heap_objects", "go_stack_in_use", "go_stack_sys", "go_mspan_in_use", "go_mspan_sys", "go_mcache_in_use", "go_mcache_sys", "go_bucket_hash_sys", "go_gc_sys", "go_other_sys", "go_next_gc", "go_last_gc", "go_total_gc_pause_ns", "go_num_gc", "go_num_forced_gc", "go_gc_cpu_fraction")
}
