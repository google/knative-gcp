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

readonly SERVICE_ACCOUNT_EMAIL_KEY="EMAIL"

function init_pubsub_service_account() {
  local project_id=${1}
  local pubsub_service_account=${2}
  echo "parameter project_id used when initiating data plane service account is'${project_id}'"
  echo "parameter control_plane_service_account used when initiating pubsub service account is'${pubsub_service_account}'"
  # Enable APIs.
  gcloud services enable pubsub.googleapis.com

  # Create the pubsub service account for the data plane if it doesn't exist
  existing_pubsub_service_account=$(gcloud iam service-accounts list \
    --filter="${SERVICE_ACCOUNT_EMAIL_KEY} ~ ^${pubsub_service_account}@")
  if [ -z "${existing_pubsub_service_account}" ]; then
    echo "Create PubSub Service Account '${pubsub_service_account}' neeeded for the Data Plane"
    gcloud iam service-accounts create ${pubsub_service_account}
  else
    echo "PubSub Service Account needed for the Data Plane '${pubsub_service_account}' already existed"
  fi

  # Grant pubsub.editor role to the service account for the data plane to read and/or write to Pub/Sub.
  gcloud projects add-iam-policy-binding ${project_id} \
    --member=serviceAccount:${pubsub_service_account}@${project_id}.iam.gserviceaccount.com \
    --role roles/pubsub.editor

}
