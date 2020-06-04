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
	"testing"

	_ "knative.dev/pkg/client/injection/kube/informers/core/v1/configmap/fake"
	_ "knative.dev/pkg/metrics/testing"
	_ "knative.dev/pkg/system/testing"
)

func TestNewInitArgs(t *testing.T) {
	t.Run("WithNoMetricNamespace", func(t *testing.T) {
		args := newInitArgs("<component>", WithKubeFakes, WithContext(context.Background()))
		if args.metricNamespace != "<component>" {
			t.Errorf("unexpected (-want, +got) = (%v, %v)", "<component>", args.metricNamespace)
		}
	})

	t.Run("WithMetricNamespace", func(t *testing.T) {
		args := newInitArgs("<component>", WithMetricNamespace("<metric>"), WithKubeFakes, WithContext(context.Background()))
		if args.metricNamespace != "<metric>" {
			t.Errorf("unexpected (-want, +got) = (%v, %v)", "<metric>", args.metricNamespace)
		}
	})
}
