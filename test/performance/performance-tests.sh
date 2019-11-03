#!/bin/bash

# Copyright 2019 The Knative Authors
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

# performance-tests.sh is added to manage all clusters that run the performance
# benchmarks in eventing repo, it is ONLY intended to be run by Prow, users
# should NOT run it manually.

# Setup env vars to override the default settings
export PROJECT_NAME="knative-eventing-performance"
export BENCHMARK_ROOT_PATH="$GOPATH/src/github.com/chizhg/knative-gcp/test/performance/benchmarks"

source vendor/knative.dev/test-infra/scripts/performance-tests.sh
source $(dirname $0)/../lib.sh

# Vars used in this script
readonly TEST_CONFIG_VARIANT="continuous"
readonly TEST_NAMESPACE="default"
readonly PUBSUB_SECRET_NAME="google-cloud-key"
readonly CLOUD_RUN_EVENTS_CONFIG="config/"

# Install the knative-gcp resources from the repo
function install_knative_gcp_resources() {
  pushd .
  cd ${GOPATH}/src/github.com/chizhg/knative-gcp

  echo ">> Update knative-gcp core"
  ko apply -f ${CLOUD_RUN_EVENTS_CONFIG} || abort "Failed to install knative-gcp"

  popd
}

function update_knative() {
  start_latest_knative_eventing
  # Create the secret for pub-sub.
  kubectl -n ${TEST_NAMESPACE} create secret generic ${PUBSUB_SECRET_NAME} \
    --from-file=key.json=${GOOGLE_APPLICATION_CREDENTIALS}
  install_knative_gcp_resources
}

function update_benchmark() {
  echo ">> Updating benchmark $1"
  ko delete -f ${BENCHMARK_ROOT_PATH}/$1/${TEST_CONFIG_VARIANT}
  ko apply -f ${BENCHMARK_ROOT_PATH}/$1/${TEST_CONFIG_VARIANT} || abort "failed to apply benchmark $1"
}

main $@
