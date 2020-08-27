# Accessing Metrics in Stackdriver

## Enable the Metrics API

In cloud console, navigate to
[`Monitoring > Metrics explorer`](https://console.cloud.google.com/monitoring/metrics-explorer).
Enable the monitoring metrics API and add your project to a workspace (or create
a new workspace).

## Add the Monitoring Metric Writer Role to the Dataplane Service Account

Determine the Google Service Account your data plane is running as. If you
followed [Install Knative-GCP](../install/install-knative-gcp.md) or
[Create a Service Account for the Data Plane](../install/dataplane-service-account.md),
then the Google Service Account will be named
`cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com`. The following command uses
that name. If the Google Service Account you are using is different, then
replace it before running the command.

```shell
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member=serviceAccount:cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com \
  --role roles/monitoring.metricWriter
```

## Enable Metrics in the `config-observability` ConfigMap

Edit the `config-observability` ConfigMap under the `cloud-run-events` namespace
in Cloud Console or with the following `kubectl` command:

```shell
kubectl edit configmap -n cloud-run-events config-observability
```

and add the following entries:

```
metrics.backend-destination: stackdriver
metrics.stackdriver-project-id: "<your stackdriver project id>" # Replace with your project's ID.
metrics.reporting-period-seconds: "60"
```

## Accessing metrics in Cloud Console

Navigate to
[`Monitoring > Metrics explorer`](https://console.cloud.google.com/monitoring/metrics-explorer)
and build a query to see metrics. An example query would look like:

- Resource type: `Cloud run for Anthos Broker`  
  Metric: `Broker event count`  
  Filter: `project_id="<your_project_id">`  
  Aggregator: `sum`

A graphical view should be displayed on canvas as the query result. The graph
can be viewed in different formats like Line, Stacked Bar, Stacked Area,
Heatmap, depending on the aggregator.
