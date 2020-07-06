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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/knative-gcp/pkg/client/clientset/versioned"
	upgrader "github.com/google/knative-gcp/pkg/upgrader/v0.16.0"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func main() {
	ctx := signals.NewContext()
	cfg := sharedmain.ParseAndGetConfigOrDie()
	ctx = context.WithValue(ctx, kubeclient.Key{}, kubernetes.NewForConfigOrDie(cfg))
	ctx = context.WithValue(ctx, dynamicclient.Key{}, versioned.NewForConfigOrDie(cfg))
	if err := upgrader.Upgrade(ctx); err != nil {
		fmt.Printf("Upgrade failed with: %v\n", err)
		os.Exit(1)
	}
}
