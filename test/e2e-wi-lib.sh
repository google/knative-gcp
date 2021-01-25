#!/usr/bin/env bash

# Copyright 2021 Google LLC
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

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

source $(dirname "${BASH_SOURCE[0]}")/../hack/lib.sh

source $(dirname "${BASH_SOURCE[0]}")/e2e-common.sh

function export_variable() {
  readonly MEMBER="serviceAccount:${E2E_PROJECT_ID}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${K8S_CONTROLLER_SERVICE_ACCOUNT}]"
  readonly BROKER_SERVICE_ACCOUNT="broker"
  readonly BROKER_MEMBER="serviceAccount:${E2E_PROJECT_ID}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${BROKER_SERVICE_ACCOUNT}]"

  readonly K8S_SERVICE_ACCOUNT_NAME="ksa-name"
  readonly CONFIG_GCP_AUTH="test/test_configs/config-gcp-auth-wi.yaml"

  if (( ! IS_PROW )); then
    readonly CONTROLLER_GSA_EMAIL="${CONTROLLER_GSA_NON_PROW}@${E2E_PROJECT_ID}.iam.gserviceaccount.com"
    readonly BROKER_GSA_EMAIL="${BROKER_GSA_NON_PROW}@${E2E_PROJECT_ID}.iam.gserviceaccount.com"
    readonly SOURCES_GSA_EMAIL="${SOURCES_GSA_NON_PROW}@${E2E_PROJECT_ID}.iam.gserviceaccount.com"
  else
    # Get the PROW service account & project name.
    readonly PROW_SERVICE_ACCOUNT_EMAIL=$(gcloud config get-value core/account)
    readonly PROW_PROJECT_NAME=$(cut -d'.' -f1 <<< "$(cut -d'@' -f2 <<< "${PROW_SERVICE_ACCOUNT_EMAIL}")")

    # Set the GSA emails.
    readonly CONTROLLER_GSA_EMAIL=${PROW_SERVICE_ACCOUNT_EMAIL}
    readonly SOURCES_GSA_EMAIL="cloud-run-events-source@${PROW_PROJECT_NAME}.iam.gserviceaccount.com"
    readonly BROKER_GSA_EMAIL=${PROW_SERVICE_ACCOUNT_EMAIL}
  fi
}

# Setup resources common to all eventing tests.
function test_setup() {
  wi_controller_auth_setup || return 1
  wi_sources_auth_setup || return 1

  # Authentication check test for BrokerCell. It is used in integration test in workload identity mode.
  # We do not put it in the same place as other integration tests, because this test can not run in parallel with others,
  # as this test requires the entire BrokerCell to be non-functional.
  if [[ -v ENABLE_AUTH_CHECK_TEST && $ENABLE_AUTH_CHECK_TEST == "true" ]]; then
    test_authentication_check_for_brokercell "workload_identity" || return 1
  fi

  wi_broker_auth_setup || return 1
  storage_setup || return 1
  scheduler_setup || return 1
  echo "Sleep 2 mins to wait for all resources to setup"
  sleep 120

  # Publish test images.
  publish_test_images
}

function wi_controller_auth_setup() {
  # When not running on Prow we need to set up a service account for managing resources.
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount used by the Control Plane"
    init_controller_gsa "${E2E_PROJECT_ID}" "${CONTROLLER_GSA_NON_PROW}"

    # Enable workload identity
    local cluster_name="$(cut -d'_' -f4 <<<"$(kubectl config current-context)")"
    local cluster_location="$(cut -d'_' -f3 <<<"$(kubectl config current-context)")"
    enable_workload_identity "${E2E_PROJECT_ID}" "${CONTROLLER_GSA_NON_PROW}" "${cluster_name}" "${cluster_location}" "${REGIONAL_CLUSTER_LOCATION_TYPE}"

	  # Allow the Kubernetes service account to use Google service account
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member "${MEMBER}" "${CONTROLLER_GSA_EMAIL}"

    kubectl annotate --overwrite serviceaccount "${K8S_CONTROLLER_SERVICE_ACCOUNT}" \
      iam.gke.io/gcp-service-account="${CONTROLLER_GSA_EMAIL}" --namespace "${CONTROL_PLANE_NAMESPACE}"
  else
    cleanup_iam_policy_binding_members

    # Allow the Kubernetes service account to use Google service account.
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member "${MEMBER}" \
      --project "${PROW_PROJECT_NAME}" "${CONTROLLER_GSA_EMAIL}"

    kubectl annotate --overwrite serviceaccount "${K8S_CONTROLLER_SERVICE_ACCOUNT}" \
      iam.gke.io/gcp-service-account="${CONTROLLER_GSA_EMAIL}" --namespace "${CONTROL_PLANE_NAMESPACE}"
  fi

  # Setup default credential information for Workload Identity.
  sed "s/K8S_SERVICE_ACCOUNT_NAME/${K8S_SERVICE_ACCOUNT_NAME}/g; s/SOURCES-GOOGLE-SERVICE-ACCOUNT/${SOURCES_GSA_EMAIL}/g" ${CONFIG_GCP_AUTH} | ko apply -f -

  wait_until_pods_running "${CONTROL_PLANE_NAMESPACE}" || return 1
}

# Create resources required for Sources authentication setup.
function wi_sources_auth_setup() {
  if (( ! IS_PROW )); then
    # When not running on Prow we need to set up a service account for sources.
    echo "Set up the Sources ServiceAccount"
    init_gsa_with_pubsub_editor "${E2E_PROJECT_ID}" "${SOURCES_GSA_NON_PROW}"
    enable_monitoring "${E2E_PROJECT_ID}" "${SOURCES_GSA_NON_PROW}"
  else
    delete_topics_and_subscriptions
  fi
}

# Create resources required for Broker authentication setup.
function wi_broker_auth_setup() {
  echo "Authentication setup for GCP Broker"

  if (( ! IS_PROW )); then
    echo "Set up the Broker ServiceAccount"
    init_gsa_with_pubsub_editor "${E2E_PROJECT_ID}" "${BROKER_GSA_NON_PROW}"
    enable_monitoring "${E2E_PROJECT_ID}" "${BROKER_GSA_NON_PROW}"

    # Allow the Kubernetes service account to use Google service account.
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member "${BROKER_MEMBER}" "${BROKER_GSA_EMAIL}"
  else
    # Allow the Kubernetes service account to use Google service account.
    gcloud iam service-accounts add-iam-policy-binding \
      --role roles/iam.workloadIdentityUser \
      --member "${BROKER_MEMBER}" \
      --project "${PROW_PROJECT_NAME}" "${BROKER_GSA_EMAIL}"
  fi

  kubectl annotate --overwrite serviceaccount ${BROKER_SERVICE_ACCOUNT} \
    iam.gke.io/gcp-service-account="${BROKER_GSA_EMAIL}" --namespace "${CONTROL_PLANE_NAMESPACE}"

  warmup_broker_setup || true
}

function apply_invalid_auth() {
  kubectl -n "${CONTROL_PLANE_NAMESPACE}" annotate sa broker iam.gke.io/gcp-service-account=fakeserviceaccount@test-project.iam.gserviceaccount.com
}

function delete_invalid_auth() {
  kubectl -n "${CONTROL_PLANE_NAMESPACE}" annotate sa broker iam.gke.io/gcp-service-account-
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
