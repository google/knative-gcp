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

package ingress

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

// BrokerPath returns the path to be set in the status of a broker.
// The format is brokerNamespace/brokerName
func BrokerPath(namespace, name string) string {
	return fmt.Sprintf("/%s/%s", namespace, name)
}

// convertPathToNamespacedName converts the broker path to a NamespaceName.
func ConvertPathToNamespacedName(path string) (types.NamespacedName, error) {
	// Path should be in the form of "/<ns>/<broker>".
	pieces := strings.Split(path, "/")
	if len(pieces) != 3 {
		return types.NamespacedName{}, fmt.Errorf("Malformed request path; expect format '/<ns>/<broker>'")
	}
	return types.NamespacedName{
		Namespace: pieces[1],
		Name:      pieces[2],
	}, nil
}
