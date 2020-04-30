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
source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

source $(dirname $0)/lib.sh

source $(dirname $0)/../hack/lib.sh

source $(dirname $0)/e2e-common.sh

readonly BROKER_SERVICE_ACCOUNT="broker"
readonly PROW_SERVICE_ACCOUNT_EMAIL=$(gcloud config get-value core/account)
# Constants used for creating ServiceAccount for Data Plane(Pub/Sub Admin) if it's not running on Prow.
readonly PUBSUB_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP="$(mktemp)"

function export_variable() {
if (( ! IS_PROW )); then
  readonly CONTROL_PLANE_SERVICE_ACCOUNT_EMAIL="${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW}@${E2E_PROJECT_ID}.iam.gserviceaccount.com"
  readonly MEMBER="serviceAccount:${E2E_PROJECT_ID}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${K8S_CONTROLLER_SERVICE_ACCOUNT}]"
  readonly BROKER_MEMBER="serviceAccount:${E2E_PROJECT_ID}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${BROKER_SERVICE_ACCOUNT}]"
  readonly PUBSUB_SERVICE_ACCOUNT_EMAIL="${PUBSUB_SERVICE_ACCOUNT_NON_PROW}@${E2E_PROJECT_ID}.iam.gserviceaccount.com"
  readonly PUBSUB_SERVICE_ACCOUNT_KEY_TEMP="${PUBSUB_SERVICE_ACCOUNT_NON_PROW_KEY_TEMP}"
else
  readonly CONTROL_PLANE_SERVICE_ACCOUNT_EMAIL=${PROW_SERVICE_ACCOUNT_EMAIL}
  readonly MEMBER="serviceAccount:${PROJECT}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${K8S_CONTROLLER_SERVICE_ACCOUNT}]"
  readonly BROKER_MEMBER="serviceAccount:${PROJECT}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${BROKER_SERVICE_ACCOUNT}]"
  # Get the PROW service account.
  readonly PROW_PROJECT_NAME=$(cut -d'.' -f1 <<< $(cut -d'@' -f2 <<< ${PROW_SERVICE_ACCOUNT_EMAIL}))
  readonly PUBSUB_SERVICE_ACCOUNT_EMAIL=${PROW_SERVICE_ACCOUNT_EMAIL}
  readonly PUBSUB_SERVICE_ACCOUNT_KEY_TEMP="${GOOGLE_APPLICATION_CREDENTIALS}"
fi
}

# Setup resources common to all eventing tests.
function test_setup() {
  pubsub_setup || return 1
  gcp_broker_setup || return 1
  # Create privacy key that will be used in storage_setup
  create_private_key_for_pubsub_service_account || return 1
  storage_setup || return 1
  scheduler_setup || return 1
  echo "Sleep 2 mins to wait for all resources to setup"
  sleep 120

  # Publish test images.
  publish_test_images
}

function control_plane_setup() {
  # When not running on Prow we need to set up a service account for managing resources.
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount used by the Control Plane"
    init_control_plane_service_account ${E2E_PROJECT_ID} ${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW}
    enable_workload_identity ${E2E_PROJECT_ID} ${CONTROL_PLANE_SERVICE_ACCOUNT_NON_PROW} ${REGIONAL_CLUSTER_LOCATION_TYPE}
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member ${MEMBER} ${CONTROL_PLANE_SERVICE_ACCOUNT_EMAIL}
  else
    # If the tests are run on Prow, clean up the member for roles/iam.workloadIdentityUser before running it.
    members=$(gcloud iam service-accounts get-iam-policy \
      --project=${PROW_PROJECT_NAME} ${CONTROL_PLANE_SERVICE_ACCOUNT_EMAIL} \
      --format="value(bindings.members)" \
      --filter="bindings.role:roles/iam.workloadIdentityUser" \
      --flatten="bindings[].members")
    while read -r member_name
    do
      # Only delete the iam bindings that is related to the current boskos project.
      if [ $(cut -d'.' -f1 <<< ${member_name}) == "serviceAccount:${PROJECT}" ]; then
        gcloud iam service-accounts remove-iam-policy-binding \
          --role roles/iam.workloadIdentityUser \
          --member ${member_name} \
          --project ${PROW_PROJECT_NAME} ${CONTROL_PLANE_SERVICE_ACCOUNT_EMAIL}
      fi
    done <<< "$members"
    # Allow the Kubernetes service account to use Google service account.
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member ${MEMBER} \
      --project ${PROW_PROJECT_NAME} ${CONTROL_PLANE_SERVICE_ACCOUNT_EMAIL}
  fi
  kubectl annotate --overwrite serviceaccount ${K8S_CONTROLLER_SERVICE_ACCOUNT} iam.gke.io/gcp-service-account=${CONTROL_PLANE_SERVICE_ACCOUNT_EMAIL} \
    --namespace ${CONTROL_PLANE_NAMESPACE}
  echo "Delete the controller pod in the namespace '${CONTROL_PLANE_NAMESPACE}' to refresh "
  kubectl delete pod -n ${CONTROL_PLANE_NAMESPACE} --selector role=controller
}

# Create resources required for Pub/Sub Admin setup.
function pubsub_setup() {
  # If the tests are run on Prow, clean up the topics and subscriptions before running them.
  # See https://github.com/google/knative-gcp/issues/494
  if (( IS_PROW )); then
    delete_topics_and_subscriptions
  fi

  # When not running on Prow we need to set up a service account for PubSub.
  if (( ! IS_PROW )); then
    # Enable monitoring
    gcloud services enable monitoring
    echo "Set up ServiceAccount for Pub/Sub Admin"
    init_pubsub_service_account ${E2E_PROJECT_ID} ${PUBSUB_SERVICE_ACCOUNT_NON_PROW}
    enable_monitoring ${E2E_PROJECT_ID} ${PUBSUB_SERVICE_ACCOUNT_NON_PROW}
  fi
}

# Create resources required for GCP Broker authentication setup.
function gcp_broker_setup() {
  echo "Authentication setup for GCP Broker"
  if (( ! IS_PROW )); then
    gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member ${BROKER_MEMBER} ${PUBSUB_SERVICE_ACCOUNT_EMAIL}
  else
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member ${BROKER_MEMBER} \
      --project ${PROW_PROJECT_NAME} ${PUBSUB_SERVICE_ACCOUNT_EMAIL}
  fi
  kubectl annotate --overwrite serviceaccount ${BROKER_SERVICE_ACCOUNT} iam.gke.io/gcp-service-account=${PUBSUB_SERVICE_ACCOUNT_EMAIL} \
    --namespace ${CONTROL_PLANE_NAMESPACE}
}

function create_private_key_for_pubsub_service_account {
  if (( ! IS_PROW )); then
    gcloud iam service-accounts keys create ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP} \
      --iam-account=${PUBSUB_SERVICE_ACCOUNT_EMAIL}
  fi
}

# Create a cluster with Workload Identity enabled.
initialize $@ --cluster-creation-flag "--workload-pool=\${PROJECT}.svc.id.goog"

# Channel related e2e tests we have in Eventing is not running here.
go_test_e2e -timeout=30m -parallel=6 ./test/e2e -workloadIndentity=true -pubsubServiceAccount=${PUBSUB_SERVICE_ACCOUNT_EMAIL} || fail_test

success
