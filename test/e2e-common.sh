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
readonly CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW="cloud-run-events"

# Constants used for creating ServiceAccount for Data Plane(Pub/Sub Admin) if it's not running on Prow.
readonly PUBSUB_SERVICE_ACCOUNT_NON_PROW="cre-pubsub"

# Vendored eventing test iamges.
readonly VENDOR_EVENTING_TEST_IMAGES="vendor/knative.dev/eventing/test/test_images/"

# Constants used for authentication setup for GCP Broker if it's not running on Prow.
readonly APP_ENGINE_REGION="us-central"

# Setup Knative GCP.
function knative_setup() {
  start_knative_gcp || return 1
  export_variable || return 1
  control_plane_setup || return 1
}

# Tear down tmp files which store the private key.
function test_teardown() {
  if (( ! IS_PROW )); then
    rm ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}
  fi
}

function publish_test_images() {
  # Publish test images.
  echo ">> Publishing test images"
  sed -i 's@ko://knative.dev/eventing/test/test_images@ko://github.com/google/knative-gcp/vendor/knative.dev/eventing/test/test_images@g' vendor/knative.dev/eventing/test/test_images/*/*.yaml
  $(dirname $0)/upload-test-images.sh ${VENDOR_EVENTING_TEST_IMAGES} e2e || fail_test "Error uploading test images from eventing"
  $(dirname $0)/upload-test-images.sh "test/test_images" e2e || fail_test "Error uploading test images from knative-gcp"
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
    storage_admin_set_up ${E2E_PROJECT_ID} ${PUBSUB_SERVICE_ACCOUNT_NON_PROW} ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}
  fi
}

function delete_topics_and_subscriptions() {
    subs=$(gcloud pubsub subscriptions list --format="value(name)")
    while read -r sub_name
    do
      gcloud pubsub subscriptions delete "${sub_name}"
    done <<<"$subs"
    topics=$(gcloud pubsub topics list --format="value(name)")
    while read -r topic_name
    do
      gcloud pubsub topics delete "${topic_name}"
    done <<<"$topics"
}

function enable_monitoring(){
  local project_id=${1}
  local pubsub_service_account=${2}

  echo "parameter project_id used when enabling monitoring is'${project_id}'"
  echo "parameter control_plane_service_account used when enabling monitoring is'${pubsub_service_account}'"
  # Enable monitoring
  echo "Enable Monitoring"
  gcloud services enable monitoring
  gcloud projects add-iam-policy-binding ${project_id} \
      --member=serviceAccount:${pubsub_service_account}@${project_id}.iam.gserviceaccount.com \
      --role roles/monitoring.editor
}

