package resources

import (
	"github.com/google/knative-gcp/pkg/apis/intevents"
	"github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
)

// GenerateScaledObjectName generates the name for the ScaledObject based on the PullSubscription information.
func GenerateScaledObjectName(ps *v1alpha1.PullSubscription) string {
	return intevents.GenerateK8sName(ps)
}
