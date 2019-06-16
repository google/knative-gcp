# Cloud Run Events

Cloud Run Events builds on Kubernetes to enable easy consumption of events from
Google Cloud services. It can be useful independently, or mixed with
[Knative](https://knative.dev).

To get started, [install Cloud Run Events](./docs/install/README.md).

Then use one of the implemented resources,

- [PullSubscription (pubsub.cloud.run/v1alpha1)](docs/pullsubscription/README.md)

The sources are implemented to bridge events from Google Cloud and translate that into an
HTTP POST to a `service` or `addressable` of your choosing; outbound Event is in
[CloudEvents](https://cloudevents.io) format.
