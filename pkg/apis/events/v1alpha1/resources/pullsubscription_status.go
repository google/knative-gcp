/*
Copyright 2020 Google LLC.

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
package resources

import (
	pubsubv1alpha1 "github.com/google/knative-gcp/pkg/apis/pubsub/v1alpha1"
	"knative.dev/pkg/apis"
)

func ReadyPullSubscriptionStatus() *pubsubv1alpha1.PullSubscriptionStatus {
	pss := &pubsubv1alpha1.PullSubscriptionStatus{}
	pss.InitializeConditions()
	pss.MarkSink(apis.HTTP("test.mynamespace.svc.cluster.local"))
	pss.MarkDeployed()
	pss.MarkSubscribed("subID")
	return pss
}

func FalsePullSubscriptionStatus() *pubsubv1alpha1.PullSubscriptionStatus {
	pss := &pubsubv1alpha1.PullSubscriptionStatus{}
	pss.InitializeConditions()
	pss.MarkNotDeployed("not deployed", "not deployed")
	return pss
}

func UnknownPullSubscriptionStatus() *pubsubv1alpha1.PullSubscriptionStatus {
	pss := &pubsubv1alpha1.PullSubscriptionStatus{}
	pss.InitializeConditions()
	return pss
}
