package resources

import (
	v1 "github.com/google/knative-gcp/pkg/apis/intevents/v1"
	"github.com/google/knative-gcp/pkg/reconciler/intevents/pullsubscription/resources"
)

// GenerateScaledObjectName generates the name for the ScaledObject based on the PullSubscription information.
func GenerateScaledObjectName(ps *v1.PullSubscription) string {
	return resources.GenerateK8sName(ps)
}
