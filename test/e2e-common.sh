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

# This script includes common functions for testing setup and teardown.

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

source $(dirname $0)/lib.sh

# random6 returns 6 random letters.
function random6() {
  go run github.com/google/knative-gcp/test/cmd/randstr/ --length=6
}

# If gcloud is not available make it a no-op, not an error.
which gcloud &> /dev/null || gcloud() { echo "[ignore-gcloud $*]" 1>&2; }

# Eventing main config.
readonly E2E_TEST_NAMESPACE="default"
readonly CONTROL_PLANE_NAMESPACE="cloud-run-events"

# Constants used for creating ServiceAccount for the Control-Plane if it's not running on Prow.
readonly CONTROL_PLANE_SERVICE_ACCOUNT="e2e-cloud-run-events-test-$(random6)"
readonly CONTROL_PLANE_SERVICE_ACCOUNT_KEY="$(mktemp)"
readonly CONTROL_PLANE_SECRET_NAME="google-cloud-key"

# Constants used for creating ServiceAccount for Pub/Sub Admin if it's not running on Prow.
readonly PUBSUB_SERVICE_ACCOUNT="e2e-pubsub-test-$(random6)"
readonly PUBSUB_SERVICE_ACCOUNT_KEY="$(mktemp)"
readonly PUBSUB_SECRET_NAME="google-cloud-key"
global GCS_SERVICE_ACCOUNT

function knative_setup() {
  control_plane_setup || return 1
  start_knative_gcp
}

# Setup resources common to all eventing tests.
function test_setup() {
  pubsub_setup || return 1
  storage_setup || return 1
  echo "Sleep 2 min to wait for all resources to setup"
  sleep 120
  # TODO: Publish test images.
  # echo ">> Publishing test images"
  # $(dirname $0)/upload-test-images.sh e2e || fail_test "Error uploading test images"
}

# Tear down resources common to all eventing tests.
function test_teardown() {
  teardown
}

# Create resources required for the Control Plane setup
function control_plane_setup() {
  local service_account_key="${GOOGLE_APPLICATION_CREDENTIALS}"
  # When not running on Prow we need to set up a service account for managing resources
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount used by the Control Plane"
    gcloud iam service-accounts create ${CONTROL_PLANE_SERVICE_ACCOUNT}
    # TODO give finer grained permissions.
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/editor
    gcloud iam service-accounts keys create ${CONTROL_PLANE_SERVICE_ACCOUNT_KEY} \
      --iam-account=${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    service_account_key="${CONTROL_PLANE_SERVICE_ACCOUNT_KEY}"
  fi
  echo "Create the control plane secret"
  kubectl -n ${CONTROL_PLANE_NAMESPACE} create secret generic ${CONTROL_PLANE_SECRET_NAME} --from-file=key.json=${service_account_key}
}

# Create resources required for Pub/Sub Admin setup
function pubsub_setup() {
  local service_account_key="${GOOGLE_APPLICATION_CREDENTIALS}"
  # Enable monitoring
  gcloud services enable monitoring
  # When not running on Prow we need to set up a service account for PubSub
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount for Pub/Sub Admin"
    gcloud services enable pubsub.googleapis.com
    gcloud iam service-accounts create ${PUBSUB_SERVICE_ACCOUNT}
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.editor
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
    --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
    --role roles/monitoring.editor
    gcloud iam service-accounts keys create ${PUBSUB_SERVICE_ACCOUNT_KEY} \
      --iam-account=${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    service_account_key="${PUBSUB_SERVICE_ACCOUNT_KEY}"
  fi
  kubectl -n ${E2E_TEST_NAMESPACE} create secret generic ${PUBSUB_SECRET_NAME} --from-file=key.json=${service_account_key}
}

# Create resources required for Storage Admin setup
function storage_setup() {
  if (( ! IS_PROW )); then
    echo "Update ServiceAccount for Storage Admin"
    gcloud services enable storage-component.googleapis.com
    gcloud services enable storage-api.googleapis.com
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/storage.admin
    GCS_SERVICE_ACCOUNT=`curl -s -X GET -H "Authorization: Bearer \`GOOGLE_APPLICATION_CREDENTIALS=${PUBSUB_SERVICE_ACCOUNT_KEY} gcloud auth application-default print-access-token\`" "https://www.googleapis.com/storage/v1/projects/${E2E_PROJECT_ID}/serviceAccount" | grep email_address | cut -d '"' -f 4`
    echo $GCS_SERVICE_ACCOUNT
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${GCS_SERVICE_ACCOUNT} \
      --role roles/pubsub.publisher
  fi
}

# Delete resources that were used for setup
function teardown() {
  # When not running on Prow we need to delete the service account created
  if (( ! IS_PROW )); then
    echo "Tear down ServiceAccount for Pub/Sub Admin"
    gcloud iam service-accounts keys delete -q ${PUBSUB_SERVICE_ACCOUNT_KEY} \
      --iam-account=${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.editor
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
    --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
    --role roles/monitoring.editor
    echo "Tear down ServiceAccount for Storage Admin"
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/storage.admin
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${GCS_SERVICE_ACCOUNT} \
      --role roles/pubsub.publisher
    gcloud iam service-accounts delete -q ${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    echo "Tear down ServiceAccount for Control Plane"
    gcloud iam service-accounts keys delete -q ${CONTROL_PLANE_SERVICE_ACCOUNT_KEY} \
      --iam-account=${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/editor
  fi
  kubectl -n ${E2E_TEST_NAMESPACE} delete secret ${PUBSUB_SECRET_NAME}
  # TODO delete control plane secret?
}

