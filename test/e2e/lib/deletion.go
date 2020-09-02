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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) DeletePubSubOrFail(name string) {
	c.T.Helper()
	pubsubs := c.KnativeGCP.EventsV1().CloudPubSubSources(c.Namespace)
	err := pubsubs.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		c.T.Fatalf("Failed to delete pubsub %s/%s: %v", c.Namespace, name, err)
	}
}

func (c *Client) DeleteBuildOrFail(name string) {
	c.T.Helper()
	builds := c.KnativeGCP.EventsV1().CloudBuildSources(c.Namespace)
	err := builds.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		c.T.Fatalf("Failed to delete build %s/%s: %v", c.Namespace, name, err)
	}
}

func (c *Client) DeleteSchedulerOrFail(name string) {
	c.T.Helper()
	schedulers := c.KnativeGCP.EventsV1().CloudSchedulerSources(c.Namespace)
	err := schedulers.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		c.T.Fatalf("Failed to delete scheduler %s/%s: %v", c.Namespace, name, err)
	}
}

func (c *Client) DeleteStorageOrFail(name string) {
	c.T.Helper()
	storages := c.KnativeGCP.EventsV1().CloudStorageSources(c.Namespace)
	err := storages.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		c.T.Fatalf("Failed to delete storage %s/%s: %v", c.Namespace, name, err)
	}
}

func (c *Client) DeleteAuditLogsOrFail(name string) {
	c.T.Helper()
	auditLogs := c.KnativeGCP.EventsV1().CloudAuditLogsSources(c.Namespace)
	err := auditLogs.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		c.T.Fatalf("Failed to delete storage %s/%s: %v", c.Namespace, name, err)
	}
}

func (c *Client) DeleteGCPBrokerOrFail(name string) {
	c.T.Helper()
	brokers := c.KnativeGCP.EventingV1beta1().Brokers(c.Namespace)
	err := brokers.Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		c.T.Fatalf("Failed to delete gcp broker %s/%s: %v", c.Namespace, name, err)
	}
}
