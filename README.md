# Knative + Google Cloud Platform = 🚀

[![GoDoc](https://godoc.org/github.com/google/knative-gcp?status.svg)](https://godoc.org/github.com/google/knative-gcp)
[![Go Report Card](https://goreportcard.com/badge/google/knative-gcp)](https://goreportcard.com/report/google/knative-gcp)
[![LICENSE](https://img.shields.io/github/license/google/knative-gcp.svg)](https://github.com/google/knative-gcp/blob/master/LICENSE)

Knative with GCP builds on Kubernetes to enable easy configuration and
consumption of Google Cloud Platform events and services. It can be useful
independently, but is best mixed with [Knative](https://knative.dev).

To get started, [install Knative with GCP](./docs/install/README.md).

Then use one of the implemented Sources:

- [PubSub (events.cloud.google.com)](docs/pubsub/README.md)
- [Storage (events.cloud.google.com)](docs/storage/README.md)
- [Scheduler (events.cloud.google.com)](docs/scheduler/README.md)
- [CloudAuditLogsSource (events.cloud.google.com)](docs/examples/cloudauditlogssource/README.md)

To use a Knative Eventing Channel backed by Pub/Sub:

- [Channel (messaging.cloud.google.com)](docs/examples/channel/README.md)

To leverage Pub/Sub directly, use one of the Pub/Sub resources:

- [PullSubscription (pubsub.cloud.google.com)](docs/examples/pullsubscription/README.md)
- [Topic (pubsub.cloud.google.com)](docs/examples/topic/README.md)


_Note:_ This repo is still in development, APIs and resource names are subject to change in the future.
