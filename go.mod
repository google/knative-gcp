module github.com/google/knative-gcp

go 1.14

require (
	cloud.google.com/go v0.72.0
	cloud.google.com/go/logging v1.0.1-0.20200331222814-69e77e66e597
	cloud.google.com/go/pubsub v1.8.0
	cloud.google.com/go/storage v1.10.0
	github.com/cloudevents/sdk-go/protocol/pubsub/v2 v2.2.1-0.20200806165906-9ae0708e27fa
	github.com/cloudevents/sdk-go/v2 v2.3.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.4
	github.com/google/uuid v1.1.2
	github.com/google/wire v0.4.0
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/rickb777/date v1.13.0
	github.com/stretchr/testify v1.5.1
	github.com/wavesoftware/go-ensure v1.0.0
	go.opencensus.io v0.22.5
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20201209123823-ac852fbbde11
	golang.org/x/oauth2 v0.0.0-20201208152858-08078c50e5b5
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20201211151036-40ec1c210f7a
	google.golang.org/grpc v1.34.0
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.18.12
	k8s.io/apimachinery v0.18.12
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/eventing v0.19.1-0.20201222225404-b482c5fad781
	knative.dev/hack v0.0.0-20201214230143-4ed1ecb8db24
	knative.dev/pkg v0.0.0-20201223002104-9d0775512af8
	knative.dev/serving v0.19.1-0.20201223170904-61531dc56905
	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/code-generator => k8s.io/code-generator v0.18.8
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d
)

// The following packages were pinned as part of the go module transition and should eventually be
// unpinned.
replace github.com/json-iterator/go => github.com/json-iterator/go v1.1.7

replace github.com/modern-go/reflect2 => github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742

replace github.com/pkg/errors => github.com/pkg/errors v0.8.1

replace github.com/robfig/cron/v3 => github.com/robfig/cron/v3 v3.0.0

replace gomodules.xyz/jsonpatch/v2 => gomodules.xyz/jsonpatch/v2 v2.0.1

replace gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.2.2

replace sigs.k8s.io/yaml => sigs.k8s.io/yaml v1.1.0
