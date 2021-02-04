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

package controller

import (
	"sync"

	corev1 "k8s.io/api/core/v1"

	. "github.com/google/knative-gcp/pkg/broker/readiness"
	"google.golang.org/grpc"
)

type DataplanePodAddress string
type GenerationQueryClientConn *grpc.ClientConn

func NewGenerationQueryClientConn(address DataplanePodAddress) (GenerationQueryClientConn, error) {
	conn, err := grpc.Dial(string(address), grpc.WithInsecure())
	return GenerationQueryClientConn(conn), err
}

func NewGenerationQueryClient(clientConn GenerationQueryClientConn) GenerationQueryServiceClient {
	return NewGenerationQueryServiceClient((*grpc.ClientConn)(clientConn))
}

type ConfigCheckClientsMap struct {
	sync.Mutex
	grpcClients map[podKeyType]GenerationQueryServiceClient
	grpcConns   map[podKeyType]GenerationQueryClientConn
}

func (c *ConfigCheckClientsMap) getClient(pod *corev1.Pod) GenerationQueryServiceClient {
	c.Lock()
	defer c.Unlock()
	key := getPodKey(pod)
	client, ok := c.grpcClients[key]
	if !ok {
		if conn, err := NewGenerationQueryClientConn(DataplanePodAddress(pod.Status.PodIP)); err != nil {
			c.grpcConns[key] = conn
			client = NewGenerationQueryClient(conn)
		} // TODO add error handling
	}
	return client
}

func (c *ConfigCheckClientsMap) deleteClient(pod *corev1.Pod) GenerationQueryServiceClient {
	c.Lock()
	defer c.Unlock()
	key := getPodKey(pod)
	client, ok := c.grpcClients[key]
	if ok {
		conn := c.grpcConns[key]
		(*grpc.ClientConn)(conn).Close()
	}
	delete(c.grpcClients, key)
	return client
}

func NewConfigCheckClientsMap() *ConfigCheckClientsMap {
	return &ConfigCheckClientsMap{
		grpcClients: make(map[podKeyType]GenerationQueryServiceClient),
		grpcConns:   make(map[podKeyType]GenerationQueryClientConn),
	}
}
