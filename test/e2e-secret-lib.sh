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

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

source $(dirname "${BASH_SOURCE[0]}")/../hack/lib.sh

source $(dirname "${BASH_SOURCE[0]}")/e2e-common.sh

# Eventing main config.
readonly E2E_TEST_NAMESPACE="default"

# Constants used for creating ServiceAccount for the Controllers GSA if it's not running on Prow.
readonly CONTROLLER_GSA_NON_PROW_KEY_TEMP="$(mktemp)"

# Constants used for creating ServiceAccount for the Sources if it's not running on Prow.
readonly SOURCES_GSA_NON_PROW_KEY_TEMP="$(mktemp)"

# Constants used for creating ServiceAccount for the Broker if it's not running on Prow.
readonly BROKER_GSA_NON_PROW_KEY_TEMP="$(mktemp)"

# Constants used for authentication setup for GCP Broker if it's not running on Prow.
readonly BROKER_GSA_SECRET_NAME="google-broker-key"

function export_variable() {
  if (( ! IS_PROW )); then
    readonly CONTROLLER_GSA_KEY_TEMP="${CONTROLLER_GSA_NON_PROW_KEY_TEMP}"
    readonly SOURCES_GSA_KEY_TEMP="${SOURCES_GSA_NON_PROW_KEY_TEMP}"
    readonly BROKER_GSA_KEY_TEMP="${BROKER_GSA_NON_PROW_KEY_TEMP}"
  else
    readonly CONTROLLER_GSA_KEY_TEMP="${GOOGLE_APPLICATION_CREDENTIALS}"
    readonly SOURCES_GSA_KEY_TEMP="${GOOGLE_APPLICATION_CREDENTIALS}"
    readonly BROKER_GSA_KEY_TEMP="${GOOGLE_APPLICATION_CREDENTIALS}"
  fi
}

# Setup resources common to all eventing tests.
function test_setup() {
  sources_auth_setup "secret" || return 1

  # Authentication check test for BrokerCell. It is used in integration test in secret mode.
  # We do not put it in the same place as other integration tests, because this test can not run in parallel with others,
  # as this test requires the entire BrokerCell to be non-functional.
  if [[ -v ENABLE_AUTH_CHECK_TEST && $ENABLE_AUTH_CHECK_TEST == "true" ]]; then
    test_authentication_check_for_brokercell "secret" || return 1
  fi

  broker_auth_setup "secret" || return 1
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
    rm "${CONTROLLER_GSA_NON_PROW_KEY_TEMP}"
  fi
}

# Create resources required for the Control Plane setup.
function control_plane_setup() {
  # When not running on Prow we need to set up a service account for managing resources.
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount used by the Control Plane"
    init_controller_gsa "${E2E_PROJECT_ID}" "${CONTROLLER_GSA_NON_PROW}"
    gcloud iam service-accounts keys create "${CONTROLLER_GSA_NON_PROW_KEY_TEMP}" \
      --iam-account="${CONTROLLER_GSA_NON_PROW}"@"${E2E_PROJECT_ID}".iam.gserviceaccount.com
  fi
  prow_control_plane_setup "secret"
  wait_until_pods_running "${CONTROL_PLANE_NAMESPACE}" || return 1
}

