#!/usr/bin/env bash

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

# This script includes common functions for testing setup and teardown.

# If gcloud is not available make it a no-op, not an error.
which gcloud &> /dev/null || gcloud() { echo "[ignore-gcloud $*]" 1>&2; }

# Constants used for creating ServiceAccount for the Control Plane if it's not running on Prow.
readonly CONTROL_PLANE_GSA_NON_PROW="events-controller-gsa"

# Constants used for creating ServiceAccount for the Sources if it's not running on Prow.
readonly SOURCES_GSA_NON_PROW="events-sources-gsa"

# Constants used for creating ServiceAccount for the Broker if it's not running on Prow.
readonly BROKER_GSA_NON_PROW="events-broker-gsa"

# Vendored eventing test images.
readonly VENDOR_EVENTING_TEST_IMAGES="vendor/knative.dev/eventing/test/test_images/"

# Constants used for authentication setup for GCP Broker if it's not running on Prow.
readonly APP_ENGINE_REGION="us-central"

export CONFIG_WARMUP_GCP_BROKER="test/test_configs/warmup-broker.yaml"
export CONFIG_INVALID_CREDENTIAL="test/test_configs/invalid-credential.json"

# Setup Knative GCP.
function knative_setup() {
  start_knative_gcp || return 1
  export_variable || return 1
  control_plane_setup || return 1
}

# Tear down tmp files which store the private key.
function test_teardown() {
  if (( ! IS_PROW )); then
    rm "${SOURCES_GSA_KEY_TEMP}"
    rm "${BROKER_GSA_KEY_TEMP}"
  fi
}

function publish_test_images() {
  # Publish test images.
  echo ">> Publishing test images"
  $(dirname "$0")/upload-test-images.sh ${VENDOR_EVENTING_TEST_IMAGES} e2e || fail_test "Error uploading test images from eventing"
  $(dirname "$0")/upload-test-images.sh "test/test_images" e2e || fail_test "Error uploading test images from knative-gcp"
}

# Create resources required for CloudSchedulerSource.
function create_app_engine() {
  echo "Create App Engine with region US-central needed for CloudSchedulerSource"
  # Please rememeber the region of App Engine and the location of CloudSchedulerSource defined in e2e tests(./test_scheduler.go) should be consistent.
  gcloud app create --region=${APP_ENGINE_REGION} || echo "AppEngine app with region ${APP_ENGINE_REGION} probably already exists, ignoring..."
}

function scheduler_setup() {
  if (( ! IS_PROW )); then
    create_app_engine
  fi
}

# Create resources required for Storage Admin setup.
function storage_setup() {
  if (( ! IS_PROW )); then
    storage_admin_set_up "${E2E_PROJECT_ID}"
  fi
}

# Create resources required for Sources authentication setup.
function sources_auth_setup() {
  local auth_mode=${1}

  if [ "${auth_mode}" == "secret" ]; then
    if (( ! IS_PROW )); then
      # When not running on Prow we need to set up a service account for sources.
      echo "Set up the Sources ServiceAccount"
      init_pubsub_service_account "${E2E_PROJECT_ID}" "${SOURCES_GSA_NON_PROW}"
      enable_monitoring "${E2E_PROJECT_ID}" "${SOURCES_GSA_NON_PROW}"
      gcloud iam service-accounts keys create "${SOURCES_GSA_KEY_TEMP}" \
        --iam-account="${SOURCES_GSA_NON_PROW}"@"${E2E_PROJECT_ID}".iam.gserviceaccount.com
    else
      delete_topics_and_subscriptions
    fi
    kubectl -n ${E2E_TEST_NAMESPACE} create secret generic "${SOURCES_GSA_SECRET_NAME}" --from-file=key.json="${SOURCES_GSA_KEY_TEMP}"
  elif [ "${auth_mode}" == "workload_identity" ]; then
    if (( ! IS_PROW )); then
      # When not running on Prow we need to set up a service account for sources.
      echo "Set up the Sources ServiceAccount"
      init_pubsub_service_account "${E2E_PROJECT_ID}" "${SOURCES_GSA_NON_PROW}"
      enable_monitoring "${E2E_PROJECT_ID}" "${SOURCES_GSA_NON_PROW}"
    else
      delete_topics_and_subscriptions
    fi
  else
    echo "Invalid parameter"
  fi
}

# Create resources required for GCP Broker authentication setup.
function broker_auth_setup() {
  echo "Authentication setup for GCP Broker"
  local auth_mode=${1}

  if [ "${auth_mode}" == "secret" ]; then
    if (( ! IS_PROW )); then
      # When not running on Prow we need to set up a service account for broker.
      init_pubsub_service_account "${E2E_PROJECT_ID}" "${BROKER_GSA_NON_PROW}"
      enable_monitoring "${E2E_PROJECT_ID}" "${BROKER_GSA_NON_PROW}"
      gcloud iam service-accounts keys create "${BROKER_GSA_KEY_TEMP}" \
        --iam-account="${BROKER_GSA_NON_PROW}"@"${E2E_PROJECT_ID}".iam.gserviceaccount.com
    fi
    kubectl -n "${CONTROL_PLANE_NAMESPACE}" create secret generic "${BROKER_GSA_SECRET_NAME}" --from-file=key.json="${BROKER_GSA_KEY_TEMP}"
  elif [ "${auth_mode}" == "workload_identity" ]; then
    if (( ! IS_PROW )); then
      # When not running on Prow we need to set up a service account for broker.
      init_pubsub_service_account "${E2E_PROJECT_ID}" "${BROKER_GSA_NON_PROW}"
      enable_monitoring "${E2E_PROJECT_ID}" "${BROKER_GSA_NON_PROW}"
      gcloud iam service-accounts add-iam-policy-binding \
        --role roles/iam.workloadIdentityUser \
        --member "${BROKER_MEMBER}" "${BROKER_GSA_EMAIL}"
    else
      gcloud iam service-accounts add-iam-policy-binding \
        --role roles/iam.workloadIdentityUser \
        --member "${BROKER_MEMBER}" \
        --project "${PROW_PROJECT_NAME}" "${BROKER_GSA_EMAIL}"
    fi
    kubectl annotate --overwrite serviceaccount ${BROKER_SERVICE_ACCOUNT} iam.gke.io/gcp-service-account="${BROKER_GSA_EMAIL}" \
      --namespace "${CONTROL_PLANE_NAMESPACE}"
  else
    echo "Invalid parameter"
  fi

  warmup_broker_setup || true
}

function prow_control_plane_setup() {
  local auth_mode=${1}

  if [ "${auth_mode}" == "secret" ]; then
    echo "Create the control plane secret"
    kubectl -n "${CONTROL_PLANE_NAMESPACE}" create secret generic "${CONTROL_PLANE_GSA_SECRET_NAME}" --from-file=key.json="${CONTROL_PLANE_GSA_KEY_TEMP}"
    echo "Delete the controller pod in the namespace '${CONTROL_PLANE_NAMESPACE}' to refresh the created/patched secret"
    kubectl delete pod -n "${CONTROL_PLANE_NAMESPACE}" --selector role=controller
  elif [ "${auth_mode}" == "workload_identity" ]; then
    cleanup_iam_policy_binding_members
    # Allow the Kubernetes service account to use Google service account.
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member "${MEMBER}" \
      --project "${PROW_PROJECT_NAME}" "${CONTROL_PLANE_GSA_EMAIL}"
    kubectl annotate --overwrite serviceaccount "${K8S_CONTROLLER_SERVICE_ACCOUNT}" iam.gke.io/gcp-service-account="${CONTROL_PLANE_GSA_EMAIL}" \
      --namespace "${CONTROL_PLANE_NAMESPACE}"
    # Setup default credential information for Workload Identity.
    sed "s/K8S_SERVICE_ACCOUNT_NAME/${K8S_SERVICE_ACCOUNT_NAME}/g; s/SOURCES-GOOGLE-SERVICE-ACCOUNT/${SOURCES_GSA_EMAIL}/g" ${CONFIG_GCP_AUTH} | ko apply -f -
  else
    echo "Invalid parameter"
  fi
}

function cleanup_iam_policy_binding_members() {
  # If the tests are run on Prow, clean up the member for roles/iam.workloadIdentityUser before running it.
  members=$(gcloud iam service-accounts get-iam-policy \
    --project="${PROW_PROJECT_NAME}" "${SOURCES_GSA_EMAIL}" \
    --format="value(bindings.members)" \
    --filter="bindings.role:roles/iam.workloadIdentityUser" \
    --flatten="bindings[].members")
  while read -r member_name
  do
    # Only delete the iam bindings that is related to the current boskos project.
    if [ "$(cut -d'.' -f1 <<< "${member_name}")" == "serviceAccount:${E2E_PROJECT_ID}" ]; then
      gcloud iam service-accounts remove-iam-policy-binding \
        --role roles/iam.workloadIdentityUser \
        --member "${member_name}" \
        --project "${PROW_PROJECT_NAME}" "${SOURCES_GSA_EMAIL}"
        # Add a sleep time between each get-set iam-policy-binding loop to avoid concurrency issue. Sleep time is based on the SLO.
        sleep 10
    fi
  done <<< "$members"
}

function delete_topics_and_subscriptions() {
    subs=$(gcloud pubsub subscriptions list --format="value(name)")
    while read -r sub_name
    do
      if [[ -n "${sub_name}" ]]; then
        gcloud pubsub subscriptions delete "${sub_name}"
      fi
    done <<<"$subs"
    topics=$(gcloud pubsub topics list --format="value(name)")
    while read -r topic_name
    do
      if [[ -n "${topic_name}" ]]; then
        gcloud pubsub topics delete "${topic_name}"
      fi
    done <<<"$topics"
}

function enable_monitoring(){
  local project_id=${1}
  local service_account=${2}

  echo "parameter project_id used when enabling monitoring is'${project_id}'"
  echo "parameter service_account used when enabling monitoring is'${service_account}'"
  # Enable monitoring
  echo "Enable Monitoring"
  gcloud services enable monitoring
  gcloud projects add-iam-policy-binding "${project_id}" \
      --member=serviceAccount:"${service_account}"@"${project_id}".iam.gserviceaccount.com \
      --role roles/monitoring.metricWriter
  gcloud projects add-iam-policy-binding "${project_id}" \
      --member=serviceAccount:"${service_account}"@"${project_id}".iam.gserviceaccount.com \
      --role roles/cloudtrace.agent
}

# test_authentication_check_for_brokercell tests the authentication check function for BrokerCell.
# This test required the entire BrokerCell to be non-functional.
# In order to avoid running in parallel with other tests, we put it in the shell script, rather than more common go tests.
# TODO Once we support multiple BrokerCells with distinct authentication mechanisms, make this a normal test that runs in parallel with the others.
function test_authentication_check_for_brokercell() {
  echo "Starting authentication check test for brokercell."
  local auth_mode=${1}
  kubectl apply -f "${CONFIG_WARMUP_GCP_BROKER}"

  echo "Starting authentication check test which is running outside of the broker related Pods."
  wait_until_brokercell_authentication_check_pending "$(non_pod_check_keywords)" || return 1

  echo "Starting authentication check test which is running inside of the broker related Pods."
  apply_invalid_auth "$auth_mode" || return 1
  wait_until_brokercell_authentication_check_pending "$(pod_check_keywords "$auth_mode")" || return 1

  # Clean up all the testing resources.
  echo "Authentication check test finished, waiting until all broker related testing resources deleted."
  delete_invalid_auth "$auth_mode" || return 1
  kubectl delete -f "${CONFIG_WARMUP_GCP_BROKER}"
  kubectl delete brokercell default -n "${CONTROL_PLANE_NAMESPACE}"
  kubectl wait pod --for=delete -n "${CONTROL_PLANE_NAMESPACE}" --selector=brokerCell=default --timeout=5m || return 1
}

function wait_until_brokercell_authentication_check_pending() {
  local keywords=${1}
  echo "Waiting until brokercell authentication check pending."
  for i in {1..150}; do #timeout after 5 minutes
    local message=$(kubectl get brokercell default -n "${CONTROL_PLANE_NAMESPACE}" -o=jsonpath='{.status.conditions[?(@.reason == "AuthenticationCheckPending")].message}' --ignore-not-found=true)
    if [[ "$message" == *"$keywords"* ]]; then
      return 0
    fi
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for brokercell authentication check pending with correct key message: ${keywords}"
  echo -e "brokercell object YAML: $(kubectl get brokercell default -n "${CONTROL_PLANE_NAMESPACE}" -o yaml)"
  return 1
}

function pod_check_keywords() {
  local auth_mode=${1}
  if [ "${auth_mode}" == "secret" ]; then
    echo "error getting the token, probably due to the key stored in the Kubernetes Secret is expired or revoked"
  elif [ "${auth_mode}" == "workload_identity" ]; then
    echo "the Pod is not fully authenticated, probably due to corresponding k8s service account and google service account do not establish a correct relationship"
  else
    echo "Invalid parameter"
    return 1
  fi
}

function non_pod_check_keywords() {
  echo "authentication is not configured"
}

function apply_invalid_auth() {
  local auth_mode=${1}
  if [ "${auth_mode}" == "secret" ]; then
    kubectl -n "${CONTROL_PLANE_NAMESPACE}" create secret generic "${BROKER_GSA_SECRET_NAME}" --from-file=key.json=${CONFIG_INVALID_CREDENTIAL}
  elif [ "${auth_mode}" == "workload_identity" ]; then
    kubectl -n "${CONTROL_PLANE_NAMESPACE}" annotate sa broker iam.gke.io/gcp-service-account=fakeserviceaccount@test-project.iam.gserviceaccount.com
  else
    echo "Invalid parameter"
    return 1
  fi
}

function delete_invalid_auth() {
  local auth_mode=${1}
  if [ "${auth_mode}" == "secret" ]; then
    kubectl -n "${CONTROL_PLANE_NAMESPACE}" delete secret "${BROKER_GSA_SECRET_NAME}"
  elif [ "${auth_mode}" == "workload_identity" ]; then
    kubectl -n "${CONTROL_PLANE_NAMESPACE}" annotate sa broker iam.gke.io/gcp-service-account-
  else
    echo "Invalid parameter"
    return 1
  fi
}

# The warm-up broker serves the following purposes:
#
# 1. When the broker data plane is created for the first time, it is expected
# that there will be some delay before workload identity credential being fully
# propagated. A warm-up broker will force the data plane to be created before
# the real testing. This helps prevent the credential propagation delay causing
# test flakiness.
#
# 2. The broker data plane will be GCed if there is no broker. Usually this would
# happen before we dump all the pod logs in the events-system namespace. The
# warm-up broker makes sure there is always one broker left and thus data plane pods
# won't be deleted before we dump logs.
function warmup_broker_setup(){
  ko apply -f ${CONFIG_WARMUP_GCP_BROKER}
}

function dump_extra_cluster_state() {
  for namespace in ${CONTROL_PLANE_NAMESPACE} gke-system knative-serving; do
    # Collecting logs from all system namespace pods.
    echo "============================================================"
    local controller_logs="${namespace}-controller-logs"
    local controller_logs_dir=${ARTIFACTS}/${controller_logs}
    echo "Creating directory ${controller_logs_dir}"
    mkdir -p ${controller_logs_dir}

    for pod in $(kubectl get pod -n $namespace | grep Running | awk '{print $1}' ); do
      for container in $(kubectl get pod "${pod}" -n $namespace -ojsonpath='{.spec.containers[*].name}'); do
        local current_output="${ARTIFACTS}/${controller_logs}/${namespace}-${pod}-${container}.txt"
        echo ">>> The dump of Namespace, Pod, Container: ${namespace}, ${pod}, ${container} is located at ${current_output}"
        echo "Namespace, Pod, Container: ${namespace}, ${pod}, ${container}"  >> "${current_output}"
        kubectl logs -n $namespace "${pod}" -c "${container}" >> "${current_output}" || true
        echo "----------------------------------------------------------"
        local previous_output="${ARTIFACTS}/${controller_logs}/previous-${namespace}-${pod}-${container}.txt"
        echo ">>> The dump of Namespace, Pod, Container (Previous instance): ${namespace}, ${pod}, ${container} is located at ${previous_output}"
        echo "Namespace, Pod, Container (Previous instance): ${namespace}, ${pod}, ${container}"  >> "${previous_output}"
        kubectl logs -p -n $namespace "${pod}" -c "${container}" >> "${previous_output}" || true
        echo "============================================================"
      done
    done
  done
}

function wait_for_file() {
  local file timeout waits
  file="$1"
  waits=300
  timeout=$waits

  echo "Waiting for existence of file: ${file}"

  while [ ! -f "${file}" ]; do
    # When the timeout is equal to zero, show an error and leave the loop.
    if [ "${timeout}" == 0 ]; then
      echo "ERROR: Timeout (${waits}s) while waiting for the file ${file}."
      return 1
    fi

    sleep 1

    # Decrease the timeout of one
    ((timeout--))
  done
  return 0
}
