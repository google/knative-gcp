module github.com/google/knative-gcp

go 1.14

require (
	cloud.google.com/go v0.62.0
	cloud.google.com/go/logging v1.0.1-0.20200331222814-69e77e66e597
	cloud.google.com/go/pubsub v1.6.1
	cloud.google.com/go/storage v1.10.0
	github.com/cloudevents/sdk-go/protocol/pubsub/v2 v2.0.1-0.20200602143929-d07dc0510d45
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.1
	github.com/google/uuid v1.1.1
	github.com/google/wire v0.4.0
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/rickb777/date v1.13.0
	go.opencensus.io v0.22.5-0.20200714042313-af30f77c5f65
	go.opentelemetry.io/otel v0.3.0 // indirect
	go.uber.org/multierr v1.5.0
	go.uber.org/zap v1.15.0
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	google.golang.org/api v0.29.0
	google.golang.org/genproto v0.0.0-20200731012542-8145dea6a485
	google.golang.org/grpc v1.31.0
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.18.7-rc.0
	k8s.io/apimachinery v0.18.7-rc.0
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/eventing v0.17.1-0.20200828141207-daaf5619ec30
	knative.dev/pkg v0.0.0-20200827231407-8d478d55c0e6
	knative.dev/serving v0.17.1-0.20200828171108-6826f9a6f80b
	knative.dev/test-infra v0.0.0-20200828152807-808074748557 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace (
	istio.io/api => istio.io/api v0.0.0-20200227213531-891bf31f3c32
	istio.io/client-go => istio.io/client-go v0.0.0-20200227214646-23b87b42e49b
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d
)

// The following packages were pinned as part of the go module transition and should eventually be
// unpinned.
replace github.com/blang/semver => github.com/blang/semver v1.1.1-0.20190414102917-ba2c2ddd8906

replace github.com/jmespath/go-jmespath => github.com/jmespath/go-jmespath v0.0.0-20180206201540-c2b33e8439af

replace github.com/json-iterator/go => github.com/json-iterator/go v1.1.7

replace github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742

replace github.com/pkg/errors => github.com/pkg/errors v0.8.1

replace github.com/robfig/cron/v3 => github.com/robfig/cron/v3 v3.0.0

replace gomodules.xyz/jsonpatch/v2 => gomodules.xyz/jsonpatch/v2 v2.0.1

replace gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.2

replace honnef.co/go/tools => honnef.co/go/tools v0.0.1-2019.2.3

replace sigs.k8s.io/yaml => sigs.k8s.io/yaml v1.1.0
