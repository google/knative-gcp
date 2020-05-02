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

# Usage: ./init_control_plane_gke.sh [CLUSTER_NAME] [CLUSTER_LOCATION] [CLUSTER_LOCATION_TYPE] [PROJECT_ID]
#  where [CLUSTER_NAME] is an optional parameter to specify the cluster to use.
#  If CLUSTER_NAME not specified, we use the cluster set in gcloud(gcloud config get-value run/cluster_location).
#  where [CLUSTER_LOCATION] is an optional parameter to specify the cluster location to use.
#  If CLUSTER_LOCATION not specified, we use the cluster location set in gcloud(gcloud config get-value run/cluster_location).
#  where [CLUSTER_LOCATION_TYPE] is an optional parameter to specify the cluster location type to use.
#  CLUSTER_LOCATION_TYPE must be `zonal` or `regional`. If CLUSTER_LOCATION_TYPE not specified, we use the `zonal`
#  where [PROJECT_ID] is an optional parameter to specify the project to use.
#  If PROJECT_ID not specified, we use the project set in gcloud(gcloud config get-value project).
#  If user want to specify a parameter, user will also need `specify all parameters before that specific paramater
# The script always uses the same service account called cloud-run-events.
set -o errexit
set -o nounset
set -euo pipefail

source $(dirname $0)/lib.sh

readonly DEFAULT_CLUSTER_LOCATION_TYPE="zonal"

CLUSTER_NAME=${1:-$(gcloud config get-value run/cluster)}
CLUSTER_LOCATION=${2:-$(gcloud config get-value run/cluster_location)}
CLUSTER_LOCATION_TYPE=${3:-$DEFAULT_CLUSTER_LOCATION_TYPE}
PROJECT_ID=${4:-$(gcloud config get-value project)}

echo "CLUSTER_NAME used when init_control_plane_gke is'${CLUSTER_NAME}'"
echo "CLUSTER_LOCATION used when init_control_plane_gke is'${CLUSTER_LOCATION}'"
echo "CLUSTER_LOCATION_TYPE used when init_control_plane_gke is'${CLUSTER_LOCATION_TYPE}'"
echo "PROJECT_ID used when init_control_plane_gke is'${PROJECT_ID}'"

init_control_plane_service_account "${PROJECT_ID}" "${CONTROL_PLANE_SERVICE_ACCOUNT}"
enable_workload_identity "${PROJECT_ID}" "${CONTROL_PLANE_SERVICE_ACCOUNT}" "${CLUSTER_NAME}" "${CLUSTER_LOCATION}" "${CLUSTER_LOCATION_TYPE}"

# Allow the Kubernetes service account to use Google service account.
MEMBER="serviceAccount:${PROJECT_ID}.svc.id.goog[${CONTROL_PLANE_NAMESPACE}/${K8S_CONTROLLER_SERVICE_ACCOUNT}]"
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "$MEMBER" "${CONTROL_PLANE_SERVICE_ACCOUNT}"@"${PROJECT_ID}".iam.gserviceaccount.com

# Add annotation to Kubernetes service account.
kubectl annotate --overwrite serviceaccount "${K8S_CONTROLLER_SERVICE_ACCOUNT}" iam.gke.io/gcp-service-account="${CONTROL_PLANE_SERVICE_ACCOUNT}"@"${PROJECT_ID}".iam.gserviceaccount.com \
  --namespace "${CONTROL_PLANE_NAMESPACE}"

# Delete the controller pod in the control plane namespace to refresh
kubectl delete pod -n "${CONTROL_PLANE_NAMESPACE}" --selector role=controller
