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

# Usage: ./init_data_plane_gke.sh [MODE] [NAMESPACE] [K8S_SERVICE_ACCOUNT] [PROJECT_ID]
#  [MODE] is an optional parameter to specify the mode to use, default to `default`.
#  [NAMESPACE] is an optional parameter to specify the namespace to use, default to `default`. If the namespace does not exist, the script will create it.
#  [K8S_SERVICE_ACCOUNT] is an optional parameter to specify the k8s service account to use, default to `default-cre-dataplane`. If the k8s service account does not exist, the script will create it.
#  [PROJECT_ID] is an optional parameter to specify the project to use, default to `gcloud config get-value project`.
#  If user wants to specify PROJECT_ID, user also need to specify MODE, NAMESPACE and K8S_SERVICE_ACCOUNT.
# The script always uses the same data plane Google service account called cre-dataplane and control plane Google service account called cloud-run-events.
set -o errexit
set -o nounset
set -euo pipefail

source $(dirname "$0")/lib.sh

DATA_PLANE_SERVICE_ACCOUNT="cre-dataplane"

DEFAULT_MODE="default"
DEFAULT_NAMESPACE="default"
DEFAULT_K8S_SERVICE_ACCOUNT="default-cre-dataplane"

echo "Start configuring workload identity..."

MODE=${1:-$DEFAULT_MODE}
echo "MODE '${MODE}' is used for this configuration"

NAMESPACE=${2:-$DEFAULT_NAMESPACE}
K8S_SERVICE_ACCOUNT=${3:-$DEFAULT_K8S_SERVICE_ACCOUNT}
PROJECT_ID=${4:-$(gcloud config get-value project)}
echo "PROJECT_ID ${PROJECT_ID} is used for this configuration"

if [[ $MODE == "default" ]]; then
  # Grant iam.serviceAccountAdmin permission of the Google service account cre-dataplane
  # to the Control Plane's Google service account cloud-run-events.
  gcloud iam service-accounts add-iam-policy-binding ${DATA_PLANE_SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com  \
  --member=serviceAccount:${CONTROL_PLANE_SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com \
  --role roles/iam.serviceAccountAdmin

# Modify ConfigMap config-gcp-auth.
  kubectl patch configmap --namespace ${CONTROL_PLANE_NAMESPACE} config-gcp-auth --type=merge --patch "data:
  default-auth-config: |
    clusterDefaults:
      serviceAccountName: ${K8S_SERVICE_ACCOUNT}
      workloadIdentityMapping:
        ${K8S_SERVICE_ACCOUNT}: ${DATA_PLANE_SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com
  "
elif [[ $MODE = "non-default" ]]; then
   # Create the namespace for the data plane if it doesn't exist.
  existing_namespace=$(kubectl get namespace "${NAMESPACE}" --ignore-not-found)
  if [ -z "${existing_namespace}" ]; then
    echo "NAMESPACE'${NAMESPACE}' doesn't exist, create it"
    kubectl create namespace "${NAMESPACE}"
  fi
  echo "NAMESPACE '${NAMESPACE}' is used for this configuration"
  # Create the k8s service account for the namespace if it doesn't exist.
  existing_ksa=$(kubectl get sa "${K8S_SERVICE_ACCOUNT}" -n "${NAMESPACE}" --ignore-not-found)
  if [ -z "${existing_ksa}" ]; then
    echo "K8S Service Account '${K8S_SERVICE_ACCOUNT}' doesn't exist, create it"
    kubectl create serviceaccount "${K8S_SERVICE_ACCOUNT}" -n "${NAMESPACE}"
  fi
  echo "K8S Service Account Name ${K8S_SERVICE_ACCOUNT} is used for this configuration"

  # Allow the k8s service account to use Google service account cre-dataplane.
  MEMBER="serviceAccount:${PROJECT_ID}.svc.id.goog[${NAMESPACE}/${K8S_SERVICE_ACCOUNT}]"
  gcloud iam service-accounts add-iam-policy-binding \
    --role roles/iam.workloadIdentityUser \
    --member "$MEMBER" "${DATA_PLANE_SERVICE_ACCOUNT}"@"${PROJECT_ID}".iam.gserviceaccount.com

  # Add annotation to k8s service account.
  kubectl annotate --overwrite serviceaccount "${K8S_SERVICE_ACCOUNT}" iam.gke.io/gcp-service-account="${DATA_PLANE_SERVICE_ACCOUNT}"@"${PROJECT_ID}".iam.gserviceaccount.com \
    --namespace "${NAMESPACE}"
else
  echo "Invalid MODE ${MODE}"
fi
