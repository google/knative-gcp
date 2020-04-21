package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	eventingtesting "knative.dev/eventing/pkg/reconciler/testing"
)

const (
	testDeploymentNS = "test-ns"
	testDeployment   = "test-deployment"
	otherNS          = "some-other-ns"
)

func pod(ns, name string, labels map[string]string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   ns,
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func deployment(ns, name string, matchLabels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: matchLabels},
		},
	}
}

func TestUpdateAnnotation(t *testing.T) {
	var (
		labels = map[string]string{
			"app":  "cloud-run-events",
			"role": "broker-component",
		}
		annotations = map[string]string{
			"somekey":           "somevalue",
			volumeGenerationKey: "1",
		}
		updatedAnnotations = map[string]string{
			"somekey":           "somevalue",
			volumeGenerationKey: "2",
		}
	)
	var tests = []struct {
		name        string
		pods        []runtime.Object
		deployments []runtime.Object
		wantPods    []corev1.Pod
		wantErr     bool
	}{
		{
			name:    "deployment not found",
			wantErr: true,
		},
		{
			name: "deployment has no pods",
			deployments: []runtime.Object{
				deployment(testDeploymentNS, testDeployment, labels),
			},
		},
		{
			name: "two pods match deployment selector but one of them in a different namespace",
			pods: []runtime.Object{
				pod(testDeploymentNS, "sameNS", labels, annotations),
				pod(otherNS, "differentNS", labels, annotations),
			},
			deployments: []runtime.Object{
				deployment(testDeploymentNS, testDeployment, labels),
			},
			wantPods: []corev1.Pod{
				*pod(testDeploymentNS, "sameNS", labels, updatedAnnotations),
				*pod(otherNS, "differentNS", labels, annotations),
			},
		},
		{
			name: "deployment has multiple pods, one with nil annotation, one with empty annotation, one with annotation",
			pods: []runtime.Object{
				pod(testDeploymentNS, "pod1", labels, nil),
				pod(testDeploymentNS, "pod2", labels, make(map[string]string)),
				pod(testDeploymentNS, "pod3", labels, annotations),
			},
			deployments: []runtime.Object{
				deployment(testDeploymentNS, testDeployment, labels),
			},
			wantPods: []corev1.Pod{
				*pod(testDeploymentNS, "pod1", labels, map[string]string{volumeGenerationKey: "1"}),
				*pod(testDeploymentNS, "pod2", labels, map[string]string{volumeGenerationKey: "1"}),
				*pod(testDeploymentNS, "pod3", labels, updatedAnnotations),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objs := append(test.deployments, test.pods...)
			client := fake.NewSimpleClientset(objs...)
			listers := eventingtesting.NewListers(objs)
			dl := listers.GetDeploymentLister()
			pl := listers.GetPodLister()

			err := UpdateVolumeGenerationForDeployment(client, dl, pl, testDeploymentNS, testDeployment)
			if !test.wantErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if test.wantErr && err == nil {
				t.Fatalf("Expect error but got nil")
			}

			gotPods := getPods(t, client, testDeploymentNS, otherNS)
			if len(gotPods) != len(test.wantPods) {
				t.Fatalf("Number of got pods is unexpected, got:%v, want:%v", len(gotPods), len(test.wantPods))
			}
			for i := range test.wantPods {
				if diff := cmp.Diff(gotPods[i], test.wantPods[i]); diff != "" {
					t.Errorf("Pod %v differ (-got, +want): %s", i, diff)
					return
				}
			}
		})
	}
}

// getPods returns pods from all given namespaces.
func getPods(t *testing.T, client kubernetes.Interface, namespaces ...string) []corev1.Pod {
	pods := []corev1.Pod{}
	for _, namespace := range namespaces {
		gotPods, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
		if err != nil {
			t.Fatalf("Failed to get pods for namespace %v: %v", namespace, err)
		}
		pods = append(pods, gotPods.Items...)
	}
	return pods
}
