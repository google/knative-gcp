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

# Returns whether the current branch is a release branch.
# Overrides test-infra's is_release_branch to allow for an optional letter suffix.
function is_release_branch() {
  [[ $(current_branch) =~ ^release-[0-9\.]+[a-z]?$ ]]
}

# Returns the URL to the latest manifest for the given Knative project.
# Overrides test-infra's get_latest_knative_yaml_source to prune optional letter suffix
# for release branches.
# Parameters: $1 - repository name of the given project
#             $2 - name of the yaml file, without extension
function get_latest_knative_yaml_source() {
  local repo_name="$1"
  local yaml_name="$2"
  # If it's a release branch, the yaml source URL should point to a specific version.
  if is_release_branch; then
    # Extract the release major&minor version from the branch name.
    local branch_name="$(current_branch)"
    local major_minor="${${branch_name##release-}%%[a-z]}"
    # Find the latest release manifest with the same major&minor version.
    local yaml_source_path="$(
      gsutil ls gs://knative-releases/${repo_name}/previous/v${major_minor}.*/${yaml_name}.yaml 2> /dev/null \
      | sort \
      | tail -n 1 \
      | cut -b6-)"
    # The version does exist, return it.
    if [[ -n "${yaml_source_path}" ]]; then
      echo "https://storage.googleapis.com/${yaml_source_path}"
      return
    fi
    # Otherwise, fall back to nightly.
  fi
  echo "https://storage.googleapis.com/knative-nightly/${repo_name}/latest/${yaml_name}.yaml"
}

# Get the latest YAML sources again to override the test-infra sources which
# fail to recognize release-0.15b as a release branch.
readonly KNATIVE_SERVING_RELEASE_CRDS="$(get_latest_knative_yaml_source "serving" "serving-crds")"
readonly KNATIVE_SERVING_RELEASE_CORE="$(get_latest_knative_yaml_source "serving" "serving-core")"
readonly KNATIVE_NET_ISTIO_RELEASE="$(get_latest_knative_yaml_source "net-istio" "net-istio")"
readonly KNATIVE_EVENTING_RELEASE="$(get_latest_knative_yaml_source "eventing" "eventing")"
readonly KNATIVE_MONITORING_RELEASE="$(get_latest_knative_yaml_source "serving" "monitoring")"

# Install all required components for running knative-gcp.
function start_knative_gcp() {
  start_latest_knative_serving || return 1
  start_latest_knative_eventing || return 1
  start_knative_monitoring "$KNATIVE_MONITORING_RELEASE" || return 1
  cloud_run_events_setup || return 1
  istio_patch || return 1
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
