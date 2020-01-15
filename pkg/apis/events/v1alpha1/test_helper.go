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

package v1alpha1

import (
	"github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
)

type testHelper struct{}

// TestHelper contains helpers for unit tests.
var TestHelper = testHelper{}

func (testHelper) ReadyPullSubscriptionStatus() *v1alpha1.PullSubscriptionStatus {
	pss := &v1alpha1.PullSubscriptionStatus{}
	pss.InitializeConditions()
	pss.MarkSink("http://test.mynamespace.svc.cluster.local")
	pss.MarkDeployed()
	pss.MarkSubscribed("subID")
	return pss
}

func (testHelper) FalsePullSubscriptionStatus() *v1alpha1.PullSubscriptionStatus {
	pss := &v1alpha1.PullSubscriptionStatus{}
	pss.InitializeConditions()
	pss.MarkNotDeployed("not deployed", "not deployed")
	return pss
}

func (testHelper) UnknownPullSubscriptionStatus() *v1alpha1.PullSubscriptionStatus {
	pss := &v1alpha1.PullSubscriptionStatus{}
	pss.InitializeConditions()
	return pss
}
