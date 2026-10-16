# Guardian CLI

Supported commands:

| **Command**                 | **Subcommand**                                                  | **Required Github Permission**                                    | **Description**                                               |
|-----------------------------|-----------------------------------------------------------------|-------------------------------------------------------------------|---------------------------------------------------------------|
| [entrypoints](#entrypoints) |                                                                 |                                                                   | Determine the entrypoint directories to run Guardian commands |
| [apply](#apply)             |                                                                 | `contents: read`<br> `pull-requests: write`<br> `id-token: write` | Run Terraform apply for a directory                           |
| [plan](#plan)               |                                                                 | `contents: read`<br> `pull-requests: write`<br> `id-token: write` | Run Terraform plan for a directory                            |
| [run](#run)                 |                                                                 | none                                                              | Run a Terraform command for a directory                       |
| iam                         | [cleanup](#iam-cleanup)                                         | none                                                              | Remove any expired IAM in a GCP organization                  |
|                             | [detect-drift](#iam-detect-drift)                               | `issues: write`                                                   | Detect IAM drift in a GCP organization                        |
| drift                       | [statefiles](#drift-statefiles)                                 | `issues: write`<br> `contents: read`                              | Detect drift for terraform statefiles                         |
| workflows                   | [plan-status-comment](#workflows-plan-status-comment)           | `pull-requests: write`                                            | Add Guardian plan comment to a pull request                   |
|                             | [remove-guardian-comments](#workflows-remove-guardian-comments) | `contents: read`<br> `pull-requests: write`                       | Remove previous Guardian comments from a pull request         |
|                             | [report](#workflows-report)                                     | `actions: read`<br> `pull-requests: write`                        | Aggregate Guardian plan/apply status into a single PR comment |
| policy                      | fetch-data                                                      | See [Policy fetch-data command](#policy-fetch-data)               | Fetch data used for policy evaluation   |
|                             | enforce                                                         | See [Policy enforce command](#policy-fetch-data)                  | Enforce a set of Guardian policies      |

## Shared Options

These options influence any command run with Guardian.

### Platform Options
These options sets the code review platform that Guardian should interact with.

- **-platform="github"** - The code review platform for Guardian to integrate with. Allowed values are ["github", "local"].

### Common Options
These options apply to commands that interact with Terraform directories (`plan`, `apply`, `run`):

- **-dir="./terraform"** - The location of the terraform directory.

### Registry Proxy Options

These options apply to commands that interact with Terraform directories (`plan`, `apply`, `run`) and route provider requests through a proxy:

- **-registry-proxy="https://localhost:8080/"** - The base URL for a Terraform Provider Registry Proxy or Network Mirror. When set, Guardian routes all provider requests through this proxy, which transparently decides where to fetch each provider from. This option can also be specified with the GUARDIAN_REGISTRY_PROXY environment variable.

### GitHub Options

These options influence how Guardian interacts with GitHub:

* **-github-actions** - Is this running as a GitHub action. The default value is "false".
  This option can also be specified with the GITHUB_ACTIONS environment variable.
* **-github-owner="organization-name"** - The GitHub repository owner.
* **-github-repo="repository-name"** - The GitHub repository name.
* **-guardian-github-token="string"** - The GitHub access token for Guardian to make GitHub API calls.
  This is separate from GITHUB_TOKEN because Terraform uses GITHUB_TOKEN to
  authenticate to the GitHub APIs also. Splitting this up allows users to
  follow least privilege for the caller (e.g. Guardian vs Terraform). If
  not supplied this will default to GITHUB_TOKEN. This option can also be
  specified with the GUARDIAN_GITHUB_TOKEN environment variable.
* **-github-pull-request-number="100"** - The GitHub pull request number associated with this
  apply. Only one of pull-request-number and commit-sha can be given. The default value is "0".

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

Usage: guardian entrypoints [options]

### Prerequisites

* Required GitHub [permissions](#guardian-cli).

### Options

Also supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-dir** - The root directory to search for entrypoint directories. Defaults to the current working directory.
* **-dest-ref="ref-name"** - The destination GitHub ref name for finding file changes.
* **-detect-changes** - Detect file changes, including all local module dependencies,
  and run for all entrypoint directories. The default value is "false".
* **-fail-unresolvable-modules** - Whether or not to error if a module cannot be
  resolved. The default value is "false".
* **-format="json"** - The format to print the output directories. The supported
  formats are: [json text]. The default value is "text".
* **-max-depth="int"** - How far to traverse the filesystem beneath the target
  directory for entrypoints. The default value is "-1".
* **-source-ref="ref-name"** - The source GitHub ref name for finding file changes.
* **--skip-reporting** - If true, then skips reporting the entrypoints in a comment/note on the platform's change request. Defaults to false.


## Apply

Run Terraform apply for a directory.

Usage: guardian apply [options]

### Prerequisites

* Required GitHub [permissions](#guardian-cli).
* The environment where you run this command must have Terraform installed locally.
* The appropriate permissions to change all resources in your terraform configuration
  (e.g. write access to all GCP/AWS/GitHub resources in your terraform).
* The user must be authenticated to the appropriate provider (e.g. for GCP they must
  have run gcloud auth).

### Options

Also supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-dir** - The Terraform directory to run the apply command. Defaults to the current working directory.
* **-allow-lockfile-changes** - Allow modification of the Terraform lockfile. The default value is "false".
* **-storage="URL"** - The storage strategy for saving Guardian plan files. Defaults to current working directory of the local filesystem.
* **--skip-reporting** - If true, then skips reporting the status of the Apply in a comment/note on the platform's change request. Defaults to false.

  *Supported values:*
    - Local file storage - Format `file://my/absolute/path`
    - Google Cloud Storage Bucket - Format `gcs://my-guardian-state-bucket`

* **-lock-timeout="10m"** - The duration Terraform should wait to obtain a lock when
  running commands that modify state. The default value is "10m".
* **-allowed-provider-sources="hashicorp/google,integrations/github"** - The allowlist of
  Terraform provider sources. See [Provider Source Restrictions](#provider-source-restrictions).
  There is no default; if unset, no providers are allowed. A bare `"*"` allows
  all and must only be used in test environments.
* **-allowed-provisioner-sources="builtin/local-exec"** - The allowlist of
  Terraform provisioner sources. See [Provisioner Source Restrictions](#provisioner-source-restrictions).
  If unset, no provisioners are allowed; use `"*"` to allow all.
* **-allowed-backend-attributes="gcs/bucket,gcs/prefix"** - The allowlist of
  backend types, and the attributes each may set, that the root module's
  `backend` and `cloud` blocks may use. See
  [Backend Attribute Restrictions](#backend-attribute-restrictions). Defaults
  to the common `gcs`, `s3` and `azurerm` attributes. An empty value allows no
  backends; `TYPE/*` allows everything for one type and should only be used
  in test environments.

## Plan

Run Terraform plan for a directory.

Usage: guardian plan [options]

### Prerequisites

* Required GitHub [permissions](#guardian-cli).
* The environment where you run this command must have Terraform installed locally.
* The appropriate permissions to view all resources in your terraform configuration
  (e.g. read access to all GCP/AWS/GitHub resources in your terraform).
* The user must be authenticated to the appropriate provider (e.g. for GCP they must
  have run gcloud auth).

### Options

Also supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-dir** - The Terraform directory to run the plan command. Defaults to the current working directory.
* **-allow-lockfile-changes** - Allow modification of the Terraform lockfile. The default value is "false".
* **-storage="URL"** - The storage strategy for saving Guardian plan files. Defaults to current working directory of the local filesystem.
* **--skip-reporting** - If true, then skips reporting the status of the Plan in a comment/note on the platform's change request. Defaults to false.

  *Supported values:*
    - Local file storage - Format `file://my/absolute/path`
    - Google Cloud Storage Bucket - Format `gcs://my-guardian-state-bucket`

* **-lock-timeout="10m"** - The duration Terraform should wait to obtain a lock when
  running commands that modify state. The default value is "10m".
* **-output-dir="./output/plan"** - Write the plan binary and JSON file to a target local directory.
* **-allowed-provider-sources="hashicorp/google,integrations/github"** - The allowlist of
  Terraform provider sources. See [Provider Source Restrictions](#provider-source-restrictions).
  There is no default; if unset, no providers are allowed. A bare `"*"` allows
  all and must only be used in test environments.
* **-allowed-provisioner-sources="builtin/local-exec"** - The allowlist of
  Terraform provisioner sources. See [Provisioner Source Restrictions](#provisioner-source-restrictions).
  If unset, no provisioners are allowed; use `"*"` to allow all.
* **-allowed-backend-attributes="gcs/bucket,gcs/prefix"** - The allowlist of
  backend types, and the attributes each may set, that the root module's
  `backend` and `cloud` blocks may use. See
  [Backend Attribute Restrictions](#backend-attribute-restrictions). Defaults
  to the common `gcs`, `s3` and `azurerm` attributes. An empty value allows no
  backends; `TYPE/*` allows everything for one type and should only be used
  in test environments.

## Run

Run a Terraform command for a directory.

Usage: guardian run [options]

### Prerequisites

* The environment where you run this command must have Terraform installed locally.
* The appropriate permissions to view/change all resources in your terraform configuration
  (e.g. read access to all GCP/AWS/GitHub resources in your terraform).
* The user must be authenticated to the appropriate provider (e.g. for GCP they must
  have run gcloud auth).

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-dir** - The Terraform directory to run the command. Defaults to the current working directory.
* **-allow-lockfile-changes** - Allow modification of the Terraform lockfile. The default value is "false".
* **-lock-timeout="10m"** - The duration Terraform should wait to obtain a lock when
  running commands that modify state. The default value is "10m".
* **-allowed-terraform-commands="plan, apply, destroy"** - The list of allowed Terraform commands to be run. Defaults to all commands.
* **-allowed-provider-sources="hashicorp/google,integrations/github"** - The allowlist of
  Terraform provider sources. See [Provider Source Restrictions](#provider-source-restrictions).
  There is no default; if unset, no providers are allowed. A bare `"*"` allows
  all and must only be used in test environments.
* **-allowed-provisioner-sources="builtin/local-exec"** - The allowlist of
  Terraform provisioner sources. See [Provisioner Source Restrictions](#provisioner-source-restrictions).
  If unset, no provisioners are allowed; use `"*"` to allow all.
* **-allowed-backend-attributes="gcs/bucket,gcs/prefix"** - The allowlist of
  backend types, and the attributes each may set, that the root module's
  `backend` and `cloud` blocks may use. See
  [Backend Attribute Restrictions](#backend-attribute-restrictions). Defaults
  to the common `gcs`, `s3` and `azurerm` attributes. An empty value allows no
  backends; `TYPE/*` allows everything for one type and should only be used
  in test environments.

## Provider Source Restrictions

`guardian apply`, `guardian plan` and `guardian run` only allow Terraform to
install and execute providers whose source is in `-allowed-provider-sources`.
**The flag has no default; if it is unset, no providers are allowed**, and the
error names the sources the configuration uses so you can add the ones you
trust. Terraform's built-in `terraform` provider is always allowed.

Each entry is a provider source in the form `[HOSTNAME/]NAMESPACE/TYPE`, using
the same defaults as Terraform: `HOSTNAME` defaults to `registry.terraform.io`
and a bare `TYPE` implies the `hashicorp` namespace. Within an entry, `*`
matches exactly one segment, so `hashicorp/*` allows every provider in the
`hashicorp` namespace on the public registry and `*/*/*` allows every provider
on any host. A bare `*` allows every provider source. Only allowlist entries
are supported; there is no exclusion syntax.

> [!WARNING]
> List exact sources. Pull requests are untrusted, and a wildcard such as
> `hashicorp/*` or `*` also allows providers that run arbitrary programs during
> `terraform plan` (`hashicorp/external`, `hashicorp/helm`, `hashicorp/enos`)
> or read the runner's files and credentials and send them anywhere
> (`hashicorp/local`, `hashicorp/http`, `hashicorp/archive`). Any pull request
> could then run code with the workflow's cloud credentials. Use wildcards
> only in test environments.

## Provisioner Source Restrictions

> [!WARNING]
> Provisioners are discouraged in general; prefer native resources or a
> configuration management tool. Plugin-based provisioners run arbitrary
> binaries and should never be used outside of quick proofs of concept.

`guardian apply`, `guardian plan` and `guardian run` only allow provisioners
whose source is in `-allowed-provisioner-sources`. **If the flag is unset or
empty, no provisioners are allowed.**

Each entry is a provisioner source in the form `[ORIGIN/]TYPE`, where `ORIGIN`
is how Terraform loads the provisioner:

* `builtin`: compiled into Terraform (`file`, `local-exec` and `remote-exec`).
  A bare `TYPE` implies `builtin`, so `local-exec` is the same as
  `builtin/local-exec`.
* `plugin`: a `terraform-provisioner-<TYPE>` plugin binary installed in the
  trusted environment (for example `~/.terraform.d/plugins`). Guardian never
  loads provisioner plugins from the Terraform directory under review; see
  [Local Plugin Protection](#local-plugin-protection).

Within an entry, `*` matches exactly one segment, so `builtin/*` allows every
built-in provisioner. A bare `*` allows every provisioner. Only allowlist
entries are supported; there is no exclusion syntax.

## Backend Attribute Restrictions

`terraform init` initializes the `backend` (or HCP Terraform `cloud`) block
before it installs any provider, and backends are compiled into Terraform, so
the provider and provisioner allowlists do not cover them. Many backend
attributes can run commands or leak the runner's credentials, for example a
kubeconfig `exec` plugin (`kubernetes` `config_path`), an AWS
`credential_process` (`s3` `shared_config_files`), or a custom endpoint that
receives the runner's OAuth token (`gcs` `storage_custom_endpoint`).

`guardian apply`, `guardian plan` and `guardian run` therefore only allow
`backend` and `cloud` blocks in the root module to use backend types and
attributes listed in `-allowed-backend-attributes` (env
`GUARDIAN_ALLOWED_BACKEND_ATTRIBUTES`). Each entry is `TYPE/ATTRIBUTE`, where
`TYPE` is the backend type (`cloud` for HCP Terraform `cloud` blocks) and
attributes of nested blocks use dotted paths, such as `cloud/workspaces.name`
or `s3/assume_role.role_arn`. Allowing a block (e.g. `s3/assume_role`) allows
everything inside it. Entries are scoped to their type: `gcs/bucket` says
nothing about `s3`. A backend type with no entries is not allowed at all, even
with an empty block, because an empty block still initializes that backend
from the runner's ambient configuration. Allowed attributes must be static
literal values (no variables, function calls or templates). Backend blocks in
child modules are ignored by Terraform and are not checked.

The default allows the state location and locking attributes of the three
common backends, and no credential, endpoint, file path or identity attributes:

| Backend | Attributes |
| --- | --- |
| `gcs` | `bucket`, `prefix`, `kms_encryption_key` |
| `s3` | `bucket`, `key`, `region`, `dynamodb_table`, `encrypt`, `use_lockfile`, `kms_key_id` |
| `azurerm` | `resource_group_name`, `storage_account_name`, `container_name`, `key`, `use_azuread_auth`, `use_oidc` |

### Using other backends or attributes

Set the flag to the default plus what you need. For example, to also allow
HCP Terraform `cloud` blocks and the `s3` `assume_role` block:

```
-allowed-backend-attributes="gcs/bucket,gcs/prefix,gcs/kms_encryption_key,s3/bucket,s3/key,s3/region,s3/dynamodb_table,s3/encrypt,s3/use_lockfile,s3/kms_key_id,azurerm/resource_group_name,azurerm/storage_account_name,azurerm/container_name,azurerm/key,azurerm/use_azuread_auth,azurerm/use_oidc,cloud/organization,cloud/workspaces.name,s3/assume_role.role_arn"
```

Review each attribute you add. The configuration under review controls its
value, so ask whether an attacker who can set it to anything could make
`terraform init` run a program, read a file, or send a request with the
runner's credentials. Attributes that take a file path, a URL or hostname, a
credential, or a command are not safe to allow.

`TYPE/*` allows every attribute and nested block of one backend type, with any
value, and a bare `*` allows every backend. These skip the review above
entirely, so only use them if you accept that any pull request can configure
that backend however it likes, for example in a test environment with no
credentials worth protecting.

## Local Plugin Protection

Terraform looks for provider and provisioner plugins in locations that the
change under review controls. Provisioner plugins also override built-in
provisioners of the same name. So, regardless of the allowlists, before
`terraform init` Guardian:

1. Refuses to run if the Terraform directory contains `terraform.d`, which
   Terraform treats as a local provider mirror and provisioner plugin
   directory.
2. Refuses to run if the Terraform directory contains any file or directory
   named `terraform-provisioner-*`, which Terraform would load as a
   provisioner plugin.
3. Removes the entire `.terraform` directory (or file, or symlink, which is
   unlinked rather than followed), so `terraform init` re-creates it from the
   Terraform files. A pre-existing `.terraform` is not safe to reuse:
   `terraform init` reuses cached provider binaries that match the dependency
   lock file, loads plugins from any directory listed in
   `.terraform/plugin_path`, follows a symlinked `.terraform/modules` when
   installing modules (which could write a module over the provider cache or
   as `terraform.d`), and reuses the cached backend configuration in
   `.terraform/terraform.tfstate`, including `-backend-config` values that are
   not in the Terraform files.

Names are matched case-insensitively. Providers and modules are re-downloaded
on each run (set `TF_PLUGIN_CACHE_DIR` to avoid re-downloading providers), and
the selected workspace is reset to the default unless `TF_WORKSPACE` is set.
Local state (`terraform.tfstate` in the Terraform directory) is kept.

After `terraform init`, Guardian refuses to run if `terraform.d` or a
`terraform-provisioner-*` plugin exists in the Terraform directory, if
`.terraform/plugins` or `.terraform/plugin_path` exists, or if `.terraform`,
`.terraform/providers` or `.terraform/modules` is not a real directory (for
example, a symlink).

## Order of Checks

1. Before `terraform init`: the [Local Plugin Protection](#local-plugin-protection)
   steps, then disallowed providers and provisioners declared in the
   configuration are rejected, so disallowed providers are never downloaded.
2. After `terraform init`, before `terraform validate` (or any other command)
   executes a provider or provisioner: any installed provider whose source is
   not allowed is rejected, and the configuration is checked again, now
   including modules installed by `terraform init`. This covers providers and
   provisioners introduced by remote modules.

The configuration is analyzed one module at a time, as Terraform loads it: the
Terraform directory plus the module directories listed in
`.terraform/modules/modules.json`. A `.tf` or `.tf.json` file that cannot be
parsed is an error.

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

* **-disable-evaluate-condition** - Whether to evaluate the IAM Condition Expression
  and only delete those IAM with false evaluation. Defaults to false. Example: An IAM
  condition with expression `request.time < timestamp("2019-01-01T00:00:00Z")` will
  evaluate to false and the IAM will be deleted. The default value is "false".
* **-iam-query="policy:abcxyz-aod-expiry"** - The query to use to filter on IAM.
* **-max-conncurrent-requests="2"** - The maximum number of concurrent requests
  allowed at any time to GCP. The default value is "10".
* **-scope="123435456456"** - The scope to clean up IAM for - organizations/123456 will
  clean up all IAM matching your query in the organization and all folders and projects beneath it.

## IAM detect-drift

Detect IAM drift in a GCP organization.

Usage: guardian iam detect-drift [options]

### Prerequisites

The actor that runs this command must have:

* Required GitHub [permissions](#guardian-cli).
* A GCP project with Asset Inventory API enabled (`cloudasset.googleapis.com`)
  and Resource Manager enabled (`cloudresourcemanager.googleapis.com`).
  If running as yourself, be sure to set this as your default project via gcloud.
* Authentication to GCP via gcloud auth.
* Read-access to view all IAM for all projects, folders, and as well as organization-level
  IAM for the organization in question.

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
* **-skip-github-issue** - Whether to create a GitHub Issue when a drift is
  detected. The default value is "false".

### Using driftignore

With a `.driftignore` file you can define iam resources that you do not want to be
alerted for. This is especially useful for resources that are configured outside
terraform. Put this file at the root of your repository or indicate its location
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

Usage: guardian drift statefiles [options]

### Prerequisites

The actor that runs this command must have:

* Required GitHub [permissions](#guardian-cli).
* A GCP project with Asset Inventory API enabled (`cloudasset.googleapis.com`)
  and Resource Manager API enabled (`cloudresourcemanager.googleapis.com`).
  If running as yourself, be sure to set this as your default project via gcloud.
* Authentication to GCP via gcloud auth.
* Read-access to view all IAM for all projects, folders, and as well as organization-level
  IAM for the organization in question.

### Options

Also supports [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-dir** - The directory to use to determine Terraform entrypoints. Defaults to the current working directory.
* **-detect-gcs-buckets-from-terraform** - Whether or not to use the terraform
  backend configs to determine gcs buckets. The default value is "false".
* **-gcs-bucket-query="labels.terraform:*"** - The label to use to find GCS
  buckets with Terraform statefiles.
* **-github-repo-terraform-topics="terraform,guardian"** - Topics to use to
  identify GitHub repositories that contain terraform configurations.
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
* **-skip-github-issue** - Whether to create a GitHub Issue when a drift is
  detected. The default value is "false".

## Workflows plan-status-comment

Add Guardian plan comments to a pull request.

Usage: guardian workflows plan-status-comment [options]

### Prerequisites

* Required GitHub [permissions](#guardian-cli).

### Options

Also supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-init-result="success"** - The Guardian init job result status.
* **-plan-result="failure"** - The Guardian plan job result status.
* **--skip-reporting** - If true, then skips reporting the Plan status in a comment/note on the platform's change request. Defaults to false.

## Workflows remove-guardian-comments

Remove previous Guardian comments from a pull request.

Usage: guardian workflows remove-guardian-comments [options]

### Prerequisites

* Required GitHub [permissions](#guardian-cli).

### Options

Supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

## Workflows report

Aggregate and report Guardian plan/apply status in a single summary table on a pull request.

Usage: guardian workflows report [options]

### Prerequisites

* Required GitHub [permissions](#guardian-cli).

### Options

Also supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-type="plan"** - The type of the report, either "plan" or "apply".
* **-entrypoints='["terraform/project1"]'** - The list of directory entrypoints as a JSON array string, or a path to a file containing the JSON array.
* **-artifacts-dir="./artifacts"** - The local path where plan artifacts (`tfplan.json`) are downloaded. Required when `-type="plan"`.

## Policy fetch-data

Fetch data used for policy evaluation.

Usage: guardian policy fetch-data [options]

### Prerequisites

* Required GitHub [permissions](#guardian-cli).

* **members: read** is required for `--include-teams` flag.

### Options

Supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-output-dir="example/dir"** - Write the policy data JSON file to a target local directory.
* **--include-teams** - If true, includes team data in payload. Requires 'members: read' token permissions."


## Policy enforce

Enforce a set of Guardian policies

Usage: guardian policy enforce [options]

### Prerequisites

* Requires `pull-requests: "write"` to report any policy violations in a Pull Request comment.

The set of required permissions depends on the the type of policy being enforced.

*Policy types:*
- `missing_approvals` - Requires `pull-requests: "write"` to assign reviewers to the pull request.

  > The default workflow token cannot assign **teams** to pull requests. See
    [github-token-minter](https://github.com/abcxyz/github-token-minter) to support assigning team reviewers.

- `deny` - No additional permissions required.

### Options

Supports [Platform Options](#platform-options), [GitHub Options](#github-options) and [Retry Options](#retry-options).

* **-results-file="results.json"** - The path to a JSON file containing the OPA eval result.
* **-silent** - Skips any actions and only reports the violations found. The default value is "false".
* **--skip-reporting** - If true, then skips reporting the policy violations in a comment/note on the platform's change request. Defaults to false.
