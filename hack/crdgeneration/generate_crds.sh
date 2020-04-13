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

function crdGeneration() {
  local file="$1"
  go run ${REPO_ROOT_DIR}/hack/crdgeneration/main.go \
    --file="${file}" \
    > "${file//.gofmt/}"
}
function allCrdGeneration() {
  trap "$(shopt -p globstar)" RETURN
  shopt -s globstar
  local file=""
  for file in ${REPO_ROOT_DIR}/config/**/*.gofmt.yaml; do
    crdGeneration "${file}"
  done
}

# Generate our CRD YAMLs.
allCrdGeneration
