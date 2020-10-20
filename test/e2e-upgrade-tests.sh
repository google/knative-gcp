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

# Docs -> file://./upgrade/README.md

# Script entry point.

source $(dirname "$0")/e2e-secret-tests.sh

readonly PROBER_READY_FILE="/tmp/prober-ready"
readonly PROBER_PIPE_FILE="/tmp/prober-signal"

# Overrides

function knative_setup {
  start_knative_gcp_from_latest_release || return 1
  export_variable || return 1
  control_plane_setup || return 1
}

function install_test_resources {
  # Nothing to install before tests
  true
}

function uninstall_test_resources {
  # Nothing to uninstall after tests
  true
}

initialize $@

TIMEOUT=${TIMEOUT:-30m}

header "Running preupgrade tests"

# knative eventing invokes test in go, but e2e test in knative gcp requires non trivial
# setup, so it is easier to run it directly.
go_test_e2e -tags=e2e -timeout="${TIMEOUT}" ./test/e2e -run=^TestSmokeGCPBroker || fail_test

header "Starting prober test"
rm -fv ${PROBER_READY_FILE}
go_test_e2e -tags=probe -timeout="${TIMEOUT}" ./test/upgrade --pipefile="${PROBER_PIPE_FILE}" --readyfile="${PROBER_READY_FILE}" &
PROBER_PID=$!
echo "Prober PID is ${PROBER_PID}"

wait_for_file ${PROBER_READY_FILE} || fail_test

header "Performing upgrade to HEAD"
install_cloud_run_events_from_head || fail_test 'Installing HEAD version failed'

header "Running postupgrade tests"
go_test_e2e -tags=e2e -timeout="${TIMEOUT}" ./test/e2e -run=^TestSmokeGCPBroker || fail_test

header "Performing downgrade to latest release"
install_cloud_run_events_from_latest_release || fail_test 'Installing latest release of Knative Eventing failed'

header "Running postdowngrade tests"
go_test_e2e -tags=e2e -timeout="${TIMEOUT}" ./test/e2e -run=^TestSmokeGCPBroker || fail_test

# The prober is blocking on ${PROBER_PIPE_FILE} to know when it should exit.
echo "done" > ${PROBER_PIPE_FILE}

header "Waiting for prober test"
wait ${PROBER_PID} || fail_test "Prober failed"

success
