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

set -o errexit
set -o nounset
set -o pipefail

# Let sed work on mac.
if [ "$(uname)" == "Darwin" ]; then
  sed=gsed
elif [ "$(expr substr $(uname -s) 1 5)" == "Linux" ]; then
  sed=sed
fi

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/library.sh

CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${REPO_ROOT_DIR}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

KNATIVE_CODEGEN_PKG=${KNATIVE_CODEGEN_PKG:-$(cd ${REPO_ROOT_DIR}; ls -d -1 ./vendor/knative.dev/pkg 2>/dev/null || echo ../pkg)}

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/GoogleCloudPlatform/cloud-run-events/pkg/client github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis \
  "pubsub:v1alpha1 messaging:v1alpha1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

# Knative Injection
${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
  github.com/GoogleCloudPlatform/cloud-run-events/pkg/client github.com/GoogleCloudPlatform/cloud-run-events/pkg/apis \
  "pubsub:v1alpha1 messaging:v1alpha1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

# Because the kubernetes code generators force pacakges to lowercase, the update-deps script will be confused for
# imports of github.com/googlecloudplatform/... We will update that to use the correct casing in the generated code.
# The following will find all files (not directories, specified by -type f) under ${REPO_ROOT_DIR}/pkg/client, and
# perform a sed command to replace "googlecloudplatform" with "GoogleCloudPlatform" on each file it finds.

find ${REPO_ROOT_DIR}/pkg/client -type f -exec $sed -i 's/googlecloudplatform/GoogleCloudPlatform/g' {} \;

# Make sure our dependencies are up-to-date
${REPO_ROOT_DIR}/hack/update-deps.sh
