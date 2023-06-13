# iam-drift-detection

GitHub workflow that will automatically detect IAM drift between your terraform
state files located in GCS Buckets and the actual IAM of your resources in GCP.

If a drift is detected a GitHub Issue will be created with details of which
resources are/aren't managed in terraform.

## Features

* Automatically determines TF state buckets via gcs bucket labels (`terraform=true`).
* Checks for drift on all gcp folders in the org.
* Checks for drift on all gcp projects in the org.
* Allows ignoring resources by adding a `.driftignore` file containing a list of resource IDs.

## Future improvements

* Parallelize checks.
* Handle iam_policy terraform resources
* Handle various resource ID formats.
