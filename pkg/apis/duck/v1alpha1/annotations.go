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

package v1alpha1

import (
	"context"
	"fmt"
	"math"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	// Autoscaling refers to the autoscaling group.
	Autoscaling = "autoscaling.knative.dev"

	// AutoscalingClassAnnotation is the annotation for the explicit class of
	// scaler that a particular resource has opted into.
	AutoscalingClassAnnotation = Autoscaling + "/class"

	// AutoscalingMinScaleAnnotation is the annotation to specify the minimum number of pods to scale to.
	AutoscalingMinScaleAnnotation = Autoscaling + "/minScale"
	// AutoscalingMaxScaleAnnotation is the annotation to specify the maximum number of pods to scale to.
	AutoscalingMaxScaleAnnotation = Autoscaling + "/maxScale"

	// KEDA is Keda autoscaler.
	KEDA = "keda.autoscaling.knative.dev"

	// KedaAutoscalingPollingIntervalAnnotation is the annotation that refers to the interval in seconds Keda
	// uses to poll metrics in order to inform its scaling decisions.
	KedaAutoscalingPollingIntervalAnnotation = KEDA + "/pollingInterval"
	// KedaAutoscalingCooldownPeriodAnnotation is the annotation that refers to the period Keda waits until it
	// scales a Deployment down.
	KedaAutoscalingCooldownPeriodAnnotation = KEDA + "/cooldownPeriod"
	// KedaAutoscalingSubscriptionSizeAnnotation is the annotation that refers to the size of unacked messages in a
	// Pub/Sub subscription that Keda uses in order to decide when and by how much to scale out.
	KedaAutoscalingSubscriptionSizeAnnotation = KEDA + "/subscriptionSize"

	// defaultMinScale is the default minimum set of Pods the scaler should
	// downscale the resource to.
	defaultMinScale = "0"
	// defaultMaxScale is the default maximum set of Pods the scaler should
	// upscale the resource to.
	defaultMaxScale = "1"

	// defaultKedaPollingInterval is the default polling interval in seconds Keda uses to poll metrics.
	defaultKedaPollingInterval = "15"
	// defaultKedaCooldownPeriod is the default cooldown period in seconds Keda uses to downscale resources.
	defaultKedaCooldownPeriod = "120"
	// defaultKedaSubscriptionSize is the default number of unacked messages in a Pub/Sub subscription that Keda
	// uses to scale out resources.
	defaultKedaSubscriptionSize = "100"

	// minimumMinScale is the minimum allowed value for the AutoscalingMinScaleAnnotation annotation.
	minimumMinScale = 0
	// minimumMaxScale is the minimum allowed value for the AutoscalingMaxScaleAnnotation annotation.
	minimumMaxScale = 1

	// minimumKedaPollingInterval is the minimum allowed value for the KedaAutoscalingPollingIntervalAnnotation annotation.
	minimumKedaPollingInterval = 5
	// minimumKedaCooldownPeriod is the minimum allowed value for the KedaAutoscalingCooldownPeriodAnnotation annotation.
	minimumKedaCooldownPeriod = 15
	// minimumKedaSubscriptionSize is the minimum allowed value for the KedaAutoscalingSubscriptionSizeAnnotation annotation.
	minimumKedaSubscriptionSize = 5
)

func SetAutoscalingAnnotationsDefaults(ctx context.Context, obj *metav1.ObjectMeta) {
	// If autoscaling was configured, then set defaults.
	if _, ok := obj.Annotations[AutoscalingClassAnnotation]; ok {
		setDefaultAnnotationIfNotPresent(obj, AutoscalingMinScaleAnnotation, defaultMinScale)
		setDefaultAnnotationIfNotPresent(obj, AutoscalingMaxScaleAnnotation, defaultMaxScale)
		setDefaultAnnotationIfNotPresent(obj, KedaAutoscalingPollingIntervalAnnotation, defaultKedaPollingInterval)
		setDefaultAnnotationIfNotPresent(obj, KedaAutoscalingCooldownPeriodAnnotation, defaultKedaCooldownPeriod)
		setDefaultAnnotationIfNotPresent(obj, KedaAutoscalingSubscriptionSizeAnnotation, defaultKedaSubscriptionSize)
		// If it wasn't configured, then delete any autoscaling related configuration.
	} else {
		deleteAnnotationIfPresent(obj, AutoscalingMinScaleAnnotation)
		deleteAnnotationIfPresent(obj, AutoscalingMaxScaleAnnotation)
		deleteAnnotationIfPresent(obj, KedaAutoscalingPollingIntervalAnnotation)
		deleteAnnotationIfPresent(obj, KedaAutoscalingCooldownPeriodAnnotation)
		deleteAnnotationIfPresent(obj, KedaAutoscalingSubscriptionSizeAnnotation)
	}
}

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

func setDefaultAnnotationIfNotPresent(obj *metav1.ObjectMeta, annotation string, defaultValue string) {
	if _, ok := obj.Annotations[annotation]; !ok {
		obj.Annotations[annotation] = defaultValue
	}
}

func deleteAnnotationIfPresent(obj *metav1.ObjectMeta, annotation string) {
	if _, ok := obj.Annotations[annotation]; ok {
		delete(obj.Annotations, annotation)
	}
}

func validateAnnotationNotExists(annotations map[string]string, annotation string, errs *apis.FieldError) *apis.FieldError {
	if _, ok := annotations[annotation]; ok {
		errs = errs.Also(apis.ErrDisallowedFields(fmt.Sprintf("metadata.annotations[%s]", annotation)))
	}
	return errs
}
