// +build wireinject

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

package probe

import (
	"context"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/google/wire"
	"k8s.io/client-go/kubernetes"

	"github.com/google/knative-gcp/pkg/utils/clients"
	"github.com/google/knative-gcp/test/test_images/probe_helper/probe/handlers"
)

func InitializeTestProbeHelper(ctx context.Context, brokerCellBaseUrl string, projectID clients.ProjectID, cronStaleDuration time.Duration, helperEnv EnvConfig, forwardListener ForwardListener, receiveListener ReceiveListener, storageClient *storage.Client, psClient *pubsub.Client, k8sClient kubernetes.Interface) (*Helper, error) {
	panic(wire.Build(TestHelperSet, handlers.HandlerSet))
}
