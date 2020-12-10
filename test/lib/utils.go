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
	"io"
	"os"
	"testing"

	"knative.dev/hack/shell"
)

// GetEnvOrFail gets the specified environment variable. If the variable is not set, then the test exits with an error.
func GetEnvOrFail(t *testing.T, key string) string {
	value, success := os.LookupEnv(key)
	if !success {
		t.Fatalf("Environment variable %q not set", key)
	}
	return value
}

func CallShellFunctionAndGetStdout(funcName, scriptNameWithPath, projectLocation string, out io.Writer) error {
	loc, err := shell.NewProjectLocation(projectLocation)
	if err != nil {
		return err
	}
	exec := shell.NewExecutor(shell.ExecutorConfig{
		ProjectLocation: loc,
		Streams: shell.Streams{
			Out: out,
		},
	})
	fn := shell.Function{
		Script: shell.Script{
			Label:      funcName,
			ScriptPath: scriptNameWithPath,
		},
		FunctionName: funcName,
	}
	return exec.RunFunction(fn)
}

func CallShellFunction(funcName, scriptNameWithPath, projectLocation string) error {
	return CallShellFunctionAndGetStdout(funcName, scriptNameWithPath, projectLocation, nil)
}
