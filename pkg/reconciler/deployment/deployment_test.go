/*
Copyright 2019 Google LLC

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deployment

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/google/knative-gcp/pkg/apis/events/v1alpha1"
	"github.com/google/knative-gcp/pkg/reconciler"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"

	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testDeploymentName = "test-controller"
	testNS             = "test-cloud-run-events"
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {

	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "deployment updated",
			Objects: []runtime.Object{
				NewDeployment(testDeploymentName, testNS),
			},
			Key: testNS + "/" + testDeploymentName,
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewDeployment(testDeploymentName, testNS,
					WithDeploymentAnnotations(map[string]string{
						"events.cloud.google.com/secretLastObservedUpdateTime": "2019-11-17 20:34:58.651387237 +0000 UTC",
					}),
				),
			}},
		}, {
			Name: "deployment with annotation updated",
			Objects: []runtime.Object{
				NewDeployment(testDeploymentName, testNS,
					WithDeploymentAnnotations(map[string]string{
						"events.cloud.google.com/secretLastObservedUpdateTime": "2019-11-16 20:34:58.651387237 +0000 UTC",
					})),
			},
			Key: testNS + "/" + testDeploymentName,
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewDeployment(testDeploymentName, testNS,
					WithDeploymentAnnotations(map[string]string{
						"events.cloud.google.com/secretLastObservedUpdateTime": "2019-11-17 20:34:58.651387237 +0000 UTC",
					}),
				),
			}},
		},
	}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, _ map[string]interface{}) controller.Reconciler {
		return &Reconciler{
			Base:             reconciler.NewBase(ctx, controllerAgentName, cmw),
			deploymentLister: listers.GetDeploymentLister(),
			clock: clock.NewFakeClock(time.Date(
				2019, 11, 17, 20, 34, 58, 651387237, time.UTC)),
		}
	}))
}
