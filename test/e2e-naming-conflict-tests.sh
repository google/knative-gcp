#!/usr/bin/env bash

# Copyright 2020 The Knative Authors
# Modified work Copyright 2020 Google LLC
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

# Script entry point.
# Make sure resources installed by knative-gcp don't overwrite
# resources installed by other components like knative eventing
# and serving

export GO111MODULE=on

source $(dirname "${BASH_SOURCE[0]}")/lib.sh

source $(dirname "${BASH_SOURCE[0]}")/../hack/lib.sh

source $(dirname "${BASH_SOURCE[0]}")/e2e-common.sh

# Overrides
function knative_setup {
  before_installing_knative_gcp || return 1
}

function test_setup {
  # Nothing to install before tests
  true
}

function test_teardown {
  # Nothing to uninstall after tests
  true
}

initialize $@

TIMEOUT=${TIMEOUT:-30m}

go_test_e2e \
  -tags=conflict \
  -timeout="${TIMEOUT}" \
  ./test/conflict \
  || fail_test

success
