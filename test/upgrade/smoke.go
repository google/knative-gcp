/*
Copyright 2020 The Knative Authors
Modified work Copyright 2020 Google LLC

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

package upgrade

import (
	"testing"

	"knative.dev/eventing/test/lib"
	"knative.dev/hack/shell"
)

var channelTestRunner lib.ComponentsTestRunner

func runSmokeTest(t *testing.T) {
	loc, err := shell.NewProjectLocation("../..")
	if err != nil {
		t.Fatal("Failed to get project location ", err)
	}
	funcName := "go_test_e2e"
	exec := shell.NewExecutor(shell.ExecutorConfig{
		ProjectLocation: loc,
	})
	fn := shell.Function{
		Script: shell.Script{
			Label:      funcName,
			ScriptPath: "test/e2e-secret-lib.sh",
		},
		FunctionName: funcName,
	}
	args := []string{"-tags=e2e", "-timeout=30m", "./test/e2e", "-run=^TestSmokeGCPBroker"}
	if err := exec.RunFunction(fn, args...); err != nil {
		t.Errorf("Function %v failed: %v\n", funcName, err)
	}
}
