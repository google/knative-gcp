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

# Usage: ./init_data_plane.sh [NAMESPACE]
#  where [NAMESPACE] is an optional parameter to specify the namespace to use. If it's not specified, we use the default one.
#                    if the namespace does not exist, the script will create it.
# The current project set in gcloud MUST be the same as where the cluster is running.
# The script always uses the same service account called cre-pubsub.

source $(dirname $0)/init_data_plane_common.sh

readonly PUBSUB_SERVICE_ACCOUNT="cre-pubsub"
readonly PUBSUB_SECRET_NAME="google-cloud-key"
readonly PUBSUB_SERVICE_ACCOUNT_KEY_TEMP="cre-pubsub.json"
readonly PROJECT_ID=$(gcloud config get-value project)
readonly NAMESPACE="default"

if [[ -z "$1" ]]; then
    echo "NAMESPACE not provided, using default"
  else
    NAMESPACE="$1"
    echo "NAMESPACE provided, using ${NAMESPACE}"
    kubectl create namespace $NAMESPACE
fi
init_pubsub_service_account ${PROJECT_ID} ${PUBSUB_SERVICE_ACCOUNT}

# Download a JSON key for the service account.
gcloud iam service-accounts keys create ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP} \
  --iam-account=${PUBSUB_SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com

# Create/Patch the secret with the download JSON key in the data plane namespace
kubectl --namespace ${NAMESPACE} create secret generic ${PUBSUB_SECRET_NAME} \
  --from-file=key.json=${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP} --dry-run -o yaml | kubectl apply --filename -

# Remove the tmp file.
rm ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}