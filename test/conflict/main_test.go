// +build conflict

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

package conflict

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/google/knative-gcp/test/lib"
	"knative.dev/eventing/test"
)

const (
	funcName        = "install_cloud_run_events_nocheck_pods_running"
	projectLocation = "../../" // should point to knative_gcp directory
	scriptWithPath  = "test/lib.sh"
)

func TestGCPEventingInstallationObjectNameConflicts(t *testing.T) {
	var out bytes.Buffer
	err := lib.CallShellFunctionAndGetStdout(funcName, scriptWithPath, projectLocation, &out)
	if err != nil {
		t.Error(err)
		return
	}

	// The output for ko apply are in the following form
	// Dec 10 17:09:28.571 install_cloud_run_events_nocheck_pods_running [OUT] clusterrolebinding.rbac.authorization.k8s.io/cloud-run-events-controller created/configured
	// If the last word is "configured", there is a naming conflict with the components already installed
	pattern := "configured"
	var prevline string
	if strings.Contains(out.String(), pattern) {
		t.Error("The following objects in knative GCP installation have naming conflicts with existing components")
		for _, line := range strings.Fields(out.String()) {
			if strings.Contains(line, pattern) {
				t.Error(prevline)
			}
			prevline = line
		}
	}
}

func TestMain(m *testing.M) {
	test.InitializeEventingFlags()
	os.Exit(m.Run())
}
