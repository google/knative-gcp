// +build e2e

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
	"fmt"
	"os"
	"strings"
	"testing"

	"knative.dev/pkg/test/logstream"
)

var packages = []string{
	"github.com/google/knative-gcp/test/cmd/target",
	"github.com/google/knative-gcp/test/cmd/storage_target",
}

var packageToImageConfig = map[string]string{}
var packageToImageConfigDone bool

func TestMain(m *testing.M) {
	for _, pack := range packages {
		image, err := KoPublish(pack)
		if err != nil {
			fmt.Printf("error attempting to ko publish: %s\n", err)
			panic(err)
		}
		i := strings.Split(pack, "/")
		packageToImageConfig[i[len(i)-1]+"Image"] = image
	}
	packageToImageConfigDone = true

	os.Exit(m.Run())
}

// This test is more for debugging the ko publish process.
func TestKoPublish(t *testing.T) {
	for k, v := range packageToImageConfig {
		t.Log(k, "-->", v)
	}
}

// Rest of e2e tests go below:

// TestSmoke makes sure we can run tests.
func TestSmokeChannel(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokeTestChannelImpl(t)
}

// TestSmokePullSubscription makes sure we can run tests on PullSubscriptions.
func TestSmokePullSubscription(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	SmokePullSubscriptionTestImpl(t)
}

// TestPullSubscriptionWithTarget tests we can knock down a target.
func TestPullSubscriptionWithTarget(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()

	PullSubscriptionWithTargetTestImpl(t, packageToImageConfig)
}

// TestStorage tests we can knock down a target fot storage
func TestStorage(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	StorageWithTestImpl(t, packageToImageConfig)
}
