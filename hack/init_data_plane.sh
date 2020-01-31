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

# Usage: ./init_data_plane.sh [NAMESPACE]
#  where [NAMESPACE] is an optional parameter to specify the namespace to use. If it's not specified, we use the default one.
#                    if the namespace does not exist, the script will create it.
# The current project set in gcloud MUST be the same as where the cluster is running.
# The script always uses the same service account called cre-pubsub.

SERVICE_ACCOUNT=cre-pubsub
KEY_TEMP=google-cloud-key.json
NAMESPACE=default
if [[ -z "$1" ]]; then
  echo "NAMESPACE not provided, using default"
else
  NAMESPACE="$1"
  echo "NAMESPACE provided, using ${NAMESPACE}"
  kubectl create namespace $NAMESPACE
fi

# Enable the pubsub APIs
gcloud services enable pubsub.googleapis.com

# Create the service account for the data plane
gcloud iam service-accounts create ${SERVICE_ACCOUNT}

# Grant pubsub.editor role to the service account for the data plane to read and/or write to Pub/Sub.
gcloud projects add-iam-policy-binding $PROJECT_ID --member=serviceAccount:${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com --role roles/pubsub.editor

# Download a JSON key for the service account.
gcloud iam service-accounts keys create ${KEY_TEMP} --iam-account=${SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com

# Create the secret with the download JSON key.
kubectl --namespace $NAMESPACE create secret generic google-cloud-key --from-file=key.json=${KEY_TEMP}

# Label the namespace to inject a Broker.
kubectl label namespace $NAMESPACE knative-eventing-injection=enabled

# Remove the tmp file.
rm ${KEY_TEMP}
