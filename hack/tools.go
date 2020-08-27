// +build tools

/*
Copyright 2020 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package tools imports tool dependencies
package tools

import (
	_ "knative.dev/pkg/configmap/hash-gen"
	_ "knative.dev/pkg/hack"

	_ "knative.dev/eventing/test/test_images/event-sender"
	_ "knative.dev/eventing/test/test_images/recordevents"
	_ "knative.dev/eventing/test/test_images/transformevents"

	_ "knative.dev/eventing/test/test_images/performance"
	_ "knative.dev/pkg/apiextensions/storageversion/cmd/migrate"
	_ "knative.dev/pkg/test/mako/stub-sidecar"
	_ "knative.dev/pkg/testutils/clustermanager/perf-tests"

	_ "github.com/google/wire/cmd/wire"
)
