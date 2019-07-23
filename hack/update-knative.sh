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

readonly ROOT_DIR=$(dirname $0)/..
source ${ROOT_DIR}/vendor/github.com/knative/test-infra/scripts/library.sh

set -o errexit
set -o nounset
set -o pipefail

cd ${ROOT_DIR}

onBranchEnsure () { # 1=branch 2=import
  local dep_branch=$1
  local dep_import=$2
  local IS_DEP_BRANCH=`grep ${dep_import} -B1 -A2 Gopkg.toml | \
     grep -E 'override|constraint' -A3 | \
     grep "$dep_branch"`

  if [ -z "$IS_DEP_BRANCH" ]
  then
    echo "💥: $dep_import not set to $dep_branch."
    grep '$dep_import' -B1 -A2 Gopkg.toml | \
       grep -E 'override|constraint' -A3
    exit
  else
    echo "✅ $dep_import set to $dep_branch."
  fi

  echo "🐎 dep ensure -update $dep_import"
  dep ensure -update $dep_import
}

onBranchEnsure master github.com/knative/serving
onBranchEnsure master github.com/knative/eventing

echo '🐎 ./hack/update-codegen.sh'
./hack/update-codegen.sh

echo "✨Done✨"
