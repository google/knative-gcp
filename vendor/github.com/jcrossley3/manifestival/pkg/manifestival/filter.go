package manifestival

import (
	"os"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FilterFn func(u *unstructured.Unstructured) bool

type Owner interface {
	v1.Object
	schema.ObjectKind
}

func (f *YamlManifest) Filter(fns ...FilterFn) Manifest {
	var results []unstructured.Unstructured
OUTER:
	for i := 0; i < len(f.resources); i++ {
		spec := f.resources[i].DeepCopy()
		for _, f := range fns {
			if !f(spec) {
				continue OUTER
			}
		}
		results = append(results, *spec)
	}
	f.resources = results
	return f
}

// We assume all resources in the manifest live in the same namespace
func ByNamespace(ns string) FilterFn {
	namespace := resolveEnv(ns)
	return func(u *unstructured.Unstructured) bool {
		switch strings.ToLower(u.GetKind()) {
		case "namespace":
			return false
		case "clusterrolebinding":
			subjects, _, _ := unstructured.NestedFieldNoCopy(u.Object, "subjects")
			for _, subject := range subjects.([]interface{}) {
				m := subject.(map[string]interface{})
				if _, ok := m["namespace"]; ok {
					m["namespace"] = namespace
				}
			}
		}
		if !isClusterScoped(u.GetKind()) {
			u.SetNamespace(namespace)
		}
		return true
	}
}

func ByOwner(owner Owner) FilterFn {
	return func(u *unstructured.Unstructured) bool {
		if !isClusterScoped(u.GetKind()) {
			// apparently reference counting for cluster-scoped
			// resources is broken, so trust the GC only for ns-scoped
			// dependents
			u.SetOwnerReferences([]v1.OwnerReference{*v1.NewControllerRef(owner, owner.GroupVersionKind())})
		}
		return true
	}
}

func isClusterScoped(kind string) bool {
	// TODO: something more clever using !APIResource.Namespaced maybe?
	switch strings.ToLower(kind) {
	case "componentstatus",
		"namespace",
		"node",
		"persistentvolume",
		"mutatingwebhookconfiguration",
		"validatingwebhookconfiguration",
		"customresourcedefinition",
		"apiservice",
		"meshpolicy",
		"tokenreview",
		"selfsubjectaccessreview",
		"selfsubjectrulesreview",
		"subjectaccessreview",
		"certificatesigningrequest",
		"podsecuritypolicy",
		"clusterrolebinding",
		"clusterrole",
		"priorityclass",
		"storageclass",
		"volumeattachment":
		return true
	}
	return false
}

func resolveEnv(x string) string {
	if len(x) > 1 && x[:1] == "$" {
		return os.Getenv(x[1:])
	}
	return x
}
