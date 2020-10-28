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

# Installs Zipkin for tracing tests.
readonly KNATIVE_GCP_MONITORING_YAML="test/e2e/config/monitoring.yaml"

# Install Knative Monitoring in the current cluster.
# Parameters: $1 - Knative Monitoring manifest.
# This is a lightly modified version of start_knative_monitoring() from test-infra's library.sh.
function start_knative_gcp_monitoring() {
  header "Starting Knative Eventing Monitoring"
  subheader "Installing Knative Eventing Monitoring"
  echo "Installing Monitoring from $1"
  kubectl apply -f "$1" || return 1
  wait_until_pods_running knative-eventing || return 1
}

# Install all required components for running knative-gcp.
function start_knative_gcp() {
  start_latest_knative_serving || return 1
  start_latest_knative_eventing || return 1
  start_knative_gcp_monitoring "$KNATIVE_GCP_MONITORING_YAML" || return 1
  cloud_run_events_setup || return 1
  istio_patch || return 1
  knative_eventing_config_tracing || return 1
}

# Setup the Cloud Run Events environment for running tests.
function cloud_run_events_setup() {
  # Install the latest Cloud Run Events in the current cluster.
  header "Starting Cloud Run Events"
  subheader "Installing Cloud Run Events"
  ko apply --strict -f ${CLOUD_RUN_EVENTS_CONFIG} || return 1
  ko apply --strict -f ${CLOUD_RUN_EVENTS_ISTIO_CONFIG} || return 1
  wait_until_pods_running cloud-run-events || return 1
}

function istio_patch() {
  header "Patching Istio"
  kubectl apply -f test/e2e/config/istio-patch/istio-knative-extras.yaml || return 1
}

function knative_eventing_config_tracing() {
  # Setup config-tracing in knative-eventing, which the tracing tests rely on.
  header "Updating ConfigMap knative-eventing/config-tracing"
  kubectl replace -f "test/e2e/config/knative-eventing-config-tracing.yaml" || return 1
}
