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

source $(dirname "$0")/../vendor/knative.dev/hack/e2e-tests.sh

source $(dirname "$0")/lib.sh

source $(dirname "$0")/../hack/lib.sh

source $(dirname "$0")/e2e-common.sh

# Eventing main config.
readonly E2E_TEST_NAMESPACE="default"

# Constants used for creating ServiceAccount for the Control Plane if it's not running on Prow.
readonly CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP="$(mktemp)"

# Constants used for creating ServiceAccount for Data Plane(Pub/Sub Admin) if it's not running on Prow.
readonly PUBSUB_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP="$(mktemp)"

# Constants used for authentication setup for GCP Broker if it's not running on Prow.
readonly GCP_BROKER_SECRET_NAME="google-broker-key"

function export_variable() {
  if (( ! IS_PROW )); then
    readonly CONTROL_PLANE_SERVICE_ACCOUNT_KEY_TEMP="${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP}"
    readonly PUBSUB_SERVICE_ACCOUNT_KEY_TEMP="${PUBSUB_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP}"
  else
    readonly CONTROL_PLANE_SERVICE_ACCOUNT_KEY_TEMP="${GOOGLE_APPLICATION_CREDENTIALS}"
    readonly PUBSUB_SERVICE_ACCOUNT_KEY_TEMP="${GOOGLE_APPLICATION_CREDENTIALS}"
  fi
}

# Setup resources common to all eventing tests.
function test_setup() {
  pubsub_setup || return 1
  gcp_broker_setup || return 1
  storage_setup || return 1
  scheduler_setup || return 1
  echo "Sleep 2 mins to wait for all resources to setup"
  sleep 120

  # Publish test images.
  publish_test_images
}

# Tear down tmp files which store the private key.
function knative_teardown() {
  if (( ! IS_PROW )); then
    rm "${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP}"
  fi
}

# Create resources required for the Control Plane setup.
function control_plane_setup() {
  # When not running on Prow we need to set up a service account for managing resources.
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount used by the Control Plane"
    init_control_plane_service_account "${E2E_PROJECT_ID}" "${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW}"
    gcloud iam service-accounts keys create "${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP}" \
      --iam-account="${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW}"@"${E2E_PROJECT_ID}".iam.gserviceaccount.com
  fi
  echo "Create the control plane secret"
  kubectl -n "${CONTROL_PLANE_NAMESPACE}" create secret generic "${CONTROL_PLANE_SECRET_NAME}" --from-file=key.json="${CONTROL_PLANE_SERVICE_ACCOUNT_KEY_TEMP}"
  echo "Delete the controller pod in the namespace '${CONTROL_PLANE_NAMESPACE}' to refresh the created/patched secret"
  kubectl delete pod -n "${CONTROL_PLANE_NAMESPACE}" --selector role=controller
  wait_until_pods_running "${CONTROL_PLANE_NAMESPACE}" || return 1
}

# Create resources required for Pub/Sub Admin setup.
function pubsub_setup() {
  # If the tests are run on Prow, clean up the topics and subscriptions before running them.
  # See https://github.com/google/knative-gcp/issues/494
  if (( IS_PROW )); then
    delete_topics_and_subscriptions
  fi

  # When not running on Prow we need to set up a service account for PubSub
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount for Pub/Sub Admin"
    init_pubsub_service_account "${E2E_PROJECT_ID}" "${PUBSUB_SERVICE_ACCOUNT_NON_PROW}"
    enable_monitoring "${E2E_PROJECT_ID}" "${PUBSUB_SERVICE_ACCOUNT_NON_PROW}"
    gcloud iam service-accounts keys create "${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}" \
      --iam-account="${PUBSUB_SERVICE_ACCOUNT_NON_PROW}"@"${E2E_PROJECT_ID}".iam.gserviceaccount.com
  fi
  kubectl -n ${E2E_TEST_NAMESPACE} create secret generic "${PUBSUB_SECRET_NAME}" --from-file=key.json="${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}"
}

# Create resources required for GCP Broker authentication setup.
function gcp_broker_setup() {
  echo "Authentication setup for GCP Broker"
  kubectl -n "${CONTROL_PLANE_NAMESPACE}" create secret generic "${GCP_BROKER_SECRET_NAME}" --from-file=key.json="${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}"

  warmup_broker_setup
}

