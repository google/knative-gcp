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

# performance-tests.sh is added to manage all clusters that run the performance
# benchmarks in eventing repo, it is ONLY intended to be run by Prow, users
# should NOT run it manually.

# Setup env vars to override the default settings
export PROJECT_NAME="knative-eventing-performance"
export BENCHMARK_ROOT_PATH="test/performance/benchmarks"

source vendor/knative.dev/hack/performance-tests.sh
source $(dirname $0)/../lib.sh

# Vars used in this script
readonly TEST_CONFIG_VARIANT="continuous"
readonly TEST_NAMESPACE="default"
readonly PUBSUB_SECRET_NAME="google-cloud-key"
readonly CONTROL_PLANE_NAMESPACE="cloud-run-events"
readonly CONTROL_PLANE_SECRET_NAME="google-cloud-key"

function update_knative() {
  # Create the secret for pub-sub if it does not exist.
  kubectl -n ${TEST_NAMESPACE} get secret ${PUBSUB_SECRET_NAME} || \
  kubectl -n ${TEST_NAMESPACE} create secret generic ${PUBSUB_SECRET_NAME} \
    --from-file=key.json="${GOOGLE_APPLICATION_CREDENTIALS}"

  # Create the control-plane namespace if it does not exist.
  kubectl get namespace ${CONTROL_PLANE_NAMESPACE} || \
    kubectl create namespace ${CONTROL_PLANE_NAMESPACE}

  # Create the secret for control-plane if it does not exist.
  kubectl -n ${CONTROL_PLANE_NAMESPACE} get secret ${CONTROL_PLANE_SECRET_NAME} || \
  kubectl -n ${CONTROL_PLANE_NAMESPACE} create secret generic ${CONTROL_PLANE_SECRET_NAME} \
    --from-file=key.json="${GOOGLE_APPLICATION_CREDENTIALS}"

  # Start Knative GCP. Fail the update process if there is any error.
  start_knative_gcp || return 1
}

function update_benchmark() {
  local benchmark_path="${BENCHMARK_ROOT_PATH}/$1"
  # TODO(chizhg): add update_environment function in hack/performance-tests.sh and move the below code there
  echo ">> Updating configmap"
  kubectl delete configmap config-mako -n "${TEST_NAMESPACE}" --ignore-not-found=true
  kubectl create configmap config-mako -n "${TEST_NAMESPACE}" --from-file="${benchmark_path}/prod.config" || abort "failed to create config-mako configmap"
  kubectl patch configmap config-mako -n "${TEST_NAMESPACE}" -p '{"data":{"environment":"prod"}}' || abort "failed to patch config-mako configmap"

  echo ">> Updating benchmark $1"
  ko delete -f "${benchmark_path}"/${TEST_CONFIG_VARIANT} --ignore-not-found=true --wait=false
  sleep 30

  # Add Git info in kodata so the benchmark can show which commit it's running on.
  local kodata_path="vendor/knative.dev/eventing/test/test_images/performance/kodata"
  mkdir "${kodata_path}"
  ln -s "${REPO_ROOT_DIR}/.git/HEAD" "${kodata_path}"
  ln -s "${REPO_ROOT_DIR}/.git/refs" "${kodata_path}"
  ko apply --strict -f "${benchmark_path}"/${TEST_CONFIG_VARIANT} || abort "failed to apply benchmark $1"

  echo "Sleeping 2 min to wait for all resources to setup"
  sleep 120
  # In the current implementation, for some reason there can be error pods after the setup, but it does not necessarily
  # mean there is an error. Delete the error pods after the setup is done.
  # TODO(chizhg): remove it after there is no longer error pod.
  delete_error_pods
}

function delete_error_pods() {
  local pods="$(kubectl get pods --no-headers -n "${TEST_NAMESPACE}" 2>/dev/null)"
  # Get pods that are not running.
  local not_running_pods=$(echo "${pods}" | grep -v Running | grep -v Completed)
  if [[ -n "${not_running_pods}" ]]; then
    # Delete all pods that are not in Running or Completed status.
    while read pod ; do
      pod_name=$(echo -n "${pod}" | cut -f1 -d' ')
      echo "Deleting error pod ${pod_name} from test namespace ${TEST_NAMESPACE}"
      kubectl delete pod "${pod_name}" -n "${TEST_NAMESPACE}"
    done <<< "${not_running_pods}"
  fi
}

main $@
