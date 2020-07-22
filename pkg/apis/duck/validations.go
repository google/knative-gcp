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

// Package duck contains Cloud Run Events API versions for duck components
package duck

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var (
	// The name of a k8s ServiceAccount object must be a valid DNS subdomain name.
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
	ksaValidationRegex = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`)
)

// ValidateAutoscalingAnnotations validates the autoscaling annotations.
// The class ensures that we reconcile using the corresponding controller.
func ValidateAutoscalingAnnotations(ctx context.Context, annotations map[string]string, errs *apis.FieldError) *apis.FieldError {
	if autoscalingClass, ok := annotations[AutoscalingClassAnnotation]; ok {
		// Only supported autoscaling class is KEDA.
		if autoscalingClass != KEDA {
			errs = errs.Also(apis.ErrInvalidValue(autoscalingClass, fmt.Sprintf("metadata.annotations[%s]", AutoscalingClassAnnotation)))
		}

		var minScale, maxScale int
		minScale, errs = validateAnnotation(annotations, AutoscalingMinScaleAnnotation, minimumMinScale, errs)
		maxScale, errs = validateAnnotation(annotations, AutoscalingMaxScaleAnnotation, minimumMaxScale, errs)
		if maxScale < minScale {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("maxScale=%d is less than minScale=%d", maxScale, minScale),
				Paths:   []string{fmt.Sprintf("metadata.annotations[%s]", AutoscalingMaxScaleAnnotation), fmt.Sprintf("[%s]", AutoscalingMinScaleAnnotation)},
			})
		}
		_, errs = validateAnnotation(annotations, KedaAutoscalingPollingIntervalAnnotation, minimumKedaPollingInterval, errs)
		_, errs = validateAnnotation(annotations, KedaAutoscalingCooldownPeriodAnnotation, minimumKedaCooldownPeriod, errs)
		_, errs = validateAnnotation(annotations, KedaAutoscalingSubscriptionSizeAnnotation, minimumKedaSubscriptionSize, errs)
	} else {
		errs = validateAnnotationNotExists(annotations, AutoscalingMinScaleAnnotation, errs)
		errs = validateAnnotationNotExists(annotations, AutoscalingMaxScaleAnnotation, errs)
		errs = validateAnnotationNotExists(annotations, KedaAutoscalingPollingIntervalAnnotation, errs)
		errs = validateAnnotationNotExists(annotations, KedaAutoscalingCooldownPeriodAnnotation, errs)
		errs = validateAnnotationNotExists(annotations, KedaAutoscalingSubscriptionSizeAnnotation, errs)
	}
	return errs
}

func validateAnnotation(annotations map[string]string, annotation string, minimumValue int, errs *apis.FieldError) (int, *apis.FieldError) {
	var value int
	if val, ok := annotations[annotation]; !ok {
		errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("metadata.annotations[%s]", annotation)))
	} else if v, err := strconv.Atoi(val); err != nil {
		errs = errs.Also(apis.ErrInvalidValue(val, fmt.Sprintf("metadata.annotations[%s]", annotation)))
	} else if v < minimumValue {
		errs = errs.Also(apis.ErrOutOfBoundsValue(v, minimumValue, math.MaxInt32, fmt.Sprintf("metadata.annotations[%s]", annotation)))
	} else {
		value = v
	}
	return value, errs
}

func validateAnnotationNotExists(annotations map[string]string, annotation string, errs *apis.FieldError) *apis.FieldError {
	if _, ok := annotations[annotation]; ok {
		errs = errs.Also(apis.ErrDisallowedFields(fmt.Sprintf("metadata.annotations[%s]", annotation)))
	}
	return errs
}

// CheckImmutableClusterNameAnnotation checks non-empty cluster-name annotation is immutable.
func CheckImmutableClusterNameAnnotation(current *metav1.ObjectMeta, original *metav1.ObjectMeta, errs *apis.FieldError) *apis.FieldError {
	if _, ok := original.Annotations[ClusterNameAnnotation]; ok {
		if diff := cmp.Diff(original.Annotations[ClusterNameAnnotation], current.Annotations[ClusterNameAnnotation]); diff != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "Immutable fields changed (-old +new)",
				Paths:   []string{fmt.Sprintf("metadata.annotations[%s]", ClusterNameAnnotation)},
				Details: diff,
			})
		}
	}
	return errs
}

// CheckImmutableAutoscalingClassAnnotations checks AutoscalingClassAnnotation Annotations are immutable.
func CheckImmutableAutoscalingClassAnnotations(current *metav1.ObjectMeta, original *metav1.ObjectMeta, errs *apis.FieldError) *apis.FieldError {
	// If AutoscalingClassAnnotation is immutable no matter if it was defined or not
	if diff := cmp.Diff(original.Annotations[AutoscalingClassAnnotation], current.Annotations[AutoscalingClassAnnotation]); diff != "" {
		errs = errs.Also(&apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{fmt.Sprintf("metadata.annotations[%s]", AutoscalingClassAnnotation)},
			Details: diff,
		})
	}
	return errs
}

// ValidateCredential checks secret and service account.
func ValidateCredential(secret *corev1.SecretKeySelector, kServiceAccountName string) *apis.FieldError {
	if secret != nil && !equality.Semantic.DeepEqual(secret, &corev1.SecretKeySelector{}) && kServiceAccountName != "" {
		return &apis.FieldError{
			Message: "Can't have spec.serviceAccountName and spec.secret at the same time",
			Paths:   []string{""},
		}
	} else if secret != nil && !equality.Semantic.DeepEqual(secret, &corev1.SecretKeySelector{}) {
		return validateSecret(secret)
	} else if kServiceAccountName != "" {
		return validateK8sServiceAccount(kServiceAccountName)
	}
	return nil
}

func validateSecret(secret *corev1.SecretKeySelector) *apis.FieldError {
	var errs *apis.FieldError
	if secret.Name == "" {
		errs = errs.Also(apis.ErrMissingField("secret.name"))
	}
	if secret.Key == "" {
		errs = errs.Also(apis.ErrMissingField("secret.key"))
	}
	return errs
}

func validateK8sServiceAccount(kServiceAccountName string) *apis.FieldError {
	match := ksaValidationRegex.FindStringSubmatch(kServiceAccountName)
	if len(match) == 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf(`invalid value: %s, serviceAccountName should have format: ^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`,
				kServiceAccountName),
			Paths: []string{"serviceAccountName"},
		}
	}
	return nil
}
