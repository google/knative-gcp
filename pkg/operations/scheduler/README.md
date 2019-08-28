# Scheduler Operations

Operations holds helper functions for creating and understanding Google
Cloud Scheduler related jobs that operate on managing Scheduler Jobs.

## Usage

### Directly

The Job that can be made:

- `NewJobOps` - Operations related to scheduler jobs.

Job accepts an Action, supported Actions are:

- `create`, will attempt to create the GCS notification
- `delete`, will attempt to delete the GCS notification

### Within Reconcilers

These jobs are used with Storage reconciler and provides the following methods:

- `EnsureSchedulerJob` - Confirm or Create SchedulerJob.
- `EnsureSchedulerJobDeleted` - Delete Scheduler Job.

## Why Jobs?

The `Jobs` are designed to be run in the namespace in which the `Scheduler` exists
with the auth provided. The `Job` has to run local to the resource to avoid
copying service accounts or secrets into the `cloud-run-events` namespace.

The controller will re-reconcile the resource if the create job is deleted, this
could be used as a healing operation by the operator if the Scheduler Job is
deleted using `gcloud` or the Cloud Console by mistake.
