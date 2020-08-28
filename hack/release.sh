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

source $(dirname "$0")/../vendor/knative.dev/test-infra/scripts/release.sh

# Yaml files to generate, and the source config dir for them.
declare -A COMPONENTS
COMPONENTS=(
  ["cloud-run-events-core.yaml"]="config"
  ["cloud-run-events-pre-install-jobs.yaml"]="config/pre-install/v0.18.0"
)
readonly COMPONENTS

declare -A RELEASES
RELEASES=(
  ["cloud-run-events.yaml"]="cloud-run-events-core.yaml"
)
readonly RELEASES

function build_release() {
  # Update release labels if this is a tagged release
  if [[ -n "${TAG}" ]]; then
    echo "Tagged release, updating release labels to events.cloud.google.com/release: \"${TAG}\""
    LABEL_YAML_CMD=(sed -e "s|events.cloud.google.com/release: devel|events.cloud.google.com/release: \"${TAG}\"|")
  else
    echo "Untagged release, will NOT update release labels"
    LABEL_YAML_CMD=(cat)
  fi

  # Build the components
  local all_yamls=()
  for yaml in "${!COMPONENTS[@]}"; do
    local config="${COMPONENTS[${yaml}]}"
    echo "Building Cloud Run Events Components - ${config}"
    ko resolve --strict "${KO_FLAGS}" -f "${config}"/ | "${LABEL_YAML_CMD[@]}" > "${yaml}"
    all_yamls+=(${yaml})
  done
  # Assemble the release
  for yaml in "${!RELEASES[@]}"; do
    echo "Assembling Cloud Run Events - ${yaml}"
    echo "" > "${yaml}"
    for component in ${RELEASES[${yaml}]}; do
      echo "---" >> "${yaml}"
      echo "# ${component}" >> "${yaml}"
      cat "${component}" >> "${yaml}"
    done
    all_yamls+=(${yaml})
  done
  ARTIFACTS_TO_PUBLISH="${all_yamls[@]}"
}

main "$@"
