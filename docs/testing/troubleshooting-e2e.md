# Troubleshooting E2E Tests

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
