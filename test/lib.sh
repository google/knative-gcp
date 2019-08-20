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

# Include after test-infra/scripts/library.sh

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
