# Knative with GCP Metrics

_All commands assume root of repo._

## Prometheus Collection

1. Enable Knatives install of Prometheus to scrape Knative with GCP, run the
   following:

   ```shell
   kubectl get configmap -n  knative-monitoring prometheus-scrape-config -oyaml > tmp.prometheus-scrape-config.yaml
   sed -e 's/^/    /' config/monitoring/metrics/prometheus/prometheus-scrape-kn-gcp.yaml > tmp.prometheus-scrape-kn-gcp.yaml
   sed -e '/    scrape_configs:/r tmp.prometheus-scrape-kn-gcp.yaml' tmp.prometheus-scrape-config.yaml \
     | kubectl apply -f -
   ```

1. To verify, run the following to show the diff between what the resource was
   original and is now (_note_: k8s will update `metadata.annotations`):

   ```shell
   kubectl get configmap -n  knative-monitoring prometheus-scrape-config -oyaml \
     | diff - tmp.prometheus-scrape-config.yaml
   ```

   Or, to just see our changes (without `metadata.annotations`) run:

   ```shell
   CHANGED_LINES=$(echo $(cat  tmp.prometheus-scrape-kn-gcp.yaml | wc -l) + 1 | bc)
   kubectl get configmap -n  knative-monitoring prometheus-scrape-config -oyaml \
     | diff - tmp.prometheus-scrape-config.yaml \
     | head -n $CHANGED_LINES
   ```

1. Restart Prometheus

   To pick up this new config, the pods that run Prometheus need to be
   restarted, run:

   ```shell
   kubectl delete pods -n knative-monitoring prometheus-system-0 prometheus-system-1
   ```

   And they will come back:

   ```shell
   $ kubectl get pods -n knative-monitoring
   NAME                                 READY   STATUS    RESTARTS   AGE
   grafana-d7478555c-8qgf7              1/1     Running   0          22h
   kube-state-metrics-765d876c6-z7dfn   4/4     Running   0          22h
   node-exporter-5m9cz                  2/2     Running   0          22h
   node-exporter-z59gz                  2/2     Running   0          22h
   prometheus-system-0                  1/1     Running   0          32s
   prometheus-system-1                  1/1     Running   0          36s
   ```

1. To remove the temp files, run:

   ```shell
   rm tmp.prometheus-scrape-kn-gcp.yaml
   rm tmp.prometheus-scrape-config.yaml
   ```

#### Remove Scrape Config

Remove the text related to Cloud Run Events from `prometheus-scrape-config`,

```shell
kubectl edit configmap -n  knative-monitoring prometheus-scrape-config
```

And then restart Prometheus.

## Grafana Dashboard

To enable the Knative with GCP dashboard in Grafana, run the following:

```shell
kubectl patch  configmap grafana-dashboard-definition-knative -n knative-monitoring \
  --patch "$(cat config/monitoring/metrics/grafana/100-grafana-dash-kn-gcp.yaml)"
```

## Accessing Prometheus and Grafana

#### Prometheus port forwarding:

```shell
kubectl port-forward -n knative-monitoring \
  $(kubectl get pods -n knative-monitoring --selector=app=prometheus --output=jsonpath="{.items[0].metadata.name}") \
  9090
```

Then, access the [Prometheus Dashboard](http://localhost:9090)

#### Grafana port forwarding:

```shell
kubectl port-forward --namespace knative-monitoring \
  $(kubectl get pods --namespace knative-monitoring --selector=app=grafana --output=jsonpath="{.items..metadata.name}") \
  3000
```

Then, access the [Grafana Dashboard](http://localhost:3000)

## StackDriver Collection

1.  Install Knative Stackdriver components by running the following command from
    the root directory of [knative/serving](https://github.com/knative/serving)
    repository:

    ```shell
          kubectl apply --recursive --filename config/monitoring/100-namespace.yaml \
              --filename config/monitoring/metrics/stackdriver
    ```

1.  Set up the [Pub/Sub Enabled Service Account](../../../docs/pubsub/README.md)
    with StackDriver Monitoring permissions.

1.  Run the following command to setup StackDriver as the metrics backend:

    ```shell
       kubectl edit cm -n cloud-run-events config-observability
    ```

Add `metrics.backend-destination: stackdriver`,
`metrics.allow-stackdriver-custom-metrics: "true"` and
`metrics.stackdriver-custom-metrics-subdomain: "cloud.google.com/events"` to the `data`
field.  
 You can find detailed information in `data._example` field in the `ConfigMap` you
are editing.

1. Open the StackDriver UI and see your resource metrics in the StackDriver
   Metrics Explorer. You should be able to see metrics with the prefix
   `custom.googleapis.com/cloud.google.com/events`.
