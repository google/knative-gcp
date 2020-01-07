/*
Copyright 2019 Google LLC

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

package e2e

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/api/iterator"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/test/common"

	"github.com/google/knative-gcp/test/e2e/metrics"
	"github.com/google/knative-gcp/test/operations"
)

// Setup runs the Setup in the common eventing test framework.
func Setup(t *testing.T, runInParallel bool) *Client {
	coreClient := common.Setup(t, runInParallel)
	client := &Client{
		Core:      coreClient,
		Namespace: coreClient.Namespace,
	}
	client.DuplicateSecret(t, "google-cloud-key", "default")
	return client
}

// TearDown runs the TearDown in the common eventing test framework.
func TearDown(client *Client) {
	common.TearDown(client.Core)
}

// Client holds instances of interfaces for making requests to Knative.
type Client struct {
	Core *common.Client

	Namespace string
}

var setStackDriverConfigOnce = sync.Once{}

func (c *Client) SetupStackDriverMetrics(t *testing.T) {
	setStackDriverConfigOnce.Do(func() {
		err := c.Core.Kube.UpdateConfigMap("cloud-run-events", "config-observability", map[string]string{
			"metrics.allow-stackdriver-custom-metrics":     "true",
			"metrics.backend-destination":                  "stackdriver",
			"metrics.stackdriver-custom-metrics-subdomain": "cloud.google.com",
			"metrics.reporting-period-seconds":             "60",
		})
		if err != nil {
			t.Fatalf("Unable to set the ConfigMap: %v", err)
		}
	})
}

// DuplicateSecret duplicates a secret from a namespace to a new namespace.
func (c *Client) DuplicateSecret(t *testing.T, name, namespace string) {
	cc := c.Core
	secret, err := cc.Kube.Kube.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to find Secret: %q in Namespace: %q: %s", name, namespace, err)
		return
	}
	newSecret := &corev1.Secret{}
	newSecret.Name = name
	newSecret.Namespace = c.Namespace
	newSecret.Data = secret.Data
	newSecret.StringData = secret.StringData
	newSecret.Type = secret.Type
	newSecret, err = cc.Kube.Kube.CoreV1().Secrets(c.Namespace).Create(newSecret)
	if err != nil {
		t.Fatalf("Failed to create Secret: %s; %v", c.Namespace, err)
	}
}

const (
	interval = 1 * time.Second
	timeout  = 5 * time.Minute
)

// WaitForResourceReady waits until the specified resource in the given namespace are ready.
func (c *Client) WaitForResourceReady(namespace, name string, gvr schema.GroupVersionResource) error {
	lastMsg := ""
	like := &duckv1.KResource{}
	return wait.PollImmediate(interval, timeout, func() (bool, error) {

		us, err := c.Core.Dynamic.Resource(gvr).Namespace(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		obj := like.DeepCopy()
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(us.Object, obj); err != nil {
			log.Fatalf("Error DefaultUnstructuree.Dynamiconverter. %v", err)
		}
		obj.ResourceVersion = gvr.Version
		obj.APIVersion = gvr.GroupVersion().String()

		ready := obj.Status.GetCondition(apis.ConditionReady)
		if ready != nil && !ready.IsTrue() {
			msg := fmt.Sprintf("%s is not ready, %s: %s", name, ready.Reason, ready.Message)
			if msg != lastMsg {
				log.Println(msg)
				lastMsg = msg
			}
		}

		return ready.IsTrue(), nil
	})
}

// WaitForResourceReady waits until the specified resource in the given namespace are ready.
func (c *Client) WaitUntilJobDone(namespace, name string) (string, error) {
	cc := c.Core
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		job, err := cc.Kube.Kube.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		return operations.IsJobComplete(job), nil
	})
	if err != nil {
		return "", err
	}

	// poll until the pod is terminated.
	err = wait.PollImmediate(interval, timeout, func() (bool, error) {
		pod, err := operations.GetJobPodByJobName(context.TODO(), cc.Kube.Kube, namespace, name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				log.Println(namespace, name, "not found", err)
				// keep polling
				return false, nil
			}
			return false, err
		}
		if pod != nil {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Terminated != nil {
					return true, nil
				}
			}
		}
		return false, nil
	})

	if err != nil {
		return "", err
	}
	pod, err := operations.GetJobPodByJobName(context.TODO(), cc.Kube.Kube, namespace, name)
	if err != nil {
		return "", err
	}
	return operations.GetFirstTerminationMessage(pod), nil
}

func (c *Client) LogsFor(namespace, name string, gvr schema.GroupVersionResource) (string, error) {
	cc := c.Core
	// Get all pods in this namespace.
	pods, err := cc.Kube.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	logs := make([]string, 0)

	// Look for a pod with the name that was passed in inside the pod name.
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, name) {
			// Collect all the logs from all the containers for this pod.
			if l, err := cc.Kube.Kube.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{}).DoRaw(); err != nil {
				logs = append(logs, err.Error())
			} else {
				logs = append(logs, string(l))
			}
		}
	}

	// Did we find a match like the given name?
	if len(logs) == 0 {
		return "", fmt.Errorf(`pod for "%s/%s" [%s] not found`, namespace, name, gvr.String())
	}

	return strings.Join(logs, "\n"), nil
}

// TODO make this function more generic.
func (c *Client) StackDriverEventCountMetricFor(namespace, projectID, filter string) (*int64, error) {
	metricClient, err := metrics.NewStackDriverMetricClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create stackdriver metric client: %v", err)
	}

	// TODO make times configurable if needed.
	metricRequest := metrics.NewStackDriverListTimeSeriesRequest(projectID,
		metrics.WithStackDriverFilter(filter),
		// Starting 5 minutes back until now.
		metrics.WithStackDriverInterval(time.Now().Add(-5*time.Minute).Unix(), time.Now().Unix()),
		// Delta counts aggregated every 2 minutes.
		// We aggregate for count as other aggregations will give higher values.
		// The reason is that PubSub upon an error, will retry, thus we will be recording multiple events.
		metrics.WithStackDriverAlignmentPeriod(2*int64(time.Minute.Seconds())),
		metrics.WithStackDriverPerSeriesAligner(monitoringpb.Aggregation_ALIGN_DELTA),
		metrics.WithStackDriverCrossSeriesReducer(monitoringpb.Aggregation_REDUCE_COUNT),
	)

	it := metricClient.ListTimeSeries(context.TODO(), metricRequest)

	for {
		res, err := it.Next()
		if err == iterator.Done {
			return nil, errors.New("no metric reported")
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over result: %v", err)
		}
		actualCount := res.GetPoints()[0].GetValue().GetInt64Value()
		return &actualCount, nil
	}
}
