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

NAMESPACE=cloud-run-events
GSA_NAME=cloud-run-events
PROJECT_ID=$(gcloud config get-value project)
CLUSTER_NAME="$(cut -d'_' -f4 <<<"$(kubectl config current-context)")"
KSA_NAME=controller

# Enable APIs.
gcloud services enable pubsub.googleapis.com
gcloud services enable storage-component.googleapis.com
gcloud services enable storage-api.googleapis.com
gcloud services enable cloudscheduler.googleapis.com
gcloud services enable logging.googleapis.com
gcloud services enable stackdriver.googleapis.com
gcloud services enable iamcredentials.googleapis.com

# Enable workload identity.
gcloud beta container clusters update ${CLUSTER_NAME} \
  --identity-namespace=${PROJECT_ID}.svc.id.goog

# Create the service account for the control plane.
gcloud iam service-accounts create ${GSA_NAME}

# Grant permissions to the service account for the control plane to manage native GCP resources.
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/pubsub.admin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/storage.admin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/cloudscheduler.admin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/logging.configWriter
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/logging.privateLogViewer
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/iam.serviceAccountAdmin

# Allow the Kubernetes service account to use Google service account.
MEMBER="serviceAccount:"${PROJECT_ID}".svc.id.goog["${NAMESPACE}"/"${KSA_NAME}"]"
gcloud iam service-accounts add-iam-policy-binding --role roles/iam.workloadIdentityUser --member $MEMBER ${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com

# Add annotation to Kubernetes service account.
kubectl annotate serviceaccount --namespace ${NAMESPACE} ${KSA_NAME} iam.gke.io/gcp-service-account=${GSA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com