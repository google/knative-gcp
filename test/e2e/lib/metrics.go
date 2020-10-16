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

package lib

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/monitoring"
)

// printAllPodMetricsIfTestFailed lists all Pods in the namespace and attempts to print out their
// metrics.
//
// The hope is that this will allow better understanding of where events are lost. E.g. did the
// source try to send the event to the channel/broker/sink?
func printAllPodMetricsIfTestFailed(ctx context.Context, client *Client) {
	if !client.T.Failed() {
		// No failure, so no need for logs!
		return
	}
	pods, err := client.Core.Kube.CoreV1().Pods(client.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		client.T.Logf("Unable to list pods: %v", err)
		return
	}
	for _, pod := range pods.Items {
		printPodMetrics(ctx, client, pod)
	}
}

// printPodMetrics attempts to print the metrics from a single Pod to the test logs.
func printPodMetrics(ctx context.Context, client *Client, pod corev1.Pod) {
	podName := types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}

	metricsPort := -1
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == "metrics" {
				metricsPort = int(p.ContainerPort)
				break
			}
		}
	}
	if metricsPort < 0 {
		client.T.Logf("Pod %q does not have a metrics port", podName)
		return
	}

	root, err := getRootOwnerOfPod(ctx, client, pod)
	if err != nil {
		client.T.Logf("Unable to get root owner of the Pod %q: %v", podName, err)
		root = "root-unknown"
	}

	podList := &corev1.PodList{
		Items: []corev1.Pod{
			pod,
		},
	}
	localPort, err := findAvailablePort()
	if err != nil {
		client.T.Logf("Unable to find an available port for Pod %q: %v", podName, err)
		return
	}
	// TODO There is almost certainly a better way to do this, but for now, just use kubectl to port
	// forward and use HTTP to read the metrics. https://github.com/google/knative-gcp/issues/1083
	pid, err := monitoring.PortForward(client.T.Logf, podList, localPort, metricsPort, pod.Namespace)
	if err != nil {
		client.T.Logf("Unable to port forward for Pod %q: %v", podName, err)
		return
	}
	defer monitoring.Cleanup(pid)

	// Port forwarding takes a bit of time to start running, so try GETs until it works.
	var resp *http.Response
	err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%v/metrics", localPort), nil)
		req.Close = true
		c := &http.Client{
			Transport: &http.Transport{DisableKeepAlives: true},
		}
		var innerErr error
		resp, innerErr = c.Do(req)
		if net.IsConnectionRefused(innerErr) {
			return false, nil
		} else {
			return true, innerErr
		}
	})

	if err != nil {
		client.T.Logf("Unable to read metrics from Pod %q (root %q): %v", podName, root, err)
		return
	}
	defer resp.Body.Close()
	if resp.ContentLength == 0 {
		client.T.Logf("Pod had no metrics reported %q (root %q)", podName, root)
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		client.T.Logf("Unable to read HTTP response body for root %q: %v", root, err)
		return
	}
	s := string(b)
	if len(s) == 0 {
		s = "<Empty Metrics>"
	}
	client.T.Logf("Metrics logs for root %q: %s", root, s)
}

func findAvailablePort() (int, error) {
	const portMin = 40_000
	const portMax = 60_000
	portRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	var localPort int
	var internalErr error
	// These times were chosen arbitrarily, feel free to change.
	err := wait.PollImmediate(20*time.Millisecond, 5*time.Second, func() (bool, error) {
		localPort = portMin + portRand.Intn(portMax-portMin)
		if err := monitoring.CheckPortAvailability(localPort); err != nil {
			internalErr = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return -1, fmt.Errorf("timeout finding an available port: %w", internalErr)
	}
	return localPort, nil
}

func getRootOwnerOfPod(ctx context.Context, client *Client, pod corev1.Pod) (string, error) {
	u := unstructured.Unstructured{}
	u.SetName(pod.Name)
	u.SetNamespace(pod.Namespace)
	u.SetOwnerReferences(pod.OwnerReferences)

	root, err := getRootOwner(ctx, client, u)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", root.GetKind(), root.GetName()), nil
}

func getRootOwner(ctx context.Context, client *Client, u unstructured.Unstructured) (unstructured.Unstructured, error) {
	for _, o := range u.GetOwnerReferences() {
		if *o.Controller {
			gvr := createGVR(o)
			g, err := client.Core.Dynamic.Resource(gvr).Namespace(u.GetNamespace()).Get(ctx, o.Name, metav1.GetOptions{})
			if err != nil {
				client.T.Logf("Failed to dynamic.Get: %v, %v, %v, %v", gvr, u.GetNamespace(), o.Name, err)
				return unstructured.Unstructured{}, err
			}
			return getRootOwner(ctx, client, *g)
		}
	}
	// There are no controlling owner references, this is the root.
	return u, nil
}

func createGVR(o metav1.OwnerReference) schema.GroupVersionResource {
	gvk := schema.GroupVersionKind{
		Kind: o.Kind,
	}
	if s := strings.Split(o.APIVersion, "/"); len(s) == 1 {
		gvk.Version = s[0]
	} else {
		gvk.Group = s[0]
		gvk.Version = s[1]
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return gvr
}
