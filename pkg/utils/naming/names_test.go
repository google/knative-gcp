/*
Copyright 2020 Google LLC

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

package naming

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
)

const (
	testUID          = "11186600-4003-4ad6-90e7-22780053debf"
	truncatedNameMax = 146
)

var (
	maxName      = strings.Repeat("n", K8sNameMax)
	maxNamespace = strings.Repeat("s", K8sNamespaceMax)
)

func TestTruncatedPubsubResourceName(t *testing.T) {
	testCases := []struct {
		prefix string
		ns     string
		n      string
		uid    string
		want   string
	}{{
		prefix: "cre-obj",
		ns:     "default",
		n:      "default",
		uid:    testUID,
		want:   fmt.Sprintf("cre-obj_default_default_%s", testUID),
	}, {
		prefix: "cre-obj",
		ns:     "with-dashes",
		n:      "more-dashes",
		uid:    testUID,
		want:   fmt.Sprintf("cre-obj_with-dashes_more-dashes_%s", testUID),
	}, {
		prefix: "cre-obj",
		ns:     maxNamespace,
		n:      maxName,
		uid:    testUID,
		want:   fmt.Sprintf("cre-obj_%s_%s_%s", maxNamespace, strings.Repeat("n", truncatedNameMax), testUID),
	}, {
		prefix: "cre-obj",
		ns:     "default",
		n:      maxName,
		uid:    testUID,
		want:   fmt.Sprintf("cre-obj_default_%s_%s", strings.Repeat("n", truncatedNameMax+(K8sNamespaceMax-7)), testUID),
	}}

	for _, tc := range testCases {
		got := TruncatedPubsubResourceName(tc.prefix, tc.ns, tc.n, types.UID(tc.uid))
		if len(got) > PubsubMax {
			t.Errorf("name length %d is greater than %d", len(got), PubsubMax)
		}
		if diff := cmp.Diff(tc.want, got); diff != "" {
			t.Errorf("unexpected (-want, +got) = %v", diff)
		}
	}
}
