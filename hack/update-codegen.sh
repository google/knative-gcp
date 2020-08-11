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

source $(dirname "$0")/../vendor/knative.dev/test-infra/scripts/library.sh

# Compute _example hash for all configmaps.
for file in "${REPO_ROOT_DIR}"/config/core/configmaps/*.yaml
do
  go run "${REPO_ROOT_DIR}/vendor/knative.dev/pkg/configmap/hash-gen" "$file"
done

CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${REPO_ROOT_DIR}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

KNATIVE_CODEGEN_PKG=${KNATIVE_CODEGEN_PKG:-$(cd "${REPO_ROOT_DIR}"; ls -d -1 ./vendor/knative.dev/pkg 2>/dev/null || echo ../pkg)}

PUBSUBAPICOPY=(
	"implements_test.go"
	"pullsubscription_defaults.go"
	"pullsubscription_defaults_test.go"
	"pullsubscription_lifecycle.go"
	"pullsubscription_lifecycle_test.go"
	"pullsubscription_types.go"
	"pullsubscription_validation.go"
	"pullsubscription_validation_test.go"
	"topic_defaults.go"
	"topic_defaults_test.go"
	"topic_lifecycle.go"
	"topic_lifecycle_test.go"
	"topic_types.go"
	"topic_validation.go"
	"topic_validation_test.go"
)

chmod +x "${CODEGEN_PKG}"/generate-groups.sh
# Only deepcopy the Duck types, as they are not real resources.
"${CODEGEN_PKG}"/generate-groups.sh "deepcopy" \
  github.com/google/knative-gcp/pkg/client github.com/google/knative-gcp/pkg/apis \
  "duck:v1alpha1 duck:v1beta1 duck:v1" \
  --go-header-file "${REPO_ROOT_DIR}"/hack/boilerplate/boilerplate.go.txt

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
"${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/google/knative-gcp/pkg/client github.com/google/knative-gcp/pkg/apis \
  "messaging:v1alpha1 messaging:v1beta1 events:v1alpha1 events:v1beta1 events:v1 broker:v1beta1 intevents:v1alpha1 intevents:v1beta1 intevents:v1" \
  --go-header-file "${REPO_ROOT_DIR}"/hack/boilerplate/boilerplate.go.txt

# Knative Injection
chmod +x "${KNATIVE_CODEGEN_PKG}"/hack/generate-knative.sh
"${KNATIVE_CODEGEN_PKG}"/hack/generate-knative.sh "injection" \
  github.com/google/knative-gcp/pkg/client github.com/google/knative-gcp/pkg/apis \
  "messaging:v1alpha1 messaging:v1beta1 events:v1alpha1 events:v1beta1 events:v1 duck:v1alpha1 duck:v1beta1 duck:v1 broker:v1beta1 intevents:v1alpha1 intevents:v1beta1 intevents:v1" \
  --go-header-file "${REPO_ROOT_DIR}"/hack/boilerplate/boilerplate.go.txt

# Deep copy configs.
${GOPATH}/bin/deepcopy-gen \
  -O zz_generated.deepcopy \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt \
  -i github.com/google/knative-gcp/pkg/apis/configs/gcpauth \
  -i github.com/google/knative-gcp/pkg/apis/configs/broker \

# TODO(yolocs): generate autoscaling v2beta2 in knative/pkg.
OUTPUT_PKG="github.com/google/knative-gcp/pkg/client/injection/kube" \
VERSIONED_CLIENTSET_PKG="k8s.io/client-go/kubernetes" \
EXTERNAL_INFORMER_PKG="k8s.io/client-go/informers" \
"${KNATIVE_CODEGEN_PKG}"/hack/generate-knative.sh "injection" \
  k8s.io/client-go \
  k8s.io/api \
  "autoscaling:v2beta2" \
  --go-header-file "${REPO_ROOT_DIR}"/hack/boilerplate/boilerplate.go.txt

go install github.com/google/wire/cmd/wire
go generate "${REPO_ROOT_DIR}"/...

# Make sure our dependencies are up-to-date
"${REPO_ROOT_DIR}"/hack/update-deps.sh
