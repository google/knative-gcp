#!/usr/bin/env bash

# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

source $(dirname "${BASH_SOURCE[0]}")/e2e-wi-lib.sh

if [ "${SKIP_TESTS:-}" == "true" ]; then
  echo "**************************************"
  echo "***         TESTS SKIPPED          ***"
  echo "**************************************"
  exit 0
fi

# Create a cluster with Workload Identity enabled.
# We could specify --version to force the cluster using a particular GKE version.
initialize "$@" --enable-workload-identity=true

# Channel related e2e tests we have in Eventing is not running here.
go_test_e2e -timeout=30m -parallel=6 ./test/e2e -workloadIdentity=true -serviceAccountName="${K8S_SERVICE_ACCOUNT_NAME}" || fail_test

success
