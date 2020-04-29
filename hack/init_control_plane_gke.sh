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
source $(dirname $0)/init_control_plane_common.sh
source $(dirname $0)/init_control_plane_gke_lib.sh

readonly PROJECT_ID=$(gcloud config get-value project)
readonly CONTROL_PLANE_SERVICE_ACCOUNT=cloud-run-events-test
readonly K8S_CONTROLLER_SERVICE_ACCOUNT=controller

init_control_plane_service_account ${PROJECT_ID} ${CONTROL_PLANE_SERVICE_ACCOUNT}
enable_workload_identity ${PROJECT_ID} ${CONTROL_PLANE_SERVICE_ACCOUNT}


# Allow the Kubernetes service account to use Google service account.
MEMBER="serviceAccount:"${PROJECT_ID}".svc.id.goog["${CONTROL_PLANE_NAMESPACE}"/"${K8S_CONTROLLER_SERVICE_ACCOUNT}"]"
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member $MEMBER ${CONTROL_PLANE_SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com

# Add annotation to Kubernetes service account.
kubectl annotate --overwrite serviceaccount ${K8S_CONTROLLER_SERVICE_ACCOUNT} iam.gke.io/gcp-service-account=${CONTROL_PLANE_SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com \
  --namespace ${CONTROL_PLANE_NAMESPACE}

# Delete the controller pod in the control plane namespace to refresh
kubectl delete pod -n ${CONTROL_PLANE_NAMESPACE} --selector role=controller
