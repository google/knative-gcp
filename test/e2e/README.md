# E2E Tests

Prow will run `./e2e-tests.sh` with authentication mechanism using Kubernetes
Secrets and `./e2e-wi-tests.sh` with authentication mechanism using Workload
Identity.

## Adding E2E Tests

E2E tests are tagged with `// +build e2e` but tagging a Go file this way will
prevent the compiler from compiling the test code. To work around this, for the
test code we separate them into different files:

```shell
test/e2e
├── e2e_test.go
└── test_xxx.go
```

- `e2e_test.go` is the testing file entry point (tagged with e2e).
- `test_xxx.go` are the test implementations (not tagged with e2e).

We leverage the
[test library in Eventing](https://github.com/knative/eventing/tree/master/test/lib)
as much as possible for implementing the e2e tests. Logic specific to
knative-gcp should be added under [knative-gcp e2e test lib](../lib).

## Setup a test cluster

Run the following command:

```shell
SKIP_TESTS=true ./test/e2e-tests.sh --skip-teardowns
```

`SKIP_TESTS` will skip the actual tests so only the cluster initialization is
run. `--skip-teardowns` tells the script to not tear down the cluster. This
command runs the cluster initialization logic and leaves the cluster in a state
that's ready to run tests.

If you run into
`Something went wrong: error creating deployer: --gcp-zone and --gcp-region cannot both be set`,
you may have set the `ZONE` environment variable. Try `unset ZONE`.

## Running E2E Tests on an existing cluster

### Prerequisites

There are two ways to set up authentication mechanism.

- (GKE Specific) If you want to run E2E tests with
  **[Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)**
  as the authentication mechanism, please follow below instructions to configure
  the authentication mechanism with
  **[Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)**.
- If you want to run E2E tests with **Kubernetes Secrets** as the authentication
  mechanism, please follow below instructions to configure the authentication
  mechanism with **Kubernetes Secrets**.

1.  A running Kubernetes cluster with
    [knative-gcp](../../docs/install/install-knative-gcp.md) installed and
    configured.
1.  Create a
    [Service Account for the Data Plane](../../docs/install/dataplane-service-account.md).
    Download a credential file and set `GOOGLE_APPLICATION_CREDENTIALS` env var.
    This is used by some tests(e.g., `TestSmokePullSubscription`) to authorize
    the Google SDK clients.
    ```
    cred_file=$(pwd)/cre-dataplane.json
    gcloud iam service-accounts keys create ${cred_file} --iam-account=cre-dataplane@$PROJECT_ID.iam.gserviceaccount.com
    export GOOGLE_APPLICATION_CREDENTIALS=${cred_file}
    ```
1.  [Install GCP Broker](../../docs/install/install-gcp-broker.md).
1.  [Broker with Pub/Sub Channel](../../docs/install/install-broker-with-pubsub-channel.md)
    installed.
1.  [CloudSchedulerSource Prerequisites](../../docs/examples/cloudschedulersource/README.md#prerequisites).
    Note that you only need to:
    1. Create with an App Engine application in your project.
1.  [CloudStorageSource Prerequisites](../../docs/examples/cloudstoragesource/README.md#prerequisites).
    Note that you only need to:
    1. Enable the Cloud Storage API on your project.
    1. Give Google Cloud Storage permissions to publish to GCP Pub/Sub.
1.  A docker repo containing [the test images](#test-images). Remember to
    specify the build tag `e2e`.
1.  (Optional) Note that if you plan on running metrics-related E2E tests using
    the StackDriver backend, you need to give your
    [Service Account](../../docs/install/dataplane-service-account.md) the
    `monitoring.metricWriter` role on your Google Cloud project:

    ```shell
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member=serviceAccount:${your_service_account}@$PROJECT_ID.iam.gserviceaccount.com \
      --role roles/monitoring.metricWriter
    ```

    If you also plan on running tracing-related E2E tests using the StackDriver
    backend, your
    [Service Account](../../docs/install/dataplane-service-account.md) needs
    additional `cloudtrace.admin` role:

    ```shell
    gcloud projects add-iam-policy-binding $PROJECT_ID \
      --member=serviceAccount:"${your_service_account}"@$PROJECT_ID.iam.gserviceaccount.com \
      --role roles/cloudtrace.admin
    ```

1.  (Optional) Note that if plan on running tracing-related E2E tests using the
    Zipkin backend, you need to install
    [zipkin-in-mem](https://github.com/knative/serving/tree/master/config/monitoring/tracing/zipkin-in-mem)
    and patch the configmap `config-tracing` in the `knative-eventing` namespace
    to use the Zipkin backend as the with
    [patch-config-tracing-configmap-with-zipkin.yaml](../../docs/install/patch-config-tracing-configmap-with-zipkin.yaml).

    ```shell
    kubectl patch configmap config-tracing -n knative-eventing --patch "\$(cat patch-config-tracing-configmap-with-zipkin.yaml)"
    ```

### Running E2E tests

### Running E2E tests with authentication mechanism using Kubernetes Secrets

```shell
E2E_PROJECT_ID=<project name> \
  go test --tags=e2e ./test/e2e/...
```

And count is supported too:

```shell
E2E_PROJECT_ID=<project name> \
  go test --tags=e2e ./test/e2e/... --count=3
```

If you want to run a specific test:

```shell
E2E_PROJECT_ID=<project name> \
  go test --tags=e2e ./test/e2e/... -run NameOfTest
```

For example, to run TestPullSubscription:

```shell
E2E_PROJECT_ID=<project name> \
  go test --tags=e2e ./test/e2e/... -run TestPullSubscription
```

### Running E2E tests with authentication mechanism using Workload Identity.

First, you'll have to modify `clusterDefaults` in ConfigMap `config-gcp-auth`.

You can directly edit the ConfigMap by:

```shell
kubectl edit configmap config-gcp-auth -n cloud-run-events
```

and replace the `default-auth-config:` part with:

```shell
  default-auth-config: |
    clusterDefaults:
      serviceAccountName: test-default-ksa
      workloadIdentityMapping:
        test-default-ksa: $PUBSUB_SERVICE_ACCOUNT@$PROJECT_ID.iam.gserviceaccount.com
```

`$PUBSUB_SERVICE_ACCOUNT@$PROJECT_ID.iam.gserviceaccount.com` is the Pub/Sub
enabled Google Cloud Service Account.

Then, add `-workloadIdentity=true` and `-serviceAccountName=test-default-ksa` to
the `go test` command.

For example,

```shell
E2E_PROJECT_ID=<project name> go test --tags=e2e ./test/e2e/... \
  -workloadIdentity=true \
  -serviceAccountName=test-default-ksa \
  -run TestPullSubscription
```

## Running E2E Tests on an new cluster

### Prerequisites

1. Enable necessary APIs:

   ```shell
   gcloud services enable compute.googleapis.com
   gcloud services enable container.googleapis.com
   ```

1. Install
   [kubetest](https://github.com/kubernetes/test-infra/issues/15700#issuecomment-571114504).
   (Note this is just a workaround because of
   [kubernetes issue](https://github.com/kubernetes/test-infra/issues/15700)

1. Set the project you want to run E2E tests to be the default one with:

   ```shell
   export PROJECT=<REPLACE_ME>
   gcloud config set core/project $PROJECT
   ```

### Running E2E tests

If you want to run E2E tests with authentication mechanism using **Kubernetes
Secrets**:

```shell
./test/e2e-tests.sh
```

If you want to run E2E tests with authentication mechanism using **Workload
Identity**:

```shell
./test/e2e-wi-tests.sh
```

## Test images

### Building the test images

_Note: this is only required when you run e2e tests locally with `go test`
commands. Running tests through e2e-tests.sh will publish the images
automatically._

The [`upload-test-images.sh`](./../upload-test-images.sh) script can be used to
build and push the test images used by the e2e tests. It requires:

- [`KO_DOCKER_REPO`](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup)
  to be set
- You need to be
  [authenticated with your `KO_DOCKER_REPO`](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup)
- [`docker`](https://docs.docker.com/install/) to be installed

For images deployed in GCR, a docker tag is mandatory to avoid issues with using
`latest` tag:

```bash
./test/upload-test-images.sh ./test/test_images e2e
sed -i 's@ko://knative.dev/eventing/test/test_images@ko://github.com/google/knative-gcp/vendor/knative.dev/eventing/test/test_images@g' vendor/knative.dev/eventing/test/test_images/*/*.yaml
./test/upload-test-images.sh ./vendor/knative.dev/eventing/test/test_images/ e2e
```

To run the script for all end to end test images:

```bash
./test/upload-test-images.sh ./test/test_images
sed -i 's@ko://knative.dev/eventing/test/test_images@ko://github.com/google/knative-gcp/vendor/knative.dev/eventing/test/test_images@g' vendor/knative.dev/eventing/test/test_images/*/*.yaml
./test/upload-test-images.sh ./vendor/knative.dev/eventing/test/test_images/
```

### Adding new test images

New test images should be placed in `./test/test_images`. For each image create
a new sub-folder and include a Go file that will be an entry point to the
application. This Go file should use the package `main` and include the function
`main()`. It is a good practice to include a `README` file as well. When
uploading test images, `ko` will build an image from this folder and upload to
the Docker repository configured as
[`KO_DOCKER_REPO`](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup).

## Troubleshooting E2E Tests

### Prow

Each PR will trigger [E2E tests](../../test/e2e). For failed tests, follow the
prow links on the PR page. Such links are in the format of
`https://prow.knative.dev/view/gcs/knative-prow/pr-logs/pull/google_knative-gcp/[PR ID]/[TEST NAME]/[TEST ID]`
, e.g.
`https://prow.knative.dev/view/gcs/knative-prow/pr-logs/pull/google_knative-gcp/1153/pull-google-knative-gcp-integration-tests/1267481606424104960`
.

If the prow page doesn't provide any useful information, check out the full logs
dump.

- The control plane pods (in `cloud-run-events` namespace) logs dump are at
  `https://console.cloud.google.com/storage/browser/knative-prow/pr-logs/pull/google_knative-gcp/[PR ID]/[TEST NAME]/[TEST ID]/artifacts/controller-logs/`
  .
- The data plane pods logs dump are at
  `https://console.cloud.google.com/storage/browser/knative-prow/pr-logs/pull/google_knative-gcp/[PR ID]/[TEST NAME]/[TEST ID]/artifacts/pod-logs/`
  .

### Local

Add `CI=true`to the `go test` command.

- The data plane pods logs dump are at
  `$GOPATH/src/github.com/google/knative-gcp/test/e2e/artifacts/pod-logs` .

For example:

```shell
CI=true E2E_PROJECT_ID=$PROJECT_ID \
 go test --tags=e2e ./test/e2e/...
```
