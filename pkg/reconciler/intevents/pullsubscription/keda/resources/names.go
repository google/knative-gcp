package resources

import (
	"fmt"

	"github.com/google/knative-gcp/pkg/apis/intevents/v1alpha1"
)

// GenerateScaledObjectName generates the name for the ScaledObject based on the PullSubscription UID.
func GenerateScaledObjectName(ps *v1alpha1.PullSubscription) string {
	return fmt.Sprintf("cre-so-%s", string(ps.UID))
}
