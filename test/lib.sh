#!/bin/bash

# Copyright 2019 Google LLC
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

# Include after test-infra/scripts/library.sh

readonly CLOUD_RUN_EVENTS_CONFIG="config/"
readonly CLOUD_RUN_EVENTS_ISTIO_CONFIG="config/istio"
readonly CLOUD_RUN_EVENTS_GKE_CONFIG="config/core/deployments/controller-gke.yaml"

# Install all required components for running knative-gcp.
function start_knative_gcp() {
  start_latest_knative_serving || return 1
  start_latest_knative_eventing || return 1
  start_knative_monitoring "$KNATIVE_MONITORING_RELEASE" || return 1
  cloud_run_events_setup "$1"  || return 1
  istio_patch || return 1
}

# Setup the Cloud Run Events environment for running tests.
function cloud_run_events_setup() {
  # Install the latest Cloud Run Events in the current cluster.
  header "Starting Cloud Run Events"
  subheader "Installing Cloud Run Events"
  ko apply --strict -f ${CLOUD_RUN_EVENTS_CONFIG} || return 1
  ko apply --strict -f ${CLOUD_RUN_EVENTS_ISTIO_CONFIG} || return 1
  if [[ "$1" == "workloadIdentityEnabled" ]]; then
    ko apply --strict -f ${CLOUD_RUN_EVENTS_GKE_CONFIG} || return 1
  fi
  wait_until_pods_running cloud-run-events || return 1
}

function istio_patch() {
  header "Patching Istio"
  kubectl apply -f test/e2e/config/istio-patch/istio-knative-extras.yaml || return 1
}
