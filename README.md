# 🔱 Guardian 🔱

Guardian is a Terraform collaboration and automation tool. Using the features of GitHub, Guardian enables
teams to enforce a propose, review, and acutate process for secure modification of resources.

> [!IMPORTANT]
> The usage of Guardian does not increase the security posture of your cloud resources if poorly configured.

**This is not an official Google product.**

## Support

For support related items, please open a [GitHub Issue](https://github.com/abcxyz/guardian/issues/new/choose).

## Releases

Guardian release announcements are done via GitHub and can be found [here](https://github.com/abcxyz/guardian/releases).
You can watch the Guardian repository in order to be notified each time a release is made.

## Guardian CLI

This is the underlying tool written in Golang that is enables all features. For more
details and to understand how to use the cli see [Guardian CLI](./cli.md).

## Guardian Features

> [!NOTE]
> While some features can be used independently, unless otherwise noted, all features have
> been designed to work with GitHub products (e.g. Pull Requests, Issues, Repositories).

* [Terraform Actuation](#terraform-actuation) via Plan, Apply, Run, and Admin cli
* [IAM Drift Detection](#iam-drift-detection) via IAM drift cli
* [Statefile Drift Detection](#statefile-drift-detection) via statefile drift cli
* [Policy Enforcement](#policy-enforcement)

### Terraform Actuation

* Show Terraform plans and applies, including outputs, in GitHub comments on Pull Requests.
* Automatically switch to a single aggregated summary table when the number of changed entrypoints reaches a configurable threshold (`GUARDIAN_REPORT_THRESHOLD`, defaults to `5`).
* Determines all Terraform entrypoints (e.g. where your Terraform backend configurations are)
  in your repository and plan/apply for each entrypoint.
* Automatically detect changes and only plan/apply entrypoints that have changed.
* Ensures Terraform plans are always up to date via native GitHub functionality.
* Designed to work with any of your terraform configurations.
* Compatible with any version of Terraform.
* Support for administrative functionality such as one-off `state rm` or `terraform apply` commands.

For more details on the user experience for an engineer developing terraform,
see [Developer Workflow](#developer-workflow). For more details on the admin
experience, see [Guardian Admin](#guardian-admin).

For more details on how to use the Guardian CLI for terraform actuation see the relevant
CLI doc:

* [Plan CLI Docs](./cli.md#plan)
* [Apply CLI Docs](./cli.md#apply)
* [Run CLI Docs](./cli.md#run)
* [Entrypoints CLI Docs](./cli.md#entrypoints)
* [Workflows Report CLI Docs](./cli.md#workflows-report)

You can get started with using Guardian for terraform actuation by
[Creating the Terraform Actuation GitHub Workflows](#creating-terraform-actuation-workflows).

### Merge Check

See [rulesets](#rulesets) for more info on enabling the merge check.

**The Problem:** When multiple Terraform PRs are active, a PR's
`terraform plan` can become **stale** if other relevant changes merge first.
Applying a stale plan risks errors or unexpected infrastructure state.

**Default Solution (Inefficient for repositories with many entrypoints):
Require Rebase Before Merge**

Requiring developers to rebase their PR onto the latest target branch before
merging is safe but can have downsides:

* **High Friction:** Frequent, often unnecessary, rebasing slows developers
  down.
* **Wasted Effort:** Rebasing and regenerating plans takes time, even if merged
  changes were unrelated.
* **Imprecise:** Treats all code changes equally, regardless of impact on the
  Terraform plan.

**Better Solution: The `guardian_merge_check` Workflow**

This GitHub Action workflow provides a smarter safety net:

1. **Simulates Merge:** Uses the `merge_group` trigger to create a temporary,
   prospective merge commit.
2. **Checks Precisely:** Runs `guardian entrypoints -detect-changes` on this
   simulated merge state.
3. **Identifies Real Conflicts:** It specifically checks if:
  * The Terraform entrypoints modified by the PR...
  * ...have *also* been modified on the target branch since the PR was
    created/updated.

**Why `guardian_merge_check` is Better:**

* **Reduces Unnecessary Rebases:** Only blocks PRs where *relevant*
  infrastructure changes have occurred concurrently. No need to rebase for
  unrelated updates.
* **Saves Time & CI Resources:** Avoids manual rebasing and plan regeneration
  when there's no actual risk to the plan's validity.
* **Provides Targeted Warnings:** If it fails, it signals a high probability of
  a stale plan, indicating a rebase is genuinely needed *before* merging.
* **Automated Safety:** Acts as an intelligent, automated pre-merge check
  specifically for IaC risks.

**In Short:** The `guardian_merge_check` replaces the need for a strict
"always rebase" policy with a precise, automated check that focuses only on
potential Terraform plan conflicts. This improves merge safety *and* developer
velocity by avoiding unnecessary work.

### IAM Drift Detection

* Compatible with Google Cloud Platform.
* Determines if there is any drift between your real IAM for Google Cloud Platform Org, Folders,
  Projects and your Terraform states.
* Generates a GitHub issue if a drift is detected.
* This issue will contain any identified click-ops changes as well as changes described
  in Terraform that are missing from your actual Google Cloud Platform IAM.

For more information on using iam drift detection see the
[IAM Drift CLI Docs](./cli.md#iam-detect-drift).

You can get started with using Guardian for drift detection by
[Creating the Drift Detection GitHub Workflows](#creating-drift-detection-workflows).

> [!TIP]
> Consider using in conjunction with [Statefile Drift Detection](#statefile-drift-detection)
> in order to locate outdated terraform state files that may incorrectly yield IAM drift.

### Statefile Drift Detection

* Compatible with Google Cloud Platform.
* Determines if there are any Terraform state files stored in remote state locations
  (GCS buckets) that are not represented in your Terraform repositories.
* This is especially useful when paired with IAM Drift Detection as you may encounter
  leftover state files that are no longer used that contain IAM resources. These IAM resources
  will falsely indicate a drift.
* Generates a GitHub issue if a drift is detected.
* This issue will contain any identified state files that are
  1. Described in your GitHub repositories that are missing in your remote state locations.
  2. In remote state locations but missing from your GitHub repositories.
  3. Empty (contains no resources and can be safely deleted) and not described in your GitHub repositories.

For more information on using statefile drift detection see the
[Statefile Drift CLI Docs](./cli.md#drift-statefiles).

You can get started with using Guardian for drift detection by
[Creating the Drift Detection GitHub Workflows](#creating-drift-detection-workflows).

### Policy Enforcement
`guardian policy` allows you to embed a set of policies within your code review
process.

#### Subcommands
`guardian policy fetch-data` - Fetches data from the corresponding code review
platform to provide additional context for evaluating policies. Requires
`contents: "read"` permission.

The result is written to a local file, `guardian_policy_context.json`.

  * Use `--include-teams` flag to return teams data in the payload. Requires
    `members: "read"` permission; Not available in default workflow token
    permissions. See [github-token-minter](https://github.com/abcxyz/github-token-minter).

    ```
    // Example
    {
      "github": {
        "pull_request_approvers": {
          "users": ["example-username"],
          "teams": ["example-team-name"]
        },
        "actor": {
          "username": "actor-name",
          "access_level": "admin",
          "teams": ["parent-team-of-actor"]
        }
      }
    }
    ```

`guardian policy enforce` - Accepts a file of OPA evaluation results, and
enforces the policies according to expected [enforcement rules](#supported-enforcement-rules).

#### Supported Enforcement Rules

* `deny` - Blocks the changes with a detailed error message.

  Policy results must be in the following format:
  ```
    {
      "name_of_policy": {
        "deny": [
          {
            "msg": "Sample deny message."
          }
        ]
      }
    }
  ```

* `missing_approvals` - Assigns principals to the change request and fails the
  status check until the required approvals are met.

  * Requires `pull-requests: "write"` permissions for GitHub workflows. Note:
    the default workflow token cannot assign teams to pull requests. See
    [github-token-minter](https://github.com/abcxyz/github-token-minter).

  Policy results must be in the following format:
  ```
    {
      "name_of_policy": {
        "missing_approvals": [
          {
            "assign_team_reviewers": [...] # github team names,
            "assign_user_reviewers": [...] # github usernames,
            "msg": "missing approvals for changes related to..."
          }
        ],
      }
    }
  ```

#### Usage
You can add policy evaluation and enforcement to your Guardian Plan workflow
with the following steps:

  > [!IMPORTANT]
  > The `tfplan.json` file is only available within the same job as the
  `guardian plan` command.

```
// guardian-plan.yml

  # ...
  # Within the Guardian Plan Job
  # ...

  - name: 'Aggregate Policy Data'
    shell: 'bash'
    env:
      # used to call GitHub API's for data aggregation
      GUARDIAN_GITHUB_TOKEN: '<TOKEN>'
    run: |-
      guardian policy fetch-data

  - name: 'Setup OPA'
    uses: 'open-policy-agent/setup-opa@v2'
    with:
      version: 'latest'

  # Use the policy definitions from the main/approved branch
  - name: 'Checkout'
    uses: 'actions/checkout@v4'
    with:
      ref: '${{ github.event.pull_request.base.sha }}'
      path: 'guardian-policy/main'

  - name: 'Evaluate Policy'
    id: 'opa_eval'
    shell: 'bash'
    run: |-
      DECISION=$(opa eval --input "${DIRECTORY}/tfplan.json" \
        --format raw \
        --data ./guardian-policy/main/policy \
        --data ./guardian_policy_context.json \
        "data.guardian")
      echo "$DECISION" > policy_results.json

  - name: 'Enforce Policy'
    shell: 'bash'
    env:
      GITHUB_TOKEN: '<TOKEN>'
    run: |-
      guardian policy enforce \
        -dir=${DIRECTORY} \
        -results-file=policy_results.json
```

## Guardian Terraform Best Practices

* Design your Terraform to have many small Terraform entrypoints. This will result
  in small Terraform state files - large Terraform states are an anti-pattern as they
  take a long time to plan/apply, are difficult to refactor, and broken applies can result
  in blocking all future work.
* Limit use of remote state. Remote state can be especially attractive when using many
  smaller Terraform entrypoints as it permits you to share configuration across entrypoints.
  However, remote state adds a lot of complexity to determining how your state should be
  planned/applied (e.g. updating entrypoint state A which is used in entrypoint state B
  means two separate applies - which puts the burden on the user to figure out how to
  manage this operation).
* If you are going to rely on remote state, try to use state that is relatively static.
  For example, a good candidate for remote state would be if you have some initial
  resources that need to be setup once and rarely change (e.g. Configuring a Google Cloud Platform Org).

## Developer workflow

- Create a PR to propose Terraform changes
- Guardian will run `terraform plan` for the configured working directories and
  will create a pull request comment with the plan diff for easy review

  - Guardian will store the plan file remotely in a Google Cloud Storage bucket,
    with a unique prefix per pull request, per Terraform working directory:

    `gs://<BUCKET_NAME>/guardian-plans/<OWNER>/<REPO>/<PR_NUMBER>/<TERRAFORM_WORKING_DIRECTORY_PATH>/tfplan.binary`

    `gs://my-terraform-state/guardian-plans/owner/repo/20/terraform/production/tfplan.binary`

- Have your PR reviewed and approved by a `CODEOWNER`
- If you need to re-run your plan, push new changes or re-run the workflow to
  generate new plan files
  - Use `git commit --allow-empty -m "sync" && git push origin BRANCH` to push
    an empty commit to re-trigger
- When the PR is merged Guardian will automatically run `terraform apply` for
  the plan file created for each working directory and post the results as PR
  comments
- Regardless of success or failure of apply, Guardian will delete all plan files
  - If the apply fails:
    - Another PR should be submitted to fix the failed state for the environment
    - A repository admin can run the Guardian Admin workflow to run Terraform
      commands to fix the state, e.g. via `terraform apply`

### Guardian Admin

The `Guardian Admin` workflow can be used to run commands manually as the
service account for Guardian. This process can only be done by someone with
Admin permissions (may require a breakglass) and is helpful in fixing error
scenarios with Terraform.

- Navigate to the Actions tab and select `Guardian Admin`
- Click the `Run workflow` drop down in the top right area
- Fill out the inputs
  - BRANCH: Only works from the default branch e.g. `main`
  - COMMAND: The Terraform command to run e.g.
    `apply -input=false -auto-approve`
  - ENTRYPOINT: A directory to find all child directories containing Terraform
    configurations. If left blank, the Terraform command will run
    for all configured directories.

### Guardian Run

The `Guardian Run` workflow can be used to run a limited set of Terraform commands manually as the service account for Guardian. This allows any user with
write permissions for the repository to run this workflow and maintain their Terraform configurations.

- Navigate to the Actions tab and select `Guardian Run`
- Click the `Run workflow` drop down in the top right area
- Fill out the inputs
  - BRANCH: Only works from the default branch e.g. `main`
  - COMMAND: Choose from the list of options you want to run.
    The default command is `plan`.
  - ENTRYPOINT: A directory to find all child directories containing Terraform
    configurations. If left blank, the Terraform command will run for all
    configured directories.

## Security

Guardian recommends the use of `pull_request_target` for the Guardian Plan
action. This is for two reasons:

1. `pull_request_target` is run in the context head commit on the default branch
   (usually `main`), this ensures that only the approved and merged workflow is
   run for any given pull request.
2. This enables the ability to restrict access to the Google Cloud Workload
   Identity Federation provider using the `@refs/heads/main` for the workflows,
   and to bind a dedicated read-only service account to `guardian-plan.yml`
   while restricting the privileged apply service account to post-merge
   workflows on `main`. This prevents someone from copying the workflows with
   the `pull_request` trigger or using an unreviewed pull request to access
   privileged credentials. See
   [Deployment Recommendations](#deployment-recommendations).

Because `pull_request_target` runs in the context of the head commit on the
repository default branch, we need to checkout the pull request branch for
Guardian to run `terraform plan` on the proposed changes. This has been
[documented as a security issue](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/).

To make this process secure, Guardian should only be run in `internal` or
`private` repositories and should disable the use of forks. This makes the
`pull_request_target` operate with the same default behavior as `pull_request`
but still ensures only the approved and merged workflow is being executed for
the pull request.

Because the pull request's Terraform configuration is untrusted, Guardian
restricts which Terraform providers and provisioners it will allow Terraform to
execute. Set `-allowed-provider-sources` (and the
`guardian_allowed_provider_sources` template input, which both default to
`hashicorp/*`) to the exact provider sources your configuration needs.
Provisioners are discouraged and blocked unless listed in
`-allowed-provisioner-sources` (the `guardian_allowed_provisioner_sources`
template input). Guardian also refuses to load plugins from the Terraform
directory itself. All of this is checked before any provider or provisioner is
executed. See [Provider Source Restrictions](cli.md#provider-source-restrictions),
[Provisioner Source Restrictions](cli.md#provisioner-source-restrictions) and
[Local Plugin Protection](cli.md#local-plugin-protection).

### Migrating from v3 to v4

Guardian v4 replaces name-based provider and provisioner restrictions with
source-based allowlists. **Even if you did not use these flags in v3, v4 is
potentially a breaking change due to safe-by-default restrictions.**

- The `-allowed-providers`, `-disallowed-providers`, `-allowed-provisioners`
  and `-disallowed-provisioners` flags are removed. Passing them to v4 fails
  with `flag provided but not defined`.
- **Defaults are restrictive.** In v3, unset flags allowed every provider and
  provisioner. In v4, `-allowed-provider-sources` defaults to `hashicorp/*`,
  and `-allowed-provisioner-sources` defaults to allowing no provisioners. An
  empty value for either flag allows nothing. Use `*` to allow all explicitly.
- Use `-allowed-provider-sources` with fully qualified provider sources instead
  of local names. For example, `-allowed-providers=google,github` becomes
  `-allowed-provider-sources=hashicorp/google,integrations/github`. Names are
  not sources, so look up the namespace of each provider you use. The `provider`
  blocks in `.terraform.lock.hcl` list the exact sources currently in use, as
  does `terraform providers`.
- The `guardian_allowed_providers` template input is replaced by
  `guardian_allowed_provider_sources`, which defaults to `hashicorp/*`.
  Re-render or upgrade the templates and add any other provider sources you
  need.
- `hashicorp/external` was previously denied by default. It is now denied only
  if it is not in your allowlist, and the default `hashicorp/*` allows it.
  Replace the default with the exact sources you use to keep it blocked.
- Use `-allowed-provisioner-sources` instead of the provisioner flags. For
  example, `-allowed-provisioners=local-exec` becomes
  `-allowed-provisioner-sources=local-exec` (that is, `builtin/local-exec`).
  The new `guardian_allowed_provisioner_sources` template input defaults to
  blank, which allows no provisioners. Provisioners are discouraged, and
  plugin-based provisioners should never be used outside of quick proofs of
  concept.
- The `guardian_service_account` template input in `base-workflows` is replaced
  by `guardian_plan_service_account` (read-only) and
  `guardian_apply_service_account` (read-write) so that pull request planning
  does not run with apply privileges. See
  [Deployment Recommendations](#deployment-recommendations).
- Provisioners are now checked before `terraform validate`, per module as
  Terraform loads them. Only the Terraform directory and the module directories
  in `.terraform/modules/modules.json` are scanned, not every subdirectory, and
  a file that cannot be parsed is an error.
- Guardian refuses to run if `terraform.d` or a `terraform-provisioner-*` plugin
  exists in a Terraform directory, and removes the `.terraform/providers`
  cache and `.terraform/plugin_path` before `terraform init` so that providers
  are always re-installed from their sources and plugins are only loaded from
  the trusted environment.

## Deployment Recommendations

### Separate Read-Only Service Account for Plan

> [!IMPORTANT]
> Always use a dedicated, **read-only** service account for `guardian plan`
> (`guardian_plan_service_account`), separate from the privileged read-write
> service account used for `guardian apply` (`guardian_apply_service_account`).

`guardian plan` runs on unreviewed pull requests via `pull_request_target` and
executes `terraform plan` against the pull request's untrusted `.tf` files.
While Guardian enforces provider source allowlists, provisioner source
allowlists, backend attribute allowlists, and local plugin protections before
providers execute, **`terraform plan` still executes your allowed cloud
providers (such as `hashicorp/google`) using the workflow's cloud credentials**
to refresh existing resources and evaluate any `data` sources or `ephemeral`
resources written in the pull request. Using a read-only service account to
limit the blast radius is a defense-in-depth best practice.

If `guardian plan` runs with the same privileged service account as
`guardian apply`:

1. **Secret and data exfiltration via `data` sources:** Any user who can open a
   pull request can add allowed provider `data` sources (for example,
   `data "google_secret_manager_secret_version"`,
   `data "google_storage_bucket_object_content"`, or
   `data "google_sql_database_instance"`) and reference their attributes in
   planned resource diffs or error expressions so that `guardian plan` prints
   the values in a pull request comment before the change is reviewed.
2. **Privilege escalation via token minting:** If the plan identity has
   permissions such as `iam.serviceAccounts.getAccessToken` or
   `roles/iam.serviceAccountTokenCreator` on privileged service accounts, a pull
   request can use `data "google_service_account_access_token"` or provider
   `impersonate_service_account` configuration during `terraform plan` to mint
   live access tokens for those accounts.
3. **Blast radius of misconfiguration:** If a deployment misconfigures provider
   or backend allowlists, running `plan` with `apply` credentials immediately
   exposes write and admin permissions over production infrastructure to
   unreviewed pull requests.

Configure the two service accounts with least privilege:

* **Plan Service Account (`guardian_plan_service_account`)** — used by
  `guardian-plan.yml` (and `plan`/`output` runs in `guardian-run.yml`):
  * **Managed cloud resources:** Grant strictly **read-only** roles required to
    refresh state for the resources managed by Terraform (for example,
    `roles/iam.securityReviewer` or custom read-only roles containing only
    `*.get`, `*.list`, and `*.getIamPolicy` permissions). Avoid `roles/viewer`,
    which includes `storage.objects.get` and allows reading potentially
    sensitive Cloud Storage objects across the project.
  * **No secret payload or impersonation permissions:** Do **not** grant
    permissions that read sensitive secret payloads (such as
    `roles/secretmanager.secretAccessor` / `secretmanager.versions.access`),
    create service account keys (`iam.serviceAccountKeys.create`), or mint
    tokens or act as other service accounts
    (`roles/iam.serviceAccountTokenCreator`, `roles/iam.serviceAccountUser`,
    `iam.serviceAccounts.getAccessToken`, `iam.serviceAccounts.getOpenIdToken`,
    `iam.serviceAccounts.signBlob`, `iam.serviceAccounts.signJwt`,
    `iam.serviceAccounts.actAs`).
  * **Terraform state and Guardian plan buckets:** Grant
    `roles/storage.objectViewer` on the Terraform state bucket so
    `terraform plan` can read `.tfstate`. Do **not** grant unrestricted write
    access to the state bucket, which would allow a pull request to overwrite or
    delete `.tfstate` objects. Instead, grant `roles/storage.objectUser`
    restricted with a
    [Cloud Storage IAM Condition](https://cloud.google.com/storage/docs/access-control/iam-conditions)
    so the plan service account can only create/delete `.tflock` lock files and
    write Guardian plan binaries under `guardian-plans/` (or use a dedicated
    bucket for `guardian_state_bucket`):
    ```
    resource.name.endsWith(".tflock") ||
    resource.name.startsWith("projects/_/buckets/<state-bucket-name>/objects/guardian-plans/")
    ```
* **Apply Service Account (`guardian_apply_service_account`)** — used only by
  `guardian-apply.yml` (on `push` to `main` after review and merge),
  `guardian-admin.yml` (restricted to repository administrators on `main`), and
  `guardian-run.yml` when `command` is `apply` on `main`:
  * Grant the read-write permissions needed to create, modify, and delete the
    managed infrastructure, read and write `.tfstate` and `.tflock` objects in
    the Terraform state bucket, and read and delete `guardian-plans/` objects in
    `guardian_state_bucket`.

### Least-Privilege Allowlists and Supply-Chain Hardening

* **Explicit Provider Allowlist (`-allowed-provider-sources`):** List only the
  exact `[HOSTNAME/]NAMESPACE/TYPE` providers required by your configuration
  (for example, `hashicorp/google,hashicorp/google-beta`). Never allow
  `hashicorp/external`, `hashicorp/local`, `hashicorp/http`, or wildcards
  (`hashicorp/*`, `*`) outside of isolated test environments.
* **Disallow Provisioners (`-allowed-provisioner-sources`):** Leave
  `-allowed-provisioner-sources` empty so all provisioners remain blocked.
* **Restrict Backend Attributes (`-allowed-backend-attributes`):** Keep the
  default state-location and locking attribute allowlist (or narrow it to only
  the backend type you use, such as
  `gcs/bucket,gcs/prefix,gcs/kms_encryption_key`). Never allow backend
  attributes that take file paths, custom endpoints/URLs, credentials, or
  commands, and do not use `TYPE/*` or `*` in production.
* **Provider Registry Proxy (`-registry-proxy`):** Where possible, route
  provider downloads through a trusted Terraform Registry Proxy or Network
  Mirror (`-registry-proxy` / `GUARDIAN_REGISTRY_PROXY`) so Terraform only
  installs vetted provider binaries.
* **Lockfile Enforcement:** Keep `-allow-lockfile-changes=false` (the default)
  and `protect_lockfile: true` in `abcxyz/secure-setup-terraform` so pull
  requests cannot modify `.terraform.lock.hcl` during planning.
* **Repository and Runner Hardening:** Run Guardian only in `private` or
  `internal` repositories with forking disabled, use ephemeral GitHub Actions
  runners, and require `CODEOWNERS` approval on `.github/workflows/`, `policy/`,
  and `.terraform.lock.hcl`.

## Workload Identity Federation
Terraform service accounts have elevated privileges, following [WIF Best Practices](https://cloud.google.com/iam/docs/best-practices-for-using-workload-identity-federation)
is recommended when setting up guardian to use Workload Identity Federation. Below is an example attribute
configuration that can be used to set up guardian in a secure manner.

The following [attribute mappings](https://cloud.google.com/iam/docs/workload-identity-federation#mapping) map claims from the [GitHub Actions JWT](https://token.actions.githubusercontent.com/.well-known/openid-configuration)
to Google STS token attributes.
  * `google.subject=assertion.sub`
  * `attribute.actor=assertion.actor`
  * `attribute.aud=assertion.aud`
  * `attribute.ref=assertion.ref`
  * `attribute.repository_owner_id=assertion.repository_owner_id`
  * `attribute.repository_id=assertion.repository_id`
  * `attribute.repository_visibility=assertion.repository_visibility`
  * `attribute.workflow_ref=assertion.workflow_ref`

The following [attribute condition](https://cloud.google.com/iam/docs/workload-identity-federation#conditions) verifies that the request is coming from your GitHub organization
and repository as well as restricting access to only the guardian workflows that run on the main branch.
```
attribute.repository_owner_id == "<your-repository-owner-id>" &&
attribute.repository_id == "<your-repository-id>" &&
attribute.repository_visibility != "public" &&
attribute.ref == "refs/heads/main" &&
attribute.workflow_ref in [
  "<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-admin.yml@refs/heads/main",
  "<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-apply.yml@refs/heads/main",
  "<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-plan.yml@refs/heads/main",
  "<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-run.yml@refs/heads/main",
]
```

### Service Account IAM Bindings

Bind `roles/iam.workloadIdentityUser` on each service account using
`attribute.workflow_ref` (not `attribute.repository_id`) so that
`guardian-plan.yml` can only impersonate the read-only plan service account and
can **never** impersonate the privileged apply service account:

* **Plan Service Account (`guardian_plan_service_account`):** Grant
  `roles/iam.workloadIdentityUser` only to the `plan` and `run` workflows:
  ```
  principalSet://iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/<pool-id>/attribute.workflow_ref/<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-plan.yml@refs/heads/main
  principalSet://iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/<pool-id>/attribute.workflow_ref/<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-run.yml@refs/heads/main
  ```
* **Apply Service Account (`guardian_apply_service_account`):** Grant
  `roles/iam.workloadIdentityUser` only to the `apply`, `admin`, and `run`
  workflows (**never** `guardian-plan.yml`):
  ```
  principalSet://iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/<pool-id>/attribute.workflow_ref/<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-apply.yml@refs/heads/main
  principalSet://iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/<pool-id>/attribute.workflow_ref/<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-admin.yml@refs/heads/main
  principalSet://iam.googleapis.com/projects/<project-number>/locations/global/workloadIdentityPools/<pool-id>/attribute.workflow_ref/<your-repository-owner-name>/<your-repository-name>/.github/workflows/guardian-run.yml@refs/heads/main
  ```

You can find the `id` and `owner.id` of your repository by using GitHub's REST API.
```shell
$ OWNER_NAME="owner"
$ REPO_NAME="repo"
$ curl https://api.github.com/repos/$OWNER_NAME/$REPO_NAME | jq '. | {"id": .id, "owner": { "id": .owner.id }}'
{
  "id": 12345,
  "owner": {
    "id": 9876
  }
}
```


## Repository Setup

### General

- Ensure repository visibility is set to `internal` or `private`
- Ensure that `Allow forking` is disabled, Guardian recommends the use of
  `pull_request_target`, see [above](#security)

### Rulesets

Rulesets are required to enable a safe and secure process. Ensuring
these values are set properly prevents multiple pull requests from stepping on
each other. There are two strategies for preventing terraform plan and apply conflicts:

1. By enabling `Require branches to be up to date before merging`, pull
   requests merged at the same time will cause one to fail and be forced to pull
   the changes from the default branch. This will kick of the planning process to
   ensure the latest changes are always merged.
2. By enabling the [merge queue check](#merge-check) in the GitHub merge queue
   we enable developers to simultaneously work on different terraform
   entrypoints and only require rebasing if there are changes that impact the
   same entrypoints modified in your pull request.

> [!NOTE]
> The [merge queue check](#merge-check) is the recommended approach for
> repositories with many terraform entrypoints.

> [!IMPORTANT]
> You must choose and implement one of either of the above strategies. You
> cannot implement both.

**Regardless of your choice of strategy, you will need to setup a ruleset with
the following rules:**

- [x] Require a pull request before merging
  - [x] Required approvals: minimum 1, suggested 2
  - [x] Require review from Code Owners
  - [x] Require conversation resolution before merging
- [x] Require status checks to pass before merging
  - [x] [optional] Require branches to be up to date before merging. You should
    remove this rule if you use the [merge queue check](#merge-check).
    Otherwise it is required.
  - After you create your first PR, make sure you require the `plan_success` job
    status check, this is required to ensure the
    `Require branches to be up to date before merging` is enforced. It is also
    required when using the merge queue.
  - If you are using the [merge queue check](#merge-check) you will also need
    to require the `merge_check` job status check, this is required to
    correctly block merging if a PR needs to be rebased.
- [x] [optional] Require merge queue. This is required if you want to use the
  [Merge Check](#merge-check).
  - [x] Require all queue entries to pass the merge check. (Required if
    enabling the merge queue).
- [x] [optional] Require signed commits
- [x] Require linear history

### Using Private Repositories as Modules
If you want to use Terraform modules located in private GitHub repositories then you will need to
configure git with the necessary permissions in order for Guardian to access these modules.

```shell
USERNAME=user-defined-name
TOKEN=your-token-with-read-acccess
git config --global url."https://${USERNAME}:${TOKEN}@github.com".insteadOf "https://github.com"
```

The `USERNAME` variable has no functional purpose and can be an arbitrary string. It is required
to be present in the URL, but GitHub will identify the user based on the token. It is recommended to provide a descriptive username and/or comment here so that developers know where this token came from (e.g. guardian-pat-token).

We recommend using [github-token-minter](https://github.com/abcxyz/github-token-minter) to generate short-lived access tokens token on demand for this purpose in your GitHub Actions.

### Directories

Guardian will only run Terraform commands for directories that have a Terraform
[backend configuration](https://developer.hashicorp.com/terraform/language/settings/backends/configuration).
This means if you add a new folder and you want Guardian to run the
`terraform plan` and `terraform apply` commands from that directory as the root
of the module, you should include a backend configuration within that directory.

### Creating Terraform Actuation Workflows

To use Guardian in your repository, see the
[template installation instructions](abc.templates/README.md#install-terraform-actuation-workflows)
in the [`abc.templates`](abc.templates) folder.

### Creating Drift Detection Workflows

To use Guardian drift detection in your repository, see the
[template installation instructions](abc.templates/README.md#install-drift-detection-workflows)
in the [`abc.templates`](abc.templates) folder.

## Metrics
We collect non-identifiable usage metrics using
[abcxyz/abc-updater](https://github.com/abcxyz/abc-updater). You can opt out of
these metrics by setting the environment variable `GUARDIAN_NO_METRICS`
to "true" in your shell.

Currently, data is collected on:
- Count of total invocations
- Count of each sub-command (apply, plan, policy enforce, policy fetch-data, ect)
- Count of invocations resulting in panic
- Runtime in ms of each invocation

Along with each metric, the following metadata is recorded:
- Application version
- Installation time with minute granularity

Metrics data is retained for 24 months.
