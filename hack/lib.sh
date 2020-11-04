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

readonly CONTROL_PLANE_SERVICE_ACCOUNT="cloud-run-events"
readonly CONTROL_PLANE_NAMESPACE="cloud-run-events"

readonly PUBSUB_SERVICE_ACCOUNT="cre-pubsub"

readonly SERVICE_ACCOUNT_EMAIL_KEY="EMAIL"

readonly ZONAL_CLUSTER_LOCATION_TYPE="zonal"
readonly REGIONAL_CLUSTER_LOCATION_TYPE="regional"

# Constants used for both init_XXX.sh and e2e-xxx.sh
export K8S_CONTROLLER_SERVICE_ACCOUNT="controller"
export CONTROL_PLANE_SECRET_NAME="google-cloud-key"
export PUBSUB_SECRET_NAME="google-cloud-key"

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
    gcloud iam service-accounts create "${control_plane_service_account}"
  else
    echo "Service Account needed for the Control Plane '${control_plane_service_account}' already existed"
  fi

  # Grant permissions to the service account for the control plane to manage GCP resources.
  echo "Set up Service Account used by the Control Plane"
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${control_plane_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/pubsub.admin
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${control_plane_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/storage.admin
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${control_plane_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/cloudscheduler.admin
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${control_plane_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/logging.configWriter
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${control_plane_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/logging.privateLogViewer

}

function init_pubsub_service_account() {
  local project_id=${1}
  local pubsub_service_account=${2}
  echo "parameter project_id used when initiating pubsub service account is'${project_id}'"
  echo "parameter data_plane_service_account used when initiating pubsub service account is'${pubsub_service_account}'"
  # Enable APIs.
  gcloud services enable pubsub.googleapis.com

  # Create the pubsub service account for the data plane if it doesn't exist
  existing_pubsub_service_account=$(gcloud iam service-accounts list \
    --filter="${SERVICE_ACCOUNT_EMAIL_KEY} ~ ^${pubsub_service_account}@")
  if [ -z "${existing_pubsub_service_account}" ]; then
    echo "Create PubSub Service Account '${pubsub_service_account}' neeeded for the Data Plane"
    gcloud iam service-accounts create "${pubsub_service_account}"
  else
    echo "PubSub Service Account '${pubsub_service_account}' needed for the Data Plane already existed"
  fi

  # Grant pubsub.editor role to the service account for the data plane to read and/or write to Pub/Sub.
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${pubsub_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/pubsub.editor

}

function enable_workload_identity(){
  local project_id=${1}
  local control_plane_service_account=${2}
  local cluster_name=${3}
  local cluster_location=${4}
  local cluster_location_type=${5}

  # Print and Verify parameters
  echo "parameter project_id used when enabling workload identity is'${project_id}'"
  echo "parameter control_plane_service_account used when enabling workload identity is'${control_plane_service_account}'"
  echo "parameter cluster_name used when enabling workload identity is'${cluster_name}'"
  echo "parameter cluster_location used when enabling workload identity is'${cluster_location}'"
  echo "parameter cluster_location_type used when enabling workload identity is'${cluster_location_type}'"

  local cluster_location_option
  if [[ ${cluster_location_type} == "${ZONAL_CLUSTER_LOCATION_TYPE}" ]]; then
    cluster_location_option=zone
  elif [[ ${cluster_location_type} == "${REGIONAL_CLUSTER_LOCATION_TYPE}" ]]; then
    cluster_location_option=region
  else
    echo >&2 "Fatal error: cluster_location_type used when enabling workload identity must be '${ZONAL_CLUSTER_LOCATION_TYPE}' or '${REGIONAL_CLUSTER_LOCATION_TYPE}'"
    exit 1
  fi
  echo "cluster_location_option used when enabling workload identity is'${cluster_location_option}'"

  # Enable API
  gcloud services enable iamcredentials.googleapis.com
  # Enable workload identity.
  echo "Enable Workload Identity"
  gcloud container clusters update "${cluster_name}" \
    --${cluster_location_option}="${cluster_location}" \
    --workload-pool="${project_id}".svc.id.goog

  # Modify all node pools to enable GKE_METADATA.
   echo "Modify all node pools to enable GKE_METADATA"
  pools=$(gcloud container node-pools list --cluster="${cluster_name}" --${cluster_location_option}="${cluster_location}" --format="value(name)")
  while read -r pool_name
  do
  gcloud container node-pools update "${pool_name}" \
    --cluster="${cluster_name}" \
    --${cluster_location_option}="${cluster_location}" \
    --workload-metadata=GKE_METADATA
  done <<<"${pools}"

  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${control_plane_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/iam.serviceAccountAdmin
  }

function storage_admin_set_up() {
  echo "Update ServiceAccount for Storage Admin"
  local project_id=${1}
  local pubsub_service_account=${2}

  echo "parameter project_id used when setting up storage admin is'${project_id}'"
  echo "parameter pubsub_service_account used when setting up storage admin is'${pubsub_service_account}'"
  echo "Update ServiceAccount for Storage Admin"
  gcloud services enable storage-component.googleapis.com
  gcloud services enable storage-api.googleapis.com
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member=serviceAccount:"${pubsub_service_account}"@"${project_id}".iam.gserviceaccount.com \
    --role roles/storage.admin

  # We assume the service account's name is in the form '<project-number>@gs-project-accounts.iam.gserviceaccount.com',
  # because it has been for all projects we've encountered. However, nothing requires this
  # format. To get the actual email, use this API:
  # https://cloud.google.com/storage/docs/json_api/v1/projects/serviceAccount/get/
  local project_number="$(gcloud projects describe ${project_id} --format="value(projectNumber)")"
  gcloud projects add-iam-policy-binding "${project_id}" \
    --member="serviceAccount:service-${project_number}@gs-project-accounts.iam.gserviceaccount.com" \
    --role roles/pubsub.publisher
}
