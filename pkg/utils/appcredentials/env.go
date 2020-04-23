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

// Package appcredentials provides utilities for the application credentials
// used to access GCP services.
package appcredentials

import "os"

const envKey = "GOOGLE_APPLICATION_CREDENTIALS"

// MustExistOrUnsetEnv checks if the credential file pointed by the
// `GOOGLE_APPLICATION_CREDENTIALS` env var exists. If not, it unsets the env
// var so that application can continue to attempt using fallback mechanisms for
// authentication.
// We use this trick to support both secret and workload identity. See https://github.com/google/knative-gcp/issues/792.
// - When using secret,  credential file is mounted under
// `GOOGLE_APPLICATION_CREDENTIALS` via secret volume mount.
// - When using workload identity, credential file doesn't exist as the volume
// mount is optional.
func MustExistOrUnsetEnv() {
	path := os.Getenv(envKey)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Unsetenv(envKey)
	}
}
