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

	"k8s.io/apimachinery/pkg/types"
)

const (
	PubsubMax       = 255
	LoggingSinkMax  = 100
	K8sNamespaceMax = 63
	K8sNameMax      = 253
)

// TruncatedPubsubResourceName generates a deterministic name for a Pub/Sub resource.
// If the name would be longer than allowed by Pub/Sub, the name is truncated to fit.
func TruncatedPubsubResourceName(prefix, ns, n string, uid types.UID) string {
	return truncateResourceName(prefix, ns, n, uid, PubsubMax)
}

// TruncatedLoggingSinkResourceName generates a deterministic name for a StackDriver logging sink.
// If the name would be longer than allowed by StackDriver, the name is truncated to fit.
func TruncatedLoggingSinkResourceName(prefix, ns, n string, uid types.UID) string {
	return truncateResourceName(prefix, ns, n, uid, LoggingSinkMax)
}

func truncateResourceName(prefix, ns, n string, uid types.UID, maximum int) string {
	s := fmt.Sprintf("%s_%s_%s_%s", prefix, ns, n, string(uid))
	if len(s) <= maximum {
		return s
	}
	names := fmt.Sprintf("%s_%s_%s", prefix, ns, n)
	namesMax := maximum - ((len(string(uid))) + 1) // 1 for the uid separator

	return fmt.Sprintf("%s_%s", names[:namesMax], string(uid))
}
