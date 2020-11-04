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
source "${ROOT_DIR}"/vendor/knative.dev/hack/library.sh

set -o errexit
set -o nounset
set -o pipefail

go_update_deps "$@"

# Patch k8s leader-election fixing graceful release
# More information: https://github.com/kubernetes/kubernetes/pull/91942
git apply ${ROOT_DIR}/hack/k8s-client-go.patch
