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
	"fmt"
	"os"
	"testing"
)

// Returns the value of the environment variable if it exists, otherwise exits the test with an error
func GetEnvOrFail(t *testing.T, key string) string {
	value, success := os.LookupEnv(key)
	if !success {
		t.Fatal(fmt.Sprintf("Environment variable %s not set", key))
	}
	return value
}
