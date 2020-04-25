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

package brokercell

import (
	"context"
	"fmt"
	"testing"

	intv1alpha1 "github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
	bcreconciler "github.com/google/knative-gcp/pkg/client/injection/reconciler/intevents/v1alpha1/brokercell"
	"github.com/google/knative-gcp/pkg/reconciler"
	. "github.com/google/knative-gcp/pkg/reconciler/testing"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS         = "testnamespace"
	brokerCellName = "test-brokercell"
)

var (
	testKey = fmt.Sprintf("%s/%s", testNS, brokerCellName)
)

func init() {
	// Add types to scheme
	_ = intv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		Key:  testKey,
	}}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher, testData map[string]interface{}) controller.Reconciler {
		r := &Reconciler{
			Base: reconciler.NewBase(ctx, controllerAgentName, cmw),
		}
		return bcreconciler.NewReconciler(ctx, r.Logger, r.RunClientSet, listers.GetBrokerCellLister(), r.Recorder, r)
	}))
}
