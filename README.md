# 🔱 Guardian 🔱

[![Guardian Apply](https://github.com/abcxyz/guardian/actions/workflows/test-apply.yml/badge.svg?event=push)](https://github.com/abcxyz/guardian/actions/workflows/test-apply.yml)

Guardian is a Terraform actuation and enforcement tool using GitHub actions.

**This is not an official Google product.**

## Guardian CLI

This is the underlying tool written in Golang that is enables all features. For more
details and to understand how to use the cli see [Gaurdian CLI](./cli.md).

## Guardian Features

> [!NOTE]
> While some features can be used independently, unless otherwise noted, all features have
> been designed to work with GitHub products (e.g. Pull Requests, Issues, Repositories).

* [Terraform Actuation](#terraform-actuation) via Plan, Apply, Run, and Admin cli
* [IAM Drift Detection](#iam-drift-detection) via IAM drift cli
* [Statefile Drift Detection](#statefile-drift-detection) via statefile drift cli

### Terraform Actuation

* Show Terraform plans and applies, including outputs, in GitHub comments on Pull Requests.
* Determines all Terraform entrypoints (e.g. where your Terraform backend configurations are)
  in your repository and plan/apply for each entrypoint.
* Automatically detect changes and only plan/apply entrypoints that have changed.
* Ensures Terraform plans are always up to date via native GitHub functionality.
* Designed to work with any of your terraform configurations.
* Compatible with any version of Terraform.
* Support for administrative functionality such as one-off `state rm` or `terraform apply` commands.
* For more details on the user experience for an engineer developing terraform,
  see [Developer Workflow](#developer-workflow).
* For more details on the admin experience, see [Guardian Admin](#guardian-admin).

### IAM Drift Detection

* Compatible with Google Cloud Platform.
* Determines if there is any drift between your real IAM for Google Cloud Platform Org, Folders,
  Projects and your Terraform states.
* Generates a GitHub issue if a drift is detected.
* This issue will contain any identified click-ops changes as well as changes described
  in Terraform that are missing from your actual Google Cloud Platform IAM.
* Consider using in conjunction with [Statefile Drift Detection](#statefile-drift-detection)
  in order to locate outdated terraform state files that may incorrectly yield IAM drift.

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

## Security

Guardian recommends the use of `pull_request_target` for the Guardian Plan
action. This is for two reasons:

1. `pull_request_target` is run in the context head commit on the default branch
   (usually `main`), this ensures that only the approved and merged workflow is
   run for any given pull request.
2. This enables the ability to restrict access to the Google Cloud Workload
   Identity Federation provider using the `@refs/heads/main` for the workflows,
   additionally ensuring only approved and merged workflows can impersonate the
   highly privileged service account. This prevents someone from copying the
   workflows with the `pull_request` trigger and using the service account
   however they want.

Because `pull_request_target` runs in the context of the head commit on the
repository default branch, we need to checkout the pull request branch for
Guardian to run `terraform plan` on the proposed changes. This has been
[documented as a security issue](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/).

To make this process secure, Guardian should only be run in `internal` or
`private` repositories and should disable the use of forks. This makes the
`pull_request_target` operate with the same default behavior as `pull_request`
but still ensures only the approved and merged workflow is being executed for
the pull request.

## Repository Setup

### General

- Ensure repository visibility is set to `internal` or `private`
- Ensure that `Allow forking` is disabled, Guardian recommends the use of
  `pull_request_target`, see [above](#security)

### Branch protection

Branch protection is required to enable a safe and secure process. Ensuring
these values are set properly prevents multiple pull requests from stepping on
each other. By enabling `Require branches to be up to date before merging`, pull
requests merged at the same time will cause one to fail and be forced to pull
the changes from the default branch. This will kick of the planning process to
ensure the latest changes are always merged.

- [x] Require a pull request before merging
- [x] Require approvals, minimum 1, suggested 2
- [x] Require review from Code Owners
- [x] Require status checks to pass before merging
  - [x] Require branches to be up to date before merging
  - After you create your first PR, make sure you require the `plan_success` job
    status check, this is required to ensure the
    `Require branches to be up to date before merging` is enforced.
- [x] Require conversation resolution before merging
- [x] Require signed commits (optional)
- [x] Require linear history

### Directories

Guardian will only run Terraform commands for directories that have a Terraform
[backend configuration](https://developer.hashicorp.com/terraform/language/settings/backends/configuration).
This means if you add a new folder and you want Guardian to run the
`terraform plan` and `terraform apply` commands from that directory as the root
of the module, you should include a backend configuration within that directory.

### Creating Workflows

To use Guardian in your repository, see the
[template installation instructions](abc.templates/README.md) in the
[`abc.templates`](abc.templates) folder.
