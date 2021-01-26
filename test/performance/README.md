# Run Performance Test Locally

## Prepare Benchmark Access

To be able to publish data to `Mako`, you will have to add yourself to the
allowlist. Currently `Mako` only allows Google accounts to publish data to the
benchmark.

1. Create a service account for data upload, e.g.
   `mako-upload@jimmy-knative-dev.iam.gserviceaccount.com`.
1. Add the new service account and your google account to the owner_list of the
   benchmark configuration files like
   [here](./benchmarks/broker-gcp/dev.config).
1. Allowlist both of your accounts by sending request to the
   [Mako](https://github.com/google/mako) team.
1. Ask an existing owner to update the benchmark.

## Run Performance Test with Mako

Make sure your accounts are already in the allowlist for the benchmark.

1. Create a cluster and install knative gcp.
1. Generate a key for your `mako-upload` service account.
1. Create a ConfigMap called config-mako in your chosen namespace containing the
   Mako config file.

   ```
   kubectl create configmap config-mako --from-file=test/performance/benchmarks/<benchmark>/dev.config
   ```

1. Create a Kubernetes secret `mako-secrets` with key `robot.json`:
   ```
   kubectl create secret generic mako-secrets --from-file=robot.json={YOUR_KEY_FILE}
   ```
1. Deploy the benchmark configurations in the continuous directory.
   ```
   ko apply -f test/performance/benchmarks/<benchmark>/continuous
   ```

## Run Performance Test without Mako

To run without Mako, follow the instructions
[here](https://github.com/knative/eventing/tree/master/test/performance).
