package resources

import (
	"fmt"

	"k8s.io/api/apps/v1"
)

// GenerateScaledObjectName generates the name for the ScaledObject based on the Deployment UID.
func GenerateScaledObjectName(ra *v1.Deployment) string {
	return fmt.Sprintf("cre-so-%s", string(ra.UID))
}
