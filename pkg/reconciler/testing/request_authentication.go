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

package testing

import (
	istiosecurityclient "istio.io/client-go/pkg/apis/security/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RequestAuthnOption func(*istiosecurityclient.RequestAuthentication)

func NewRequestAuthentication(name, namespace string, opts ...RequestAuthnOption) *istiosecurityclient.RequestAuthentication {
	r := &istiosecurityclient.RequestAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			// OwnerReferences: []metav1.OwnerReference{ownerRef(parent)},
		},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}
