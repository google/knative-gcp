/*
Copyright 2019 Google LLC

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

package resources

import (
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	servingv1alpha1 "knative.dev/serving/pkg/apis/serving/v1alpha1"
	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
)

// DecoratorArgs are the arguments needed to create a Topic publisher.
// Every field is required.
type DecoratorArgs struct {
	Image     string
	Decorator *v1alpha1.Decorator
	Labels    map[string]string
}

func MakeDecoratorExtensionsMap(base64Extensions string) map[string]string {

	quotedExtensions := strconv.Quote(string(base64Extensions))

	var byteExtensions []byte
	err := json.Unmarshal([]byte(quotedExtensions), &byteExtensions)
	if err != nil {
		return map[string]string{
			"error": err.Error(),
		}
	}

	var extensions map[string]string
	err = json.Unmarshal(byteExtensions, &extensions)
	if err != nil {
		return map[string]string{
			"error": err.Error(),
		}
	}

	return extensions
}

func MakeDecoratorExtensionsString(extensions map[string]string) string {
	jsonExtensions, err := json.Marshal(extensions)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}
	// if we json.Marshal a []byte, we will get back a base64 encoded quoted string.
	base64Extensions, err := json.Marshal(jsonExtensions)
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}

	extensionsString, err := strconv.Unquote(string(base64Extensions))
	if err != nil {
		return fmt.Sprintf(`{"error":"%s}`, err.Error())
	}
	// Turn the base64 encoded []byte back into a string.
	return extensionsString
}

func makeDecoratorPodSpec(args *DecoratorArgs) corev1.PodSpec {

	additions := MakeDecoratorExtensionsString(args.Decorator.Spec.Extensions)

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: args.Image,
			Env: []corev1.EnvVar{{
				Name:  "K_CE_EXTENSIONS",
				Value: additions,
			}, {
				Name:  "K_SINK",
				Value: args.Decorator.Status.SinkURI,
			}},
		}},
	}
	return podSpec
}

// MakeDecorator generates (but does not insert into K8s) the Decorator.
func MakeDecoratorV1alpha1(args *DecoratorArgs) *servingv1alpha1.Service {
	podSpec := makeDecoratorPodSpec(args)

	return &servingv1alpha1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Decorator.Namespace,
			Name:            GenerateDecoratorName(args.Decorator),
			Labels:          args.Labels,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Decorator)},
		},
		Spec: servingv1alpha1.ServiceSpec{
			ConfigurationSpec: servingv1alpha1.ConfigurationSpec{
				Template: &servingv1alpha1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: args.Labels,
					},
					Spec: servingv1alpha1.RevisionSpec{
						RevisionSpec: servingv1beta1.RevisionSpec{
							PodSpec: podSpec,
						},
					},
				},
			},
		},
	}
}
