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

/*
 Utility functions used for testing
*/
package lib

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"

	apiduckv1 "github.com/google/knative-gcp/pkg/apis/duck/v1"
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/utils/authcheck"
)

// GetEnvOrFail gets the specified environment variable. If the variable is not set, then the test exits with an error.
func GetEnvOrFail(t *testing.T, key string) string {
	value, success := os.LookupEnv(key)
	if !success {
		t.Fatalf("Environment variable %q not set", key)
	}
	return value
}

// WaitForSourceAuthCheckPending polls the status of the Source from client
// every interval until isSourceAuthCheckPending returns `true` indicating
// the Source is in AuthenticationCheckPending state,
// or returns an error, or timeout.
// AuthenticationCheckPending is the state that will continue until authentication check succeeds.
func WaitForSourceAuthCheckPending(dynamicClient dynamic.Interface, obj *resources.MetaResource) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		return checkSourceAuthCheckPending(dynamicClient, obj)
	})
}

func checkSourceAuthCheckPending(dynamicClient dynamic.Interface, obj *resources.MetaResource) (bool, error) {
	psObj, err := duck.GetGenericObject(dynamicClient, obj, &apiduckv1.PubSub{})
	if err != nil {
		// Return error to stop the polling.
		return false, err
	}
	return isSourceAuthCheckPending(psObj), nil
}

func isSourceAuthCheckPending(psObj runtime.Object) bool {
	source := psObj.(*apiduckv1.PubSub)
	cond := source.Status.GetCondition(v1.PullSubscriptionConditionReady)
	return cond != nil && cond.IsUnknown() && cond.Reason == authcheck.AuthenticationCheckUnknownReason
}
