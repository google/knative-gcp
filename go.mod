module github.com/google/knative-gcp

go 1.13

require (
	cloud.google.com/go v0.56.0
	cloud.google.com/go/logging v1.0.1-0.20200331222814-69e77e66e597
	cloud.google.com/go/pubsub v1.3.2-0.20200331222814-69e77e66e597
	cloud.google.com/go/storage v1.6.1-0.20200331222814-69e77e66e597
	github.com/cloudevents/sdk-go v1.2.0
	github.com/cloudevents/sdk-go/v2 v2.0.0-RC2
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.0
	github.com/google/go-cmp v0.4.0
	github.com/google/uuid v1.1.1
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/pkg/errors v0.9.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/otel v0.3.0 // indirect
	go.uber.org/multierr v1.2.0
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20200317142112-1b76d66859c6 // indirect
	google.golang.org/api v0.22.1-0.20200430202532-ac9be1f8f530
	google.golang.org/genproto v0.0.0-20200430143042-b979b6f78d84
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.21.0
	istio.io/api v0.0.0-20200227213531-891bf31f3c32
	istio.io/client-go v0.0.0-20200227214646-23b87b42e49b
	istio.io/gogo-genproto v0.0.0-20200130224810-a0338448499a // indirect
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.18.1
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d // indirect
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f // indirect
	knative.dev/eventing v0.14.1-0.20200501170243-0bb51bb8d62b
	knative.dev/pkg v0.0.0-20200501164043-2e4e82aa49f1
	knative.dev/serving v0.14.1-0.20200424135249-b16b68297056
)

replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.12.9-0.20191108183826-59d068f8d8ff
	go.opencensus.io => go.opencensus.io v0.22.1
	istio.io/api => istio.io/api v0.0.0-20200227213531-891bf31f3c32
	istio.io/client-go => istio.io/client-go v0.0.0-20200227214646-23b87b42e49b
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.5-beta.1
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.5-beta.1
	k8s.io/gengo => k8s.io/gengo v0.0.0-20190907103519-ebc107f98eab
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d
)

replace github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.25.1

replace github.com/blang/semver => github.com/blang/semver v1.1.1-0.20190414102917-ba2c2ddd8906
