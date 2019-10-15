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
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/knative-gcp/pkg/operations"
	"github.com/google/knative-gcp/test/e2e/metrics"
	"google.golang.org/api/iterator"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/test"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// Setup creates the client objects needed in the e2e tests,
// and does other setups, like creating namespaces, set the test case to run in parallel, etc.
func Setup(t *testing.T, runInParallel bool) *Client {
	// Create a new namespace to run this test case.
	baseName := helpers.AppendRandomString(helpers.GetBaseFuncName(t.Name()))
	namespace := helpers.MakeK8sNamePrefix(baseName)
	t.Logf("namespace is : %q", namespace)
	client, err := NewClient(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		namespace,
		t)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}

	client.CreateNamespaceIfNeeded(t)
	client.DuplicateSecret(t, "google-cloud-key", "default")

	// Disallow manually interrupting the tests.
	// TODO(Fredy-Z): t.Skip() can only be called on its own goroutine.
	//                Investigate if there is other way to gracefully terminte the tests in the middle.
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("Test %q running, please don't interrupt...\n", t.Name())
	}()

	// Run the test case in parallel if needed.
	if runInParallel {
		t.Parallel()
	}

	return client
}

// TearDown will delete created names using clients.
func TearDown(client *Client) {
	if err := DeleteNameSpace(client); err != nil {
		client.T.Logf("Could not delete the namespace %q: %v", client.Namespace, err)
	}
}

// DeleteNameSpace deletes the namespace that has the given name.
func DeleteNameSpace(client *Client) error {
	_, err := client.Kube.Kube.CoreV1().Namespaces().Get(client.Namespace, metav1.GetOptions{})
	if err == nil || !apierrors.IsNotFound(err) {
		return client.Kube.Kube.CoreV1().Namespaces().Delete(client.Namespace, nil)
	}
	return err
}

// Client holds instances of interfaces for making requests to Knative.
type Client struct {
	Kube    *test.KubeClient
	Dynamic dynamic.Interface

	Namespace string
	T         *testing.T
}

// NewClient instantiates and returns clientsets required for making request to the
// cluster specified by the combination of clusterName and configPath.
func NewClient(configPath string, clusterName string, namespace string, t *testing.T) (*Client, error) {
	client := &Client{}
	cfg, err := test.BuildClientConfig(configPath, clusterName)
	if err != nil {
		return nil, err
	}
	client.Kube, err = test.NewKubeClient(configPath, clusterName)
	if err != nil {
		return nil, err
	}

	client.Dynamic, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	client.Namespace = namespace
	client.T = t
	return client, nil
}

// CreateNamespaceIfNeeded creates a new namespace if it does not exist.
func (c *Client) CreateNamespaceIfNeeded(t *testing.T) {
	nsSpec, err := c.Kube.Kube.CoreV1().Namespaces().Get(c.Namespace, metav1.GetOptions{})

	if err != nil && apierrors.IsNotFound(err) {
		nsSpec = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: c.Namespace}}
		nsSpec, err = c.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)

		if err != nil {
			t.Fatalf("Failed to create Namespace: %s; %v", c.Namespace, err)
		}

		// https://github.com/kubernetes/kubernetes/issues/66689
		// We can only start creating pods after the default ServiceAccount is created by the kube-controller-manager.
		err = waitForServiceAccountExists(t, c, "default", c.Namespace)
		if err != nil {
			t.Fatalf("The default ServiceAccount was not created for the Namespace: %s", c.Namespace)
		}
	}
}

var setStackDriverConfigOnce = sync.Once{}

func (c *Client) SetupStackDriverMetrics(t *testing.T) {
	setStackDriverConfigOnce.Do(func() {
		err := c.Kube.UpdateConfigMap("cloud-run-events", "config-observability", map[string]string{
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
	secret, err := c.Kube.Kube.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
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
	newSecret, err = c.Kube.Kube.CoreV1().Secrets(c.Namespace).Create(newSecret)
	if err != nil {
		t.Fatalf("Failed to create Secret: %s; %v", c.Namespace, err)
	}
}

const (
	interval = 1 * time.Second
	timeout  = 5 * time.Minute
)

// waitForServiceAccountExists waits until the ServiceAccount exists.
func waitForServiceAccountExists(t *testing.T, client *Client, name, namespace string) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		sas := client.Kube.Kube.CoreV1().ServiceAccounts(namespace)
		if _, err := sas.Get(name, metav1.GetOptions{}); err == nil {
			return true, nil
		}
		return false, nil
	})
}

// WaitForResourceReady waits until the specified resource in the given namespace are ready.
func (c *Client) WaitForResourceReady(namespace, name string, gvr schema.GroupVersionResource) error {
	lastMsg := ""
	like := &duckv1.KResource{}
	return wait.PollImmediate(interval, timeout, func() (bool, error) {

		us, err := c.Dynamic.Resource(gvr).Namespace(namespace).Get(name, metav1.GetOptions{})
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
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		job, err := c.Kube.Kube.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
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
		pod, err := operations.GetJobPodByJobName(context.TODO(), c.Kube.Kube, namespace, name)
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
	pod, err := operations.GetJobPodByJobName(context.TODO(), c.Kube.Kube, namespace, name)
	if err != nil {
		return "", err
	}
	return operations.GetFirstTerminationMessage(pod), nil
}

func (c *Client) LogsFor(namespace, name string, gvr schema.GroupVersionResource) (string, error) {
	// Get all pods in this namespace.
	pods, err := c.Kube.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	logs := make([]string, 0)

	// Look for a pod with the name that was passed in inside the pod name.
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, name) {
			// Collect all the logs from all the containers for this pod.
			if l, err := c.Kube.Kube.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{}).DoRaw(); err != nil {
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
