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

# Usage: ./init_control_plane.sh
# The current project set in gcloud MUST be the same as where the cluster is running.

NAMESPACE=cloud-run-events
SERVICE_ACCOUNT=cloud-run-events
PROJECT_ID=$(gcloud config get-value project)
KEY_TEMP=google-cloud-key.json

# Enable APIs.
gcloud services enable pubsub.googleapis.com
gcloud services enable storage-component.googleapis.com
gcloud services enable storage-api.googleapis.com
gcloud services enable cloudscheduler.googleapis.com
gcloud services enable cloudbuild.googleapis.com
gcloud services enable logging.googleapis.com
gcloud services enable stackdriver.googleapis.com

# Create the service account for the control plane
gcloud iam service-accounts create ${SERVICE_ACCOUNT}

# Grant permissions to the service account for the control plane to manage native GCP resources.
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/pubsub.admin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/storage.admin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/cloudscheduler.admin
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/logging.configWriter
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/logging.privateLogViewer

# Download a JSON key for the service account.
gcloud iam service-accounts keys create ${KEY_TEMP} --iam-account=${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com

# Create/Patch the secret with the download JSON key in the control plane namespace
kubectl -n ${NAMESPACE} create secret generic google-cloud-key --from-file=key.json=${KEY_TEMP} --dry-run -o yaml | kubectl apply --filename -

# Delete the controller pod in the control plane namespace to refresh the created/patched secret
kubectl delete pod -n ${NAMESPACE} --selector role=controller

# Remove the tmp file.
rm ${KEY_TEMP}
