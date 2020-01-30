#!/usr/bin/env bash

# Copyright 2019 Google LLC
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

# Usage: ./init.sh [OPTIONAL: EXISTING_GSA_CONTROL_PLANE] [OPTIONAL: EXISTING_GSA_CONTROL_PLANE]
# If the first parameter is not provided, a default service account name ("cloud-run-events") will be used.
# If the second parameter is not provided, a default service account name ("cre-pubsub") will be used.
# The current project set in gcloud MUST be the same as where the cluster is running.

set -o errexit
set -o pipefail
set -o verbose

NAMESPACE=cloud-run-events
PROJECT_ID=$(gcloud config get-value project)
KEY_TEMP_CONTROL_PLANE=google-cloud-key.json
KEY_TEMP_DATA_PLANE=cre-pubsub.json

GSA_CONTROL_PLANE=cloud-run-events
if [[ -z "$1" ]]; then
  echo "GSA_CONTROL_PLANE not provided; will create GSA_CONTROL_PLANE ${GSA_CONTROL_PLANE} in project ${PROJECT_ID} instead."
  gcloud iam service-accounts create ${GSA_CONTROL_PLANE}
else
  echo "Will use provided GSA_CONTROL_PLANE ${GSA_CONTROL_PLANE}."
  GSA_CONTROL_PLANE="$1"
fi

# Grant owner role to the service account for the control plane to manage native GCP resources.
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA_CONTROL_PLANE}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/owner

# Download JSON key for the service account.
gcloud iam service-accounts keys create ${KEY_TEMP_CONTROL_PLANE} --iam-account=${GSA_CONTROL_PLANE}@${PROJECT_ID}.iam.gserviceaccount.com

# Create/Patch the secret with the download JSON key in the namespace
kubectl -n ${NAMESPACE} create secret generic google-cloud-key --from-file=key.json=${KEY_TEMP_CONTROL_PLANE} --dry-run -o yaml | kubectl apply --filename -

# Delete the controller pod in namespace to fetch the created/patched secret
kubectl delete pod -n cloud-run-events --selector role=controller

gcloud services enable pubsub.googleapis.com

GSA_DATA_PLANE=cre-pubsub
if [[ -z "$2" ]]; then
  echo "GSA_DATA_PLANE not provided; will create GSA_DATA_PLANE ${GSA_DATA_PLANE} in project ${PROJECT_ID} instead."
  gcloud iam service-accounts create ${GSA_DATA_PLANE}
else
  echo "Will use provided GSA_CONTROL_PLANE ${GSA_DATA_PLANE}."
  GSA_DATA_PLANE="$2"
fi

# Grant pubsub.editor role to the service account for the data plane to read and/or write to Pub/Sub.
gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:${GSA_DATA_PLANE}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/pubsub.editor

# # Download JSON key for the service account.
gcloud iam service-accounts keys create ${KEY_TEMP_DATA_PLANE} --iam-account=${GSA_DATA_PLANE}@${PROJECT_ID}.iam.gserviceaccount.com

#Create the secret with the download JSON key in the default namespace
kubectl --namespace default create secret generic google-cloud-key --from-file=key.json=${KEY_TEMP_DATA_PLANE}

# Label the namespace to inject broker.
kubectl label namespace default knative-eventing-injection=enabled

rm ${KEY_TEMP_CONTROL_PLANE}
rm ${KEY_TEMP_DATA_PLANE}
