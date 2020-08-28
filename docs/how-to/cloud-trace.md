# Accessing Event Traces in Cloud Trace

## Enable the Cloud Trace API

GCP projects have the Cloud Trace API enabled by default. If it has been
disabled, follow
[these instructions](https://cloud.google.com/trace/docs/setup#gcp-config) to
enable the API.

## Add the Cloud Trace Agent Role to Dataplane Service Accounts

Follow
[these instructions](../install/dataplane-service-account.md#create-a-google-cloud-service-account-to-interact-with-pubsub)
to grant dataplane service accounts `roles/cloudtrace.agent`.

## Enable Tracing in the `config-tracing` ConfigMap

Edit the `config-tracing` ConfigMap under the `cloud-run-events` namespace in
Cloud Console or with the following `kubectl` command:

```shell
kubectl edit configmap -n cloud-run-events config-tracing
```

and add the following entries to the data section:

```
data:
  backend: stackdriver
  stackdriver-project-id: <project-id> # Replace with your project's ID.
  sample-rate: 0.01 # Replace with the rate you want. Currently 1%.
```

## Accessing Traces in Cloud Console

_Note:_ The following instructions are written for the Trace list UI preview.
Filter syntax will differ when using the classic Trace list UI.

Navigate to
[`Tools > Trace > Trace list`](https://console.cloud.google.com/traces/list) and
add `messaging.system:knative` to the trace filter to select only knative
eventing traces. To search for traces through a particular broker or trigger,
the following filter can be added:

```
SpanName:Recv.broker:<broker-name>.<broker-namespace>
SpanName:trigger:<trigger-name>.<trigger-namespace>
```

The following additional filters can be used to filter by event attributes:

```
messaging.message_id:<event-id>
cloudevents.type:<event-type>
cloudevents.source<event-source>
cloudevents.subject<event-subject>
cloudevents.datacontenttype:<content-type>
cloudevents.specversion:<cloudevent-version>
```
