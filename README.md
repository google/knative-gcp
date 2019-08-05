# Knative + Google Cloud Platform = 🚀

[![GoDoc](https://godoc.org/github.com/google/knative-gcp?status.svg)](https://godoc.org/github.com/google/knative-gcp)
[![Go Report Card](https://goreportcard.com/badge/google/knative-gcp)](https://goreportcard.com/report/google/knative-gcp)
[![LICENSE](https://img.shields.io/github/license/google/knative-gcp.svg)](https://github.com/google/knative-gcp/blob/master/LICENSE)

Knative with GCP builds on Kubernetes to enable easy configuration and
consumption of Google Cloud Platform events and services. It can be useful
independently, but is best mixed with [Knative](https://knative.dev).

To get started, [install Knative with GCP](./docs/install/README.md).

Then use one of the implemented resources,

- [PullSubscription (pubsub.cloud.run/v1alpha1)](docs/pullsubscription/README.md)
- [Topic (pubsub.cloud.run/v1alpha1)](docs/topic/README.md)
- [Channel (messaging.cloud.run/v1alpha1)](docs/channel/README.md)
