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
	"context"
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	"github.com/google/knative-gcp/pkg/apis/messaging/v1alpha1"
)

// DecoratorArgs are the arguments needed to create a Topic publisher.
// Every field is required.
type DecoratorArgs struct {
	Image     string
	Decorator *v1alpha1.Decorator
	Labels    map[string]string
}

func Base64ToMap(base64 string) (map[string]string, error) {
	if base64 == "" {
		return nil, errors.New("base64 map string is empty")
	}

	quotedBase64 := strconv.Quote(string(base64))

	var byteExtensions []byte
	err := json.Unmarshal([]byte(quotedBase64), &byteExtensions)
	if err != nil {
		return nil, err
	}

	var extensions map[string]string
	err = json.Unmarshal(byteExtensions, &extensions)
	if err != nil {
		return nil, err
	}

	return extensions, err
}

func MapToBase64(extensions map[string]string) (string, error) {
	if extensions == nil {
		return "", errors.New("map is nil")
	}

	jsonExtensions, err := json.Marshal(extensions)
	if err != nil {
		return "", err
	}
	// if we json.Marshal a []byte, we will get back a base64 encoded quoted string.
	base64Extensions, err := json.Marshal(jsonExtensions)
	if err != nil {
		return "", err
	}

	extensionsString, err := strconv.Unquote(string(base64Extensions))
	if err != nil {
		return "", err
	}
	// Turn the base64 encoded []byte back into a string.
	return extensionsString, nil
}

func makeDecoratorPodSpec(ctx context.Context, args *DecoratorArgs) corev1.PodSpec {
	ceExtensions, err := MapToBase64(args.Decorator.Spec.CloudEventOverrides.Extensions)
	if err != nil {
		logging.FromContext(ctx).Warnw("failed to make decorator extensions",
			zap.Error(err),
			zap.Any("extensions", args.Decorator.Spec.CloudEventOverrides.Extensions))
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Image: args.Image,
			Env: []corev1.EnvVar{{
				Name:  "K_CE_EXTENSIONS",
				Value: ceExtensions,
			}, {
				Name:  "K_SINK",
				Value: args.Decorator.Status.SinkURI,
			}},
		}},
	}
	return podSpec
}

// MakeDecorator generates (but does not insert into K8s) the Decorator.
func MakeDecoratorV1alpha1(ctx context.Context, args *DecoratorArgs) *servingv1.Service {
	podSpec := makeDecoratorPodSpec(ctx, args)

	return &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       args.Decorator.Namespace,
			Name:            GenerateDecoratorName(args.Decorator),
			Labels:          args.Labels,
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(args.Decorator)},
		},
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: args.Labels,
					},
					Spec: servingv1.RevisionSpec{
						PodSpec: podSpec,
					},
				},
			},
		},
	}
}
