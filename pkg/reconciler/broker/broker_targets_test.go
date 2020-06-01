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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/knative-gcp/pkg/broker/config"
	"github.com/google/knative-gcp/pkg/broker/config/memory"
	"github.com/google/knative-gcp/pkg/reconciler"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"

	//. "knative.dev/pkg/reconciler/testing"
	fakerunclient "github.com/google/knative-gcp/pkg/client/injection/client/fake"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	fakeservingclient "knative.dev/serving/pkg/client/injection/client/fake"
)

func TestUpdateTargetsConfig(t *testing.T) {
	testCases := []struct {
		name          string
		targetsConfig config.Targets
		existing      *corev1.ConfigMap
		desired       *corev1.ConfigMap
	}{{
		name: "no existing",
		desired: NewConfigMap(targetsCMName, systemNS,
			WithConfigMapDataEntry("targets.txt", ""),
			WithConfigMapBinaryDataEntry("targets", nil),
		),
	}, {
		name:     "empty existing",
		existing: NewConfigMap(targetsCMName, systemNS),
		desired: NewConfigMap(targetsCMName, systemNS,
			WithConfigMapDataEntry("targets.txt", ""),
			WithConfigMapBinaryDataEntry("targets", nil),
		),
		//TODO tests verifying marshal of targets config
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			listers := NewListers([]runtime.Object{})
			if tc.existing != nil {
				listers = NewListers([]runtime.Object{tc.existing})
			}
			ctx := context.Background()
			logger := logtesting.TestLogger(t)
			ctx = logging.WithLogger(ctx, logger)

			ctx, kubeClient := fakekubeclient.With(ctx, listers.GetKubeObjects()...)
			ctx, _ = fakerunclient.With(ctx, listers.GetEventsObjects()...)
			ctx, _ = fakeservingclient.With(ctx, listers.GetServingObjects()...)
			ctx, _ = fakedynamicclient.With(ctx, scheme.Scheme, listers.GetAllObjects()...)

			if tc.targetsConfig == nil {
				tc.targetsConfig = memory.NewEmptyTargets()
			}

			r := &Reconciler{
				Base:               reconciler.NewBase(ctx, controllerAgentName, nil),
				triggerLister:      listers.GetTriggerLister(),
				configMapLister:    listers.GetConfigMapLister(),
				endpointsLister:    listers.GetEndpointsLister(),
				deploymentLister:   listers.GetDeploymentLister(),
				targetsConfig:      tc.targetsConfig,
				targetsNeedsUpdate: make(chan struct{}),
				projectID:          testProject,
				pubsubClient:       nil,
			}

			if err := r.updateTargetsConfig(ctx); err != nil {
				t.Errorf("%v", err)
			}
			if tc.desired != nil {
				want := tc.desired
				got, err := kubeClient.CoreV1().ConfigMaps(want.Namespace).Get(want.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("error getting desired configmap: %v", err)
				}
				if diff := cmp.Diff(want.Data, got.Data); diff != "" {
					t.Errorf("unexpected Data (-want, +got) = %v", diff)
				}
				if diff := cmp.Diff(want.BinaryData, got.BinaryData); diff != "" {
					t.Errorf("unexpected BinaryData (-want, +got) = %v", diff)
				}
			}

		})
	}
}
