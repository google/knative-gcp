# Cloud Run Events

Cloud Run Events builds on Kubernetes to enable easy consumption of events from
GCP services. It can be useful independently, or mixed with
[Knative](https://knative.dev).

To get started, [install Cloud Run Events](./install/README.md).

Then use one of the implemented resources,

- [PubSubSource.events.cloud.run/v1alpha1](./pubsubsource/README.md)

The sources are implemented to bridge events from GCP and translate that into an
HTTP POST to a `service` or `addressable` of your choosing; outbound Event is in
[CloudEvents](https://cloudevents.io) format.
