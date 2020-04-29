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

# Usage: ./init_control_plane_gke.sh
# The current project set in gcloud MUST be the same as where the cluster is running.

function enable_workload_identity(){
  local project_id=${1}
  local control_plane_service_account=${2}

  echo "parameter project_id used when enabling workload identity is'${project_id}'"
  echo "parameter control_plane_service_account used when when enabling workload identity is'${control_plane_service_account}'"
  # Enable API
  gcloud services enable iamcredentials.googleapis.com
  # Enable workload identity.
  echo "Enable Workload Identity"
  local cluster_name="$(cut -d'_' -f4 <<<"$(kubectl config current-context)")"
  gcloud container clusters update ${cluster_name} \
    --workload-pool=${PROJECT_ID}.svc.id.goog

  # Modify all node pools to enable GKE_METADATA.
   echo "Modify all node pools to enable GKE_METADATA"
  pools=$(gcloud container node-pools list --cluster=${cluster_name} --format="value(name)")
  while read -r pool_name
  do
  gcloud container node-pools update "${pool_name}" \
    --cluster=${cluster_name} \
    --workload-metadata=GKE_METADATA
  done <<<"${pools}"

  gcloud projects add-iam-policy-binding ${project_id} --member=serviceAccount:${control_plane_service_account}@${project_id}.iam.gserviceaccount.com --role roles/iam.serviceAccountAdmin
  }
