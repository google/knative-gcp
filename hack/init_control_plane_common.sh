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

readonly CONTROL_PLANE_NAMESPACE="cloud-run-events"
readonly SERVICE_ACCOUNT_EMAIL_KEY="EMAIL"

function init_control_plane_service_account() {
  local project_id=${1}
  local control_plane_service_account=${2}

  echo "parameter project_id used when initiating control plane service account is'${project_id}'"
  echo "parameter control_plane_service_account used when initiating control plane service account is'${control_plane_service_account}'"

  # Enable APIs.
  gcloud services enable pubsub.googleapis.com
  gcloud services enable storage-component.googleapis.com
  gcloud services enable storage-api.googleapis.com
  gcloud services enable cloudscheduler.googleapis.com
  gcloud services enable cloudbuild.googleapis.com
  gcloud services enable logging.googleapis.com
  gcloud services enable stackdriver.googleapis.com
  # Create the service account for the control plane if it doesn't exist
  existing_control_plane_service_account=$(gcloud iam service-accounts list \
    --filter="${SERVICE_ACCOUNT_EMAIL_KEY} ~ ^${control_plane_service_account}@")
  if [ -z "${existing_control_plane_service_account}" ]; then
    echo "Create Service Account '${control_plane_service_account}' neeeded for the Control Plane"
    gcloud iam service-accounts create ${control_plane_service_account}
  else
    echo "Service Account needed for the Control Plane '${control_plane_service_account}' already existed"
  fi

  # Grant permissions to the service account for the control plane to manage native GCP resources.
  echo "Set up Service Account used by the Control Plane"
  gcloud projects add-iam-policy-binding ${project_id} \
    --member=serviceAccount:${control_plane_service_account}@${project_id}.iam.gserviceaccount.com \
    --role roles/pubsub.admin
  gcloud projects add-iam-policy-binding ${project_id} \
    --member=serviceAccount:${control_plane_service_account}@${project_id}.iam.gserviceaccount.com \
    --role roles/storage.admin
  gcloud projects add-iam-policy-binding ${project_id} \
    --member=serviceAccount:${control_plane_service_account}@${project_id}.iam.gserviceaccount.com \
    --role roles/cloudscheduler.admin
  gcloud projects add-iam-policy-binding ${project_id} \
    --member=serviceAccount:${control_plane_service_account}@${project_id}.iam.gserviceaccount.com \
    --role roles/logging.configWriter
  gcloud projects add-iam-policy-binding ${project_id} \
    --member=serviceAccount:${control_plane_service_account}@${project_id}.iam.gserviceaccount.com \
    --role roles/logging.privateLogViewer

}