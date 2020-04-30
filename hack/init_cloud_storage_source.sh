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

# Usage: ./init_cloud_storage_source.sh
# The current project set in gcloud MUST be the same as where the cluster is running.
source $(dirname $0)/lib.sh

PUBSUB_SERVICE_ACCOUNT_KEY_TEMP="cre-pubsub.json"

# Download a JSON key for the service account.
gcloud iam service-accounts keys create ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP} \
  --iam-account=${PUBSUB_SERVICE_ACCOUNT}@${PROJECT_ID}.iam.gserviceaccount.com

storage_admin_set_up ${PROJECT_ID} ${PUBSUB_SERVICE_ACCOUNT} ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}

# Remove the tmp file.
rm ${PUBSUB_SERVICE_ACCOUNT_KEY_TEMP}