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

source $(dirname $0)/e2e-common.sh

# Override the setup and teardown functions to install wi-enabled control plane components.
readonly K8S_CONTROLLER_SERVICE_ACCOUNT="controller"
readonly PROW_SERVICE_ACCOUNT=$(gcloud config get-value core/account)

if (( ! IS_PROW )); then
  readonly AUTHENTICATED_SERVICE_ACCOUNT="${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com"
  readonly MEMBER="serviceAccount:${E2E_PROJECT_ID}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${K8S_CONTROLLER_SERVICE_ACCOUNT}]"
  readonly DATA_PLANE_SERVICE_ACCOUNT="${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com"
else
  readonly AUTHENTICATED_SERVICE_ACCOUNT=${PROW_SERVICE_ACCOUNT}
  readonly MEMBER="serviceAccount:${PROJECT}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${K8S_CONTROLLER_SERVICE_ACCOUNT}]"
  # Get the PROW service account.
  readonly PROW_PROJECT_NAME=$(cut -d'.' -f1 <<< $(cut -d'@' -f2 <<< ${AUTHENTICATED_SERVICE_ACCOUNT}))
  readonly DATA_PLANE_SERVICE_ACCOUNT=${AUTHENTICATED_SERVICE_ACCOUNT}
fi

# Create resources required for the Control Plane setup.
function knative_setup() {
  control_plane_setup || return 1
  start_knative_gcp "workloadIdentityEnabled" || return 1
  kubectl annotate serviceaccount ${K8S_CONTROLLER_SERVICE_ACCOUNT} iam.gke.io/gcp-service-account=${AUTHENTICATED_SERVICE_ACCOUNT} \
    --namespace ${CONTROL_PLANE_NAMESPACE}
}

function control_plane_setup() {
  # When not running on Prow we need to set up a service account for managing resources.
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount used by the Control Plane"
    # Enable iamcredentials.googleapis.com service for Workload Identity.
    gcloud services enable iamcredentials.googleapis.com
    gcloud iam service-accounts create ${CONTROL_PLANE_SERVICE_ACCOUNT}
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.admin
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.editor
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/storage.admin
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/cloudscheduler.admin
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/logging.configWriter
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/logging.privateLogViewer
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/cloudscheduler.admin
        # Give iam.serviceAccountAdmin role to the Google service account.
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/iam.serviceAccountAdmin
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member ${MEMBER} ${AUTHENTICATED_SERVICE_ACCOUNT}
  else
    # If the tests are run on Prow, clean up the member for roles/iam.workloadIdentityUser before running it.
    members=$(gcloud iam service-accounts get-iam-policy \
      --project=${PROW_PROJECT_NAME} ${AUTHENTICATED_SERVICE_ACCOUNT} \
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
          --project ${PROW_PROJECT_NAME} ${AUTHENTICATED_SERVICE_ACCOUNT}
      fi
    done <<< "$members"
    # Allow the Kubernetes service account to use Google service account.
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member ${MEMBER} \
      --project ${PROW_PROJECT_NAME} ${AUTHENTICATED_SERVICE_ACCOUNT}
  fi
}

# Create resources required for Pub/Sub Admin setup.
function pubsub_setup() {
  # If the tests are run on Prow, clean up the topics and subscriptions before running them.
  # See https://github.com/google/knative-gcp/issues/494
  if (( IS_PROW )); then
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
  fi

  # When not running on Prow we need to set up a service account for PubSub.
  if (( ! IS_PROW )); then
    # Enable monitoring
    gcloud services enable monitoring
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
}

# Tear down resources required for Control Plane setup.
function control_plane_teardown() {
  # When not running on Prow we need to delete the service accounts and namespaces created.
  if (( ! IS_PROW )); then
    echo "Tear down ServiceAccount for Control Plane"
    gcloud iam service-accounts keys delete -q ${CONTROL_PLANE_SERVICE_ACCOUNT_KEY} \
      --iam-account=${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.admin
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.editor
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/storage.admin
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/cloudscheduler.admin
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/logging.configWriter
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/logging.privateLogViewer
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/cloudscheduler.admin
    # Remove iam.serviceAccountAdmin role to the Google service account.
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/iam.serviceAccountAdmin
    gcloud iam service-accounts remove-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member ${MEMBER} ${AUTHENTICATED_SERVICE_ACCOUNT}
  else
    gcloud iam service-accounts remove-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member ${MEMBER} \
      --project ${PROW_PROJECT_NAME} ${AUTHENTICATED_SERVICE_ACCOUNT}
  fi
}

# Create a cluster with Workload Identity enabled.
initialize $@ --cluster-creation-flag "--workload-pool=\${PROJECT}.svc.id.goog"

# Channel related e2e tests we have in Eventing is not running here.
go_test_e2e -timeout=30m -parallel=6 ./test/e2e -workloadIndentity=true -pubsubServiceAccount=${DATA_PLANE_SERVICE_ACCOUNT} || fail_test

success
