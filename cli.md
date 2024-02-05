# Guardian CLI

Supported commands:

| **Command**                 | **Subcommand**                                          | **Required Github Permission**                              | **Description**                                               |
|-----------------------------|---------------------------------------------------------|-------------------------------------------------------------|---------------------------------------------------------------|
| [entrypoints](#entrypoints) |                                                         | `contents: read`, `pull-requests: write`                    | Determine the entrypoint directories to run Guardian commands |
| [apply](#apply)             |                                                         | `contents: read`, `pull-requests: write`, `id-token: write` | Run Terraform apply for a directory                           |
| [plan](#plan)               |                                                         | `contents: read`, `pull-requests: write`, `id-token: write` | Run Terraform plan for a directory                            |
| [run](#run)                 |                                                         | none                                                        | Run a Terraform command for a directory                       |
| iam                         | [cleanup](#iam-cleanup)                                 | none                                                        | Remove any expired IAM in a GCP organization                  |
|                             | [detect-drift](#iam-detect-drift)                       | `issues: write`                                             | Detect IAM drift in a GCP organization                        |
| drift                       | [statefiles](#drift-statefiles)                         | `issues: write`, `contents: read`                           | Detect drift for terraform statefiles                         |
| workflows                   | [plan-status-comment](#workflows-plan-status-comment)   | `pull-requests: write`                                      | Add Guardian plan comment to a pull request                   |
|                             | [remove-plan-comments](#workflows-remove-plan-comments) | `pull-requests: write`                                      | Remove previous Guardian plan comments from a pull request    |
|                             | [validate-permissions](#workflows-validate-permissions) | `contents: read`                                            | Validate required permissions for the current GitHub workflow |

## Shared Options

These options influence any command run with Guardian.

### GitHub Options

These options influence how Guardian interacts with GitHub:

* **-github-actions** - Is this running as a GitHub action. The default value is "false".
  This option can also be specified with the GITHUB_ACTIONS environment variable.
* **-github-owner="organization-name"** - The GitHub repository owner.
* **-github-repo="repository-name"** - The GitHub repository name.
* **-github-token="string"** - The GitHub access token to make GitHub API calls. This
  value is automatically set on GitHub Actions. This option can also be specified with
  the GITHUB_TOKEN environment variable.

### Retry Options

These options influence how Guardian attempts to retry failed requests:

* **-retry-initial-delay="10s"** - The initial duration to wait before retrying any
  failures. The default value is "2s".
* **-retry-max-attempts="1"** - The maximum number of attempts to retry any failures.
  The default value is "3".
* **-retry-max-delay="5m"** - The maximum duration to wait before retrying any
  failures. The default value is "1m".

## Entrypoints

Determine the entrypoint directories to run Guardian commands.

Usage: guardian entrypoints [options] <directory>

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-dest-ref="ref-name"** - The destination GitHub ref name for finding file changes.
* **-detect-changes** - Detect file changes, including all local module dependencies,
  and run for all entrypoint directories. The default value is "false".
* **-fail-unresolvable-modules** - Whether or not to error if a module cannot be
  resolved. The default value is "false".
* **-format="json"** - The format to print the output directories. The supported
  formats are: [json text]. The default value is "text".
* **-max-depth="int"** - How far to traverse the filesystem beneath the target
  directory for entrypoints. The default value is "-1".
* **-pull-request-number="100"** - The GitHub pull request number associated with
  this plan. The default value is "0".
* **-source-ref="ref-name"** - The source GitHub ref name for finding file changes.

## Apply

Run Terraform apply for a directory.

Usage: guardian apply [options] <directory>.

### Prerequisites

* The environment where you run this command must have Terraform installed locally.
* Write permission to the target GitHub repository `pull-requests`.
* The appropriate permissions to change all resources in your terraform configuration
  (e.g. write access to all GCP/AWS/GitHub resources in your terraform).
* The user must be authenticated to the appropriate provider (e.g. for GCP they must
  have run gcloud auth).

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-allow-lockfile-changes** - Allow modification of the Terraform lockfile. The default value is "false".
* **-bucket-name="my-guardian-state-bucket"** - The Google Cloud Storage bucket name to store Guardian plan files.
* **-commit-sha="e538db9a29f2ff7a404a2ef40bb62a6df88c98c1"** - The commit sha to determine
  the pull request number associated with this apply. Only one of pull-request-number
  and commit-sha can be given.
* **-lock-timeout="10m"** - The duration Terraform should wait to obtain a lock when
  running commands that modify state. The default value is "10m".
* **-pull-request-number="100"** The GitHub pull request number associated with this
  apply. Only one of pull-request-number and commit-sha can be given. The default value is "0".

## Plan

Run Terraform plan for a directory.

Usage: guardian plan [options] <directory>

### Prerequisites

* The environment where you run this command must have Terraform installed locally.
* Write permission to the target GitHub repository `pull-requests`.
* The appropriate permissions to view all resources in your terraform configuration
  (e.g. read access to all GCP/AWS/GitHub resources in your terraform).
* The user must be authenticated to the appropriate provider (e.g. for GCP they must
  have run gcloud auth).

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-allow-lockfile-changes** - Allow modification of the Terraform lockfile. The default value is "false".
* **-bucket-name="my-guardian-state-bucket"** - The Google Cloud Storage bucket name to store Guardian plan files.
* **-lock-timeout="10m"** - The duration Terraform should wait to obtain a lock when
  running commands that modify state. The default value is "10m".
* **-pull-request-number="100"** The GitHub pull request number associated with this
  plan. Only one of pull-request-number and commit-sha can be given. The default value is "0".

## Run

Run a Terraform command for a directory.

Usage: guardian run [options] <directory>

### Prerequisites

* The environment where you run this command must have Terraform installed locally.
* The appropriate permissions to view/change all resources in your terraform configuration
  (e.g. read access to all GCP/AWS/GitHub resources in your terraform).
* The user must be authenticated to the appropriate provider (e.g. for GCP they must
  have run gcloud auth).

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-allow-lockfile-changes** - Allow modification of the Terraform lockfile. The default value is "false".
* **-bucket-name="my-guardian-state-bucket"** - The Google Cloud Storage bucket name to store Guardian plan files.
* **-commit-sha="e538db9a29f2ff7a404a2ef40bb62a6df88c98c1"** - The commit sha to determine
  the pull request number associated with this apply run. Only one of pull-request-number
  and commit-sha can be given.
* **-lock-timeout="10m"** - The duration Terraform should wait to obtain a lock when
  running commands that modify state. The default value is "10m".
* **-pull-request-number="100"** The GitHub pull request number associated with this
  apply run. Only one of pull-request-number and commit-sha can be given. The default value is "0".

## IAM cleanup

Cleanup expired IAM in a GCP organization.

Usage: guardian iam cleanup [options]

### Prerequisites

The actor that runs this command must have:

* A GCP project with Asset Inventory API enabled (`cloudasset.googleapis.com`)
  and Resource Manager enabled (`cloudresourcemanager.googleapis.com`).
  If running as yourself, be sure to set this as your default project via gcloud.
* Authentication to GCP via gcloud auth.
* Read-access to view all IAM for all projects, folders, and as well as organization-level
  IAM for the organization in question.
* Write-access to change all IAM for all projects, folders, and as well as organization-level
  IAM for the organization in question.

### Options

Also supports [Retry Options](#retry-options).

* **-disable-evaluate-condition** - Whether or not to evaluate the IAM Condition Expression
  and only delete those IAM with false evaluation. Defaults to false. Example: An IAM
  condition with expression `request.time < timestamp("2019-01-01T00:00:00Z")` will
  evaluate to false and the IAM will be deleted. The default value is "false".
* **-iam-query="policy:abcxyz-aod-expiry"** - The query to use to filter on IAM.
* **-max-conncurrent-requests="2"** - The maximum number of concurrent requests
  allowed at any time to GCP. The default value is "10".
* **-scope="123435456456"** - The scope to cleanup IAM for - organizations/123456 will
  cleanup all IAM matching your query in the organization and all folders and projects beneath it.

## IAM detect-drift

Detect IAM drift in a GCP organization.

Usage: guardian iam detect-drift [options]

### Prerequisites

The actor that runs this command must have:

* A GCP project with Asset Inventory API enabled (`cloudasset.googleapis.com`)
  and Resource Manager enabled (`cloudresourcemanager.googleapis.com`).
  If running as yourself, be sure to set this as your default project via gcloud.
* Authentication to GCP via gcloud auth.
* Read-access to view all IAM for all projects, folders, and as well as organization-level
  IAM for the organization in question.
* Write permission to the target GitHub repository `issues`.

### Options

Also supports [GitHub Options](#github-options).

* **-driftignore-file=".driftignore"** - The driftignore file to use which contains
  values to ignore. The default value is ".driftignore". See
  [Using driftignore](#using-driftignore) for more details.
* **-gcs-bucket-query="labels.terraform:*"** - The label to use to find GCS buckets
  with Terraform statefiles.
* **-max-conncurrent-requests="10"** - The maximum number of concurrent requests
  allowed at any time to GCP. The default value is "10".
* **-organization-id="123435456456"** - The Google Cloud organization ID for which
  to detect drift.
* **-github-comment-message-append="@dcreey, @my-org/my-team"** - Any arbitrary
  string message to append to the drift GitHub comment.
* **-github-issue-assignees="dcreey"** - The assignees to assign to for any created
  GitHub Issues.
* **-github-issue-labels="guardian-iam-drift"** - The labels to use on any created
  GitHub Issues.
* **-skip-github-issue** - Whether or not to create a GitHub Issue when a drift is
  detected. The default value is "false".

### Using driftignore

With a `.driftignore` file you can define iam resources that you do not want to be
alerted for. This is especially useful for resources that are configured outside
of terraform. Put this file at the root of your repository or indicate its location
using the `-driftignore-file` option.

#### Supported syntax:

Each line in your `.driftignore` file can contain one of the following

* `/organizations/{number}/projects/{name-or-number}` - Ignores all IAM for this
  GCP project.
* `/organizations/{number}/folders/{number}` - Ignores all IAM for this GCP folder
  and all folders and projects beneath it.
* `/roles/{role}/{member}` - Ignores all IAM in any GCP project, folder, or org
  that matches this role & membership pair.
* `{iam-uri}` - The full IAM uri as shown in the generated IAM drift GitHub issue.

#### Example driftignore

```
# Ignore the 555555555567 folder because it is a customer tenant folder managed by a service
/organizations/555555555555/folders/555555555567
# my-service-account SA is the owner by default of every project because it creates all projects
/roles/owner/serviceAccount:my-service-account@some-project.iam.gserviceaccount.com
# my-service-account SA is the owner by default of every project because it creates all projects
/roles/resourcemanager.folderAdmin/serviceAccount:my-service-account@some-project.iam.gserviceaccount.com
/roles/resourcemanager.folderEditor/serviceAccount:my-service-account@some-project.iam.gserviceaccount.com
# Project that doesn't use terraform
/organizations/555555555555/projects/my-click-ops-project
# Ignore this particular IAM resource
/organizations/555555555555/projects/some-project/roles/storage.admin/user:me@google.com
```

## Drift Statefiles

Run the drift detection for terraform statefiles in a directory.

Usage: guardian drift statefiles [options] <directory>

### Prerequisites

The actor that runs this command must have:

* A GCP project with Asset Inventory API enabled (`cloudasset.googleapis.com`)
  and Resource Manager API enabled (`cloudresourcemanager.googleapis.com`).
  If running as yourself, be sure to set this as your default project via gcloud.
* Authentication to GCP via gcloud auth.
* Read-access to view all IAM for all projects, folders, and as well as organization-level
  IAM for the organization in question.
* Write permission to the target GitHub repository `issues`.
* Read permissions to clone all GitHub repositories containing relevant terraform in the
  target GitHub organization (if necessary).

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-detect-gcs-buckets-from-terraform** - Whether or not to use the terraform
  backend configs to determine gcs buckets. The default value is "false".
* **-gcs-bucket-query="labels.terraform:*"** - The label to use to find GCS
  buckets with Terraform statefiles.
* **-github-repo-terraform-topics="terraform,guardian"** - Topics to use to
  identify github repositories that contain terraform configurations.
* **-ignore-dir-patterns="templates\\/&ast;&ast;,test\\/&ast;&ast;"** - Directories to filter
  from the possible terraform entrypoint locations. Paths will be matched against
  the root of each cloned repository.
* **-organization-id="123435456456"** - The Google Cloud organization ID for which
  to detect drift.
* **-github-comment-message-append="@dcreey, @my-org/my-team"** - Any arbitrary
  string message to append to the drift GitHub comment.
* **-github-issue-assignees="dcreey"** - The assignees to assign to for any created
  GitHub Issues.
* **-github-issue-labels="guardian-iam-drift"** - The labels to use on any created
  GitHub Issues.
* **-skip-github-issue** - Whether or not to create a GitHub Issue when a drift is
  detected. The default value is "false".

## Workflows plan-status-comment

Add Guardian plan comments to a pull request.

Usage: guardian workflows plan-status-comment [options] <pull_request_number>

### Prerequisites

* Write permission to the target GitHub repository `pull-requests`.

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-init-result="success"** - The Guardian init job result status.
* **-plan-result="failure"** - The Guardian plan job result status.
* **-pull-request-number="100"** - The GitHub pull request number to remove plan
  comments from. The default value is "0".

## Workflows remove-plan-comments

Remove previous Guardian plan comments from a pull request.

Usage: guardian workflows remove-plan-comments [options] <pull_request_number>

### Prerequisites

* Write permission to the target GitHub repository `pull-requests`.

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-pull-request-number="100"** - The GitHub pull request number to remove plan
  comments from. The default value is "0".

## Workflows validate-permissions

Validate a list of allowed permissions for the actor running the current GitHub workflow.

Usage: guardian workflows validate-permissions [options]

### Prerequisites

* Read permission to the target GitHub repository `content`.

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-allowed-permissions="admin, write"** - The list of allowed permissions to validate against.
        