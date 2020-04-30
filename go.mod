module github.com/google/knative-gcp

go 1.13

require (
	cloud.google.com/go v0.55.0
	cloud.google.com/go/logging v1.0.0
	cloud.google.com/go/pubsub v1.3.1
	cloud.google.com/go/storage v1.6.0
	github.com/cloudevents/sdk-go v1.2.0
	github.com/cloudevents/sdk-go/v2 v2.0.0-RC2
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.0
	github.com/google/go-cmp v0.4.0
	github.com/google/go-containerregistry v0.0.0-20200331213917-3d03ed9b1ca2 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gax-go/v2 v2.0.5
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/pkg/errors v0.9.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/otel v0.3.0 // indirect
	go.uber.org/multierr v1.2.0
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20200317142112-1b76d66859c6 // indirect
	google.golang.org/api v0.20.0
	google.golang.org/genproto v0.0.0-20200326112834-f447254575fd
	google.golang.org/grpc v1.28.0
	google.golang.org/protobuf v1.21.0
	istio.io/api v0.0.0-20200227213531-891bf31f3c32
	istio.io/client-go v0.0.0-20200227214646-23b87b42e49b
	istio.io/gogo-genproto v0.0.0-20200130224810-a0338448499a // indirect
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/eventing v0.14.1-0.20200430203842-9bdaf658626b
	knative.dev/pkg v0.0.0-20200429233442-1ebb4d56f726
	knative.dev/serving v0.14.1-0.20200424135249-b16b68297056
)

replace (
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
)
