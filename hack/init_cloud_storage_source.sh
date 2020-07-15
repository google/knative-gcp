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

# Usage: ./init_cloud_storage_source.sh [PROJECT_ID]
#  [PROJECT_ID] is an optional parameter to specify the project to use, default to `gcloud config get-value project`.
# The script always uses the same service account called cre-pubsub.
set -o errexit
set -o nounset
set -euo pipefail

source $(dirname "$0")/lib.sh

PROJECT_ID=${1:-$(gcloud config get-value project)}
echo "PROJECT_ID used when init_cloud_storage_source is'${PROJECT_ID}'"

storage_admin_set_up "${PROJECT_ID}" "${PUBSUB_SERVICE_ACCOUNT}"
