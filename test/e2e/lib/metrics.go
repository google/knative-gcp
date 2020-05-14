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
	"fmt"
	"io/ioutil"
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
func printAllPodMetricsIfTestFailed(client *Client) {
	if !client.T.Failed() {
		// No failure, so no need for logs!
		return
	}
	pods, err := client.Core.Kube.Kube.CoreV1().Pods(client.Namespace).List(metav1.ListOptions{})
	if err != nil {
		client.T.Logf("Unable to list pods: %v", err)
		return
	}
	for _, pod := range pods.Items {
		printPodMetrics(client, pod)
	}
}

// printPodMetrics attempts to print the metrics from a single Pod to the test logs.
func printPodMetrics(client *Client, pod corev1.Pod) {
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
		client.T.Logf("Pod '%v' does not have a metrics port", podName)
		return
	}

	root, err := getRootOwnerOfPod(client, pod)
	if err != nil {
		client.T.Logf("Unable to get root owner of the Pod '%v': %v", podName, err)
		root = "root-unknown"
	}

	podList := &corev1.PodList{
		Items: []corev1.Pod{
			pod,
		},
	}
	// This is just a random number, could be anything. Probably should retry if this port is taken.
	localPort := 58295
	// There is almost certainly a better way to do this, but for now, just use kubectl to port
	// forward and use HTTP to read the metrics.
	pid, err := monitoring.PortForward(client.T.Logf, podList, localPort, metricsPort, pod.Namespace)
	if err != nil {
		client.T.Logf("Unable to port forward for Pod '%v': %v", podName, err)
		return
	}
	defer monitoring.Cleanup(pid)

	// Port forwarding takes a bit of time to start running, so try gets until it works.
	var resp *http.Response
	err = wait.PollImmediate(500*time.Millisecond, 10*time.Second, func() (bool, error) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://localhost:%v/metrics", localPort), nil)
		req.Close = true
		c := &http.Client{
			Transport: &http.Transport{DisableKeepAlives: true},
		}
		resp, err = c.Do(req)
		if net.IsConnectionRefused(err) {
			return false, nil
		} else {
			return true, err
		}
	})

	if err != nil {
		client.T.Logf("Unable to read metrics from Pod '%v' (root %q): %v", podName, root, err)
		return
	}
	defer resp.Body.Close()
	if resp.ContentLength == 0 {
		client.T.Logf("Pod had no metrics reported '%v' (root %q)", podName, root)
		return
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		client.T.Logf("Unable to read HTTP response body for root %q: %v", root, err)
		return
	}
	client.T.Logf("Metrics logs for root %q: %s", root, string(b))
}

func getRootOwnerOfPod(client *Client, pod corev1.Pod) (string, error) {
	u := unstructured.Unstructured{}
	u.SetName(pod.Name)
	u.SetNamespace(pod.Namespace)
	u.SetOwnerReferences(pod.OwnerReferences)

	root, err := getRootOwner(client, u)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", root.GetKind(), root.GetName()), nil
}

func getRootOwner(client *Client, u unstructured.Unstructured) (unstructured.Unstructured, error) {
	for _, o := range u.GetOwnerReferences() {
		if *o.Controller {
			gvr := createGVR(o)
			g, err := client.Core.Dynamic.Resource(gvr).Namespace(u.GetNamespace()).Get(o.Name, metav1.GetOptions{})
			if err != nil {
				client.T.Logf("Failed to dynamic.Get: %v, %v, %v, %v", gvr, u.GetNamespace(), o.Name, err)
				return unstructured.Unstructured{}, err
			}
			return getRootOwner(client, *g)
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
