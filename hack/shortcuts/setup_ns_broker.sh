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

# Usage: ./setup_ns_broker.sh [REQUIRED: EXISTING_NAMESPACE] [OPTIONAL: EXISTING_GSA]
# If the second parameter is not provided, a default service account name ("cloud-run-events") will be used.
# The current project set in gcloud MUST be the same as where the cluster is running.

set -o errexit
set -o pipefail
set -o verbose

if [[ -z "$1" ]]; then
  echo "Namespace not provided."
  exit 1
fi

NAMESPACE=$1
PROJECT_ID=$(gcloud config get-value project)
KEY_TEMP=google-cloud-key.json

GSA=cloud-run-events
if [[ -z "$2" ]]; then
  echo "GSA not provided; will create GSA ${GSA} in project ${PROJECT_ID} instead."
  gcloud iam service-accounts create ${GSA}
else
  echo "Will use provided GSA ${GSA}."
  GSA="$2"
fi

# Grant pubsub editor role to the service account.
gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${GSA}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/pubsub.editor

# Download JSON key for the service account.
gcloud iam service-accounts keys create ${KEY_TEMP} --iam-account=${GSA}@${PROJECT_ID}.iam.gserviceaccount.com

# Make sure namespace exist.
kubectl get namespace ${NAMESPACE}

# Create secret for the JSON key in the namespace.
kubectl --namespace ${NAMESPACE} create secret generic google-cloud-key --from-file=key.json=${KEY_TEMP}

# Label the namespace to inject broker.
kubectl label namespace ${NAMESPACE} knative-eventing-injection=enabled

rm ${KEY_TEMP}
