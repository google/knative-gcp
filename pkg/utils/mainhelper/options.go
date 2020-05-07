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

package mainhelper

import (
	"context"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

type initArgs struct {
	component string

	injection           injection.Interface
	ctx                 context.Context
	kubeConfig          *rest.Config
	skipK8sVersionCheck bool
	env                 interface{}
}

type InitOption func(*initArgs)

func newInitArgs(component string, opts ...InitOption) initArgs {
	args := initArgs{
		component: component,
	}
	for _, opt := range opts {
		opt(&args)
	}
	if args.kubeConfig == nil {
		args.kubeConfig = sharedmain.ParseAndGetConfigOrDie()
	}
	if args.injection == nil {
		args.injection = injection.Default
	}
	if args.ctx == nil {
		// Set up signals so we handle the first shutdown signal gracefully.
		args.ctx = signals.NewContext()
	}
	return args
}

// WithKubeFakes uses knative injections fakes for any k8s related setup. This is used in tests.
var WithKubeFakes InitOption = func(args *initArgs) {
	args.injection = injection.Fake

	// TODO If we can run a k8s APIServer locally, we can use a real config and avoid skipping version check.
	// check k8s.io/kubernetes/test/integration/utils
	args.kubeConfig = &rest.Config{}
	args.skipK8sVersionCheck = true
}

// WithContext specifies the context to use.
func WithContext(ctx context.Context) InitOption {
	return func(args *initArgs) {
		args.ctx = ctx
	}
}

// WithEnv specifies a pointer to an envConfig struct.
func WithEnv(env interface{}) InitOption {
	return func(args *initArgs) {
		args.env = env
	}
}
