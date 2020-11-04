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

# Include after hack/library.sh

# Set and export SYSTEM_NAMESPACE where Knative Eventing is installed for pkg/system.
readonly SYSTEM_NAMESPACE="knative-eventing"
export SYSTEM_NAMESPACE

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
  cloud_run_events_setup $@ || return 1
  istio_patch || return 1
  knative_eventing_config_tracing || return 1
}

# Setup the Cloud Run Events environment for running tests.
function cloud_run_events_setup() {
  local kne_config
  kne_config="${1:-${CLOUD_RUN_EVENTS_CONFIG}}"
  header "Starting Cloud Run Events"
  subheader "Installing Cloud Run Events from: ${kne_config}"
  if [ -d "${kne_config}" ]; then
    # Install the latest Cloud Run Events in the current cluster.
    ko apply --strict -f ${kne_config} || return 1
  else
    kubectl apply -f $kne_config || return 1
  fi
  ko apply --strict -f ${CLOUD_RUN_EVENTS_ISTIO_CONFIG}|| return 1
  wait_until_pods_running cloud-run-events || return 1
}

latest_version() {
  # TODO: currently we don't have the right version tag in master,
  # we should switch back to use the above logic instead in v0.19
  # https://github.com/google/knative-gcp/issues/1857

  #local semver=$(git describe --match "v[0-9]*" --abbrev=0)
  #local major_minor=$(echo "$semver" | cut -d. -f1-2)

  # Get the latest patch release for the major minor
  #git tag -l "${major_minor}*" | sort -r --version-sort | head -n1

  git tag | sort -r --version-sort | head -n1
}

# Latest release. If user does not supply this as a flag, the latest
# tagged release on the current branch will be used.
readonly LATEST_RELEASE_VERSION=$(latest_version)

function start_knative_gcp_from_head() {
  # Install Knative Eventing from HEAD in the current cluster.
  echo ">> Installing Knative GCP from HEAD"
  start_knative_gcp || return 1
}

function start_knative_gcp_from_latest_release() {
  header ">> Installing Knative GCP latest public release"
  local url="https://github.com/google/knative-gcp/releases/download/${LATEST_RELEASE_VERSION}"
  local yaml="cloud-run-events.yaml"

  start_knative_gcp \
    "${url}/${yaml}" || return 1
}

# The install_cloud_run_events* functions are used for installing a different version
# of cloud run events, so it does not reinstall iostio related work
function install_cloud_run_events() {
  local kne_config
  kne_config="${1:-${CLOUD_RUN_EVENTS_CONFIG}}"
  subheader "Installing Cloud Run Events from: ${kne_config}"
  if [ -d "${kne_config}" ]; then
    ko apply --strict -f ${kne_config} || return 1
  else
    kubectl apply -f $kne_config || return 1
  fi
  wait_until_pods_running cloud-run-events || return 1
}

function install_cloud_run_events_from_head() {
  # Install Cloud Run Events from HEAD in the current cluster.
  echo ">> Installing Cloud Run Events from HEAD"
  install_cloud_run_events || return 1
}

function install_cloud_run_events_from_latest_release() {
  header ">> Installing Cloud Run Events latest public release"
  local url="https://github.com/google/knative-gcp/releases/download/${LATEST_RELEASE_VERSION}"
  local yaml="cloud-run-events.yaml"

  install_cloud_run_events \
    "${url}/${yaml}" || return 1
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
