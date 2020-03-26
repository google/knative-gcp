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

package test

import (
	"flag"
	"fmt"
	"github.com/google/knative-gcp/test/e2e/lib"
	"log"
	"os"
)

// Flags holds the command line flags specific to knative-gcp.
var Flags EnvironmentFlags

// EventingEnvironmentFlags holds the e2e flags needed only by the eventing repo.
type EnvironmentFlags struct {
	WorkloadIdentityEnabled bool
	PubsubServiceAccount    string
}

// InitializeFlags registers flags used by e2e tests, calling flag.Parse() here would fail in
// go1.13+, see https://github.com/knative/test-infra/issues/1329 for details
func InitializeFlags() {
	flag.BoolVar(&Flags.WorkloadIdentityEnabled, "workloadIdentityEnabled", false, "Indicating whether the workload identity is enabled or not.")
	flag.StringVar(&Flags.PubsubServiceAccount, "pubsubServiceAccount", "", "Google Cloud ServiceAccount used for data plane.")
	flag.Parse()

	// WorkloadIdentity will be enabled only if the input is true.
	if Flags.WorkloadIdentityEnabled {
		// PubsubServiceAccount is used when WorkloadIdentity is enabled
		if Flags.PubsubServiceAccount == "" {
			log.Fatalf("PubsubServiceAccount not specified.")
		}
		// The format is service-account-name@project-id.iam.gserviceaccount.com
		Flags.PubsubServiceAccount = fmt.Sprintf("%v@%v.iam.gserviceaccount.com", Flags.PubsubServiceAccount, os.Getenv(lib.ProwProjectKey))
	} else {
		Flags.PubsubServiceAccount = ""
	}
}
