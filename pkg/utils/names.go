package utils

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
)

const (
	PubsubMax = 255
	K8sNamespaceMax = 63
	K8sNameMax      = 253
)

// TruncatedPubsubResourceName generates a deterministic name for a Pub/Sub resource.
// If the name would be longer than allowed by Pub/Sub, the name is truncated to fit.
func TruncatedPubsubResourceName(prefix, ns, n string, uid types.UID) string {
	s := fmt.Sprintf("%s_%s_%s_%s", prefix, ns, n, string(uid))
	if len(s) <= PubsubMax {
		return s
	}
	names := fmt.Sprintf("%s_%s_%s", prefix, ns, n)
	namesMax := PubsubMax - ((len(string(uid))) + 1) // 1 for the uid separator

	return fmt.Sprintf("%s_%s", names[:namesMax], string(uid))
}
