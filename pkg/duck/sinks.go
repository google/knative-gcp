/*
Copyright 2018 The Knative Authors

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

package duck

import (
	"context"
	"fmt"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	duckapis "github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

// TODO: This should up upstreamed into knative/pkg.

// GetSinkURI retrieves the sink URI from the object referenced by the given
// ObjectReference.
func GetSinkURI(ctx context.Context, dynamicClient dynamic.Interface, sink *corev1.ObjectReference, namespace string) (string, error) {
	if sink == nil {
		return "", fmt.Errorf("sink ref is nil")
	}

	// K8s Services are special cased. They can be called, even though they do not satisfy the
	// Callable interface.
	if sink.APIVersion == "v1" && sink.Kind == "Service" {
		if sink.Namespace != "" {
			return DomainToURL(ServiceHostName(sink.Name, sink.Namespace)), nil
		}
		return DomainToURL(ServiceHostName(sink.Name, namespace)), nil
	}

	rc := dynamicClient.Resource(duckapis.KindToResource(sink.GroupVersionKind()))
	if rc == nil {
		return "", fmt.Errorf("failed to create dynamic client resource")
	}

	u, err := rc.Namespace(namespace).Get(sink.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	objIdentifier := fmt.Sprintf("\"%s/%s\" (%s)", u.GetNamespace(), u.GetName(), u.GroupVersionKind())

	t := duckv1alpha1.AddressableType{}
	err = duck.FromUnstructured(u, &t)
	if err != nil {
		return "", fmt.Errorf("failed to deserialize sink %s: %v", objIdentifier, err)
	}

	if t.Status.Address == nil {
		return "", fmt.Errorf("sink %s does not contain address", objIdentifier)
	}

	if t.Status.Address.Hostname == "" {
		return "", fmt.Errorf("sink %s contains an empty hostname", objIdentifier)
	}

	return fmt.Sprintf("http://%s/", t.Status.Address.Hostname), nil
}

// DomainToURL converts a domain into an HTTP URL.
func DomainToURL(domain string) string {
	u := url.URL{
		Scheme: "http",
		Host:   domain,
		Path:   "/",
	}
	return u.String()
}

// ServiceHostName creates the hostname for a Kubernetes Service.
func ServiceHostName(serviceName, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.%s", serviceName, namespace, GetClusterDomainName())
}
