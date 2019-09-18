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
readonly CLOUD_RUN_EVENTS_CONFIG="config/"
readonly E2E_TEST_NAMESPACE="default"

# Constants used for creating ServiceAccount for Pub/Sub Admin if it's not running on Prow.
readonly PUBSUB_SERVICE_ACCOUNT="e2e-pubsub-test-$(random6)"
readonly PUBSUB_SERVICE_ACCOUNT_KEY="$(mktemp)"
readonly PUBSUB_SECRET_NAME="google-cloud-key"
global GCS_SERVICE_ACCOUNT

# Setup the Cloud Run Events environment for running tests.
function cloud_run_events_setup() {
  # Install the latest Cloud Run Events in the current cluster.
  echo ">> Starting Cloud Run Events"
  echo "Installing Cloud Run Events"
  ko apply -f ${CLOUD_RUN_EVENTS_CONFIG} || return 1
  wait_until_pods_running cloud-run-events || fail_test "Cloud Run Events did not come up"
}

function knative_setup() {
  start_latest_knative_serving
  start_latest_knative_eventing
  cloud_run_events_setup
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

# Create resources required for Pub/Sub Admin setup
function pubsub_setup() {
  local service_account_key="${GOOGLE_APPLICATION_CREDENTIALS}"
  # When not running on Prow we need to set up a service account for PubSub
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount for Pub/Sub Admin"
    gcloud services enable pubsub.googleapis.com
    gcloud iam service-accounts create ${PUBSUB_SERVICE_ACCOUNT}
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.editor
    gcloud iam service-accounts keys create ${PUBSUB_SERVICE_ACCOUNT_KEY} \
      --iam-account=${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    service_account_key="${PUBSUB_SERVICE_ACCOUNT_KEY}"
  fi
  kubectl -n ${E2E_TEST_NAMESPACE} create secret generic ${PUBSUB_SECRET_NAME} --from-file=key.json=${service_account_key}
}

# Create resources required for Storage Admin setu
function storage_setup() {
  if (( ! IS_PROW )); then
    echo "Update ServiceAccount for Storage Admin"
    gcloud services enable storage-component.googleapis.com
    gcloud services enable storage-api.googleapis.com
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/storage.admin
  fi
  GCS_SERVICE_ACCOUNT=`curl -s -X GET -H "Authorization: Bearer \`GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS} gcloud auth application-default print-access-token\`" "https://www.googleapis.com/storage/v1/projects/${E2E_PROJECT_ID}/serviceAccount" | grep email_address | cut -d '"' -f 4`
  echo "###################"
  echo $GCS_SERVICE_ACCOUNT
  echo "###################"
  gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
    --member=serviceAccount:${GCS_SERVICE_ACCOUNT} \
    --role roles/pubsub.publisher
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
    echo "Tear down ServiceAccount for Storage Admin"
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/storage.admin
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${GCS_SERVICE_ACCOUNT} \
      --role roles/pubsub.publisher
    gcloud iam service-accounts delete -q ${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
  fi
  kubectl -n ${E2E_TEST_NAMESPACE} delete secret ${PUBSUB_SECRET_NAME}
}
