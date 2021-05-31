module github.com/google/knative-gcp

go 1.15

require (
	cloud.google.com/go v0.72.0
	cloud.google.com/go/logging v1.0.1-0.20200331222814-69e77e66e597
	cloud.google.com/go/pubsub v1.8.0
	cloud.google.com/go/storage v1.10.0
	github.com/cloudevents/sdk-go/observability/opencensus/v2 v2.4.1
	github.com/cloudevents/sdk-go/protocol/pubsub/v2 v2.2.1-0.20200806165906-9ae0708e27fa
	github.com/cloudevents/sdk-go/v2 v2.4.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.2.0
	github.com/google/wire v0.4.0
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/lightstep/tracecontext.go v0.0.0-20181129014701-1757c391b1ac
	github.com/pkg/errors v0.9.1
	github.com/rickb777/date v1.13.0
	go.opencensus.io v0.23.0
	go.uber.org/multierr v1.6.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	golang.org/x/oauth2 v0.0.0-20210514164344-f6687ab2804c
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20210416161957-9910b6c460de
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	knative.dev/eventing v0.23.1-0.20210531083832-65bed404d6ca
	knative.dev/hack v0.0.0-20210428122153-93ad9129c268
	knative.dev/pkg v0.0.0-20210528203030-47dfdcfaedfd
	knative.dev/serving v0.23.1-0.20210531083132-a9aeafb95ea1
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
