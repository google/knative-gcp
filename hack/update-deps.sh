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

readonly ROOT_DIR=$(dirname "$0")/..
source "${ROOT_DIR}"/vendor/knative.dev/test-infra/scripts/library.sh

set -o errexit
set -o nounset
set -o pipefail

cd "${ROOT_DIR}"

# Used to pin floating deps to a release version.
VERSION="release-0.18"
# The list of dependencies that we track at HEAD and periodically
# float forward in this repository.
FLOATING_DEPS=(
  "knative.dev/pkg@$VERSION"
  "knative.dev/serving@$VERSION"
  "knative.dev/eventing@$VERSION"
  "knative.dev/test-infra@$VERSION"
)

# Parse flags to determine any we should pass to dep.
GO_GET=0
while [[ $# -ne 0 ]]; do
  parameter=$1
  case ${parameter} in
    --upgrade) GO_GET=1 ;;
    *) abort "unknown option ${parameter}" ;;
  esac
  shift
done
readonly GO_GET

if (( GO_GET )); then
  go get -d ${FLOATING_DEPS[@]}
fi

# Prune modules.
go mod tidy
go mod vendor

find vendor/ \( -name OWNERS -o -name OWNERS_ALIASES -o -name BUILD -o -name BUILD.bazel \) -delete

update_licenses third_party/VENDOR-LICENSE "./..."

# Patch k8s leader-election fixing graceful release
# More information: https://github.com/kubernetes/kubernetes/pull/91942
git apply ${ROOT_DIR}/hack/k8s-client-go.patch
