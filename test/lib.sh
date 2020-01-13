#!/bin/bash

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

# Include after test-infra/scripts/library.sh

readonly CLOUD_RUN_EVENTS_CONFIG="config/"

# Install all required components for running knative-gcp.
function start_knative_gcp() {
  start_latest_knative_serving
  start_latest_knative_eventing
  cloud_run_events_setup
  istio_patch
}

# Setup the Cloud Run Events environment for running tests.
function cloud_run_events_setup() {
  # Install the latest Cloud Run Events in the current cluster.
  header "Starting Cloud Run Events"
  subheader "Installing Cloud Run Events"
  ko apply -f ${CLOUD_RUN_EVENTS_CONFIG} || return 1
  wait_until_pods_running cloud-run-events || return 1
}

function istio_patch() {
  header "Patching Istio"
  kubectl apply -f test/e2e/config/istio-patch/istio-knative-extras.yaml
}

# Install Knative Eventing in the current cluster.
# Parameters: $1 - Knative Eventing manifest.
function start_knative_eventing() {
  header "Starting Knative Eventing"
  subheader "Installing Knative Eventing"
  echo "Installing Eventing CRDs from $1"
  kubectl apply --selector knative.dev/crd-install=true -f "$1"
  echo "Installing the rest of eventing components from $1"
  kubectl apply -f "$1"
  wait_until_pods_running knative-eventing || return 1
}

# Install the stable release Knative/eventing in the current cluster.
# Parameters: $1 - Knative Eventing version number, e.g. 0.6.0.
function start_release_knative_eventing() {
  start_knative_eventing "https://storage.googleapis.com/knative-releases/eventing/previous/v$1/eventing.yaml"
}

# Install the latest stable Knative Eventing in the current cluster.
function start_latest_knative_eventing() {
  start_knative_eventing "${KNATIVE_EVENTING_RELEASE}"
}
