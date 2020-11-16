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

# Usage: ./init_data_plane.sh [NAMESPACE] [SECRET] [PROJECT_ID]
#  [NAMESPACE] is an optional parameter to specify the namespace to use, default to `default`. If the namespace does not exist, the script will create it.
#  [SECRET] is an optional parameter to specify the secret, default to `google-cloud-key`. If the secret does not exist, the script will create it.
#  [PROJECT_ID] is an optional parameter to specify the project to use, default to `gcloud config get-value project`.
#  If user wants to specify PROJECT_ID, user also need to specify NAMESPACE, SECRET.
# The script always uses the same data plane google service account called cre-dataplane.
set -o errexit
set -o nounset
set -euo pipefail

source $(dirname "$0")/lib.sh

DATA_PLANE_SERVICE_ACCOUNT="cre-dataplane"
DATA_PLANE_SERVICE_ACCOUNT_KEY_TEMP="$(mktemp)"
DEFAULT_NAMESPACE="default"

echo "Start exporting service account key and storing it as a k8s secret..."

NAMESPACE=${1:-$DEFAULT_NAMESPACE}
 # Create the namespace for the data plane if it doesn't exist
existing_namespace=$(kubectl get namespace "${NAMESPACE}" --ignore-not-found)
  if [ -z "${existing_namespace}" ]; then
    echo "NAMESPACE'${NAMESPACE}' doesn't exist, create it"
    kubectl create namespace "${NAMESPACE}"
  fi
echo "NAMESPACE '${NAMESPACE}' is used for this configuration"

SECRET=${2:-$PUBSUB_SECRET_NAME}
echo "SECRET '${SECRET}' is used"
PROJECT_ID=${3:-$(gcloud config get-value project)}
echo "PROJECT_ID '${PROJECT_ID}' is used for this configuration"

# Download a JSON key for the service account.
gcloud iam service-accounts keys create "${DATA_PLANE_SERVICE_ACCOUNT_KEY_TEMP}" \
  --iam-account="${DATA_PLANE_SERVICE_ACCOUNT}"@"${PROJECT_ID}".iam.gserviceaccount.com

# Create/Patch the secret with the download JSON key in the data plane namespace
kubectl --namespace "${NAMESPACE}" create secret generic "${SECRET}" \
  --from-file=key.json="${DATA_PLANE_SERVICE_ACCOUNT_KEY_TEMP}" --dry-run -o yaml | kubectl apply --filename -

# Remove the tmp file.
rm "${DATA_PLANE_SERVICE_ACCOUNT_KEY_TEMP}"
