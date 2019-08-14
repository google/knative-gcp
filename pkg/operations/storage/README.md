# Storage Operations

Operations holds helper functions for creating and understanding GCS related
jobs that operate on configuring notifications on GCS.

## Usage

### Directly

There are two Jobs that can be made:

- `NewNotificationOps` - Operations related to notifications.

Each Job accepts an Action, supported Actions are:

- `create`, will attempt to create the GCS notification
- `delete`, will attempt to delete the GCS notification

### Within Reconcilers

These jobs are used with Storage reconciler and provides the following methods:

- `EnsureNotification` - Confirm or Create notification.
- `EnsureNotificationDeleted` - Delete notification.

## Why Jobs?

The `Jobs` are designed to be run in the namespace in which the
`Storage` exists with the auth provided. The `Job` has to run local to
the resource to avoid copying service accounts or secrets into the
`cloud-run-events` namespace.

The controller will re-reconcile the resource if the create job is deleted, this
could be used as a healing operation by the operator if the GCS notification
is deleted using `gsutil` or the Cloud Console by mistake.
