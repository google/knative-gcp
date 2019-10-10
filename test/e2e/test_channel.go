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

package e2e

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	// The following line to load the gcp plugin (only required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// SmokeTestChannelImpl makes sure we can run tests.
func SmokeTestChannelImpl(t *testing.T) {
	client := Setup(t, true)
	defer TearDown(client)

	installer := NewInstaller(client.Dynamic, map[string]string{
		"namespace": client.Namespace,
	}, EndToEndConfigYaml([]string{"smoke_test", "istio"})...)

	// Create the resources for the test.
	if err := installer.Do("create"); err != nil {
		t.Errorf("failed to create, %s", err)
		return
	}

	// Delete deferred.
	defer func() {
		if err := installer.Do("delete"); err != nil {
			t.Errorf("failed to create, %s", err)
		}
		// Just chill for tick.
		time.Sleep(10 * time.Second)
	}()

	if err := client.WaitForResourceReady(client.Namespace, "e2e-smoke-test", schema.GroupVersionResource{
		Group:    "messaging.cloud.google.com",
		Version:  "v1alpha1",
		Resource: "channels",
	}); err != nil {
		t.Error(err)
	}
}
