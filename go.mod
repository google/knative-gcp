module github.com/google/knative-gcp

go 1.15

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
	github.com/google/uuid v1.2.0
	github.com/google/wire v0.4.0
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/pkg/errors v0.9.1
	github.com/rickb777/date v1.13.0
	go.opencensus.io v0.22.6
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	golang.org/x/oauth2 v0.0.0-20210126194326-f9ce19ea3013
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d
	google.golang.org/grpc v1.35.0
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	knative.dev/eventing v0.20.1-0.20210219190903-207aa2673903
	knative.dev/hack v0.0.0-20210203173706-8368e1f6eacf
	knative.dev/pkg v0.0.0-20210216013737-584933f8280b
	knative.dev/serving v0.20.1-0.20210223012946-6ef778d32302
	sigs.k8s.io/yaml v1.2.0
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
