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

# Usage: ./init_data_plane.sh [NAMESPACE] [PROJECT_ID]
#  [NAMESPACE] is an optional parameter to specify the namespace to use, default to `default`. If the namespace does not exist, the script will create it.
#  [PROJECT_ID] is an optional parameter to specify the project to use, default to `gcloud config get-value project`.
#  If user wants to sepcify PROJECT_ID, user also need to specify NAMESPACE
# The script always uses the same service account called cre-pubsub.
set -o errexit
set -o nounset
set -euo pipefail

source $(dirname "$0")/lib.sh

PUBSUB_SERVICE_ACCOUNT_KEY_TEMP="$(mktemp)"
DEFAULT_NAMESPACE="default"

NAMESPACE=${1:-$DEFAULT_NAMESPACE}
 # Create the namespace for the data plane if it doesn't exist
existing_namespace=$(kubectl get namespace "${NAMESPACE}")
  if [ -z "${existing_namespace}" ]; then
    echo "Create NAMESPACE'${NAMESPACE}' neeeded for the Data Plane"
    kubectl create namespace "${NAMESPACE}"
  else
    echo "NAMESPACE needed for the Data Plane '${NAMESPACE}' already existed"
  fi
echo "NAMESPACE used when init_data_plane is'${NAMESPACE}'"
PROJECT_ID=${2:-$(gcloud config get-value project)}
echo "PROJECT_ID used when init_data_plane is'${PROJECT_ID}'"

init_pubsub_service_account "${PROJECT_ID}" "${PUBSUB_SERVICE_ACCOUNT}"

# Download a JSON key for the service account.
gcloud iam service-accounts keys create "${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}" \
  --iam-account="${PUBSUB_SERVICE_ACCOUNT}"@"${PROJECT_ID}".iam.gserviceaccount.com

# Create/Patch the secret with the download JSON key in the data plane namespace
kubectl --namespace "${NAMESPACE}" create secret generic "${PUBSUB_SECRET_NAME}" \
  --from-file=key.json="${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}" --dry-run -o yaml | kubectl apply --filename -

# Remove the tmp file.
rm "${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}"