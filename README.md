# 🔱 Guardian 🔱

[![Guardian Apply](https://github.com/abcxyz/guardian/actions/workflows/test-apply.yml/badge.svg?event=push)](https://github.com/abcxyz/guardian/actions/workflows/test-apply.yml)

Guardian is a Terraform actuation and enforcement tool using GitHub actions.

**This is not an official Google product.**

## Installation

There are two options for installing Guardian:

- As a single-static binary from the [releases page](https://github.com/abcxyz/guardian/releases)

  Example:

  ```shell
  curl -sSL https://github.com/abcxyz/guardian/releases/download/v1.0.0/guardian_1.0.0_linux_amd64.tar.gz | tar -xzv
  ```

  > NOTE: The Guardian binary will need to be placed in your `$PATH`

- From source using Go

  ```shell
  go install github.com/abcxyz/guardian/cmd/guardian@latest
  ```

## Developer workflow

- Create a PR to propose terraform changes
- Guardian will run `terraform plan` for the configured working directories and will create a pull request comment with the plan diff for easy review

  - Guardian will store the plan file remotely in a Google Cloud Storage bucket, with a unique prefix per pull request, per terraform working directory:

    `gs://<BUCKET_NAME>/guardian-plans/<OWNER>/<REPO>/<PR_NUMBER>/<TERRAFORM_WORKING_DIRECTORY_PATH>/tfplan.binary`

    `gs://my-terraform-state/guardian-plans/owner/repo/20/terraform/production/tfplan.binary`

- Have your PR reviewed and approved by a CODEOWNER
- If you need to re-run your plan, push new changes or re-run the workflow to generate new plan files
  - Use `git commit --allow-empty -m "sync" && git push origin BRANCH` to push an empty commit to re-trigger
- When the PR is merged Guardian will automatically run `terraform apply` for the plan file created for each working directory and post the results as PR comments
- Regardless of success or failure of apply, Guardian will delete all plan files
  - If the apply fails:
    - Another PR should be submitted to fix the failed state for the environment
    - A repostiory admin can run the Guardian Admin workflow to run terraform commands to fix the state, e.g. via `terraform apply`

### Guardian Admin

The `Guardian Admin` workflow can be used to run commands manually as the service account for Guardian. This process can only be done by someone with Admin permissions (may require a breakglass) and is helpful in fixing error scenarios with Terraform.

- Navigate to the Actions tab and select `Guardian Admin`
- Click the `Run workflow` drop down in the top right area
- Fill out the inputs
  - BRANCH: Only works from the default branch e.g. `main`
  - COMMAND: The terraform command to run e.g. `apply -input=false -auto-approve`

## Security

Guardian recommends the use of `pull_request_target` for the Guardian Plan action. This is for two reasons:

1. `pull_request_target` is run in the context head commit on the default branch (usually `main`), this ensures that only the approved and merged workflow is run for any given pull request.
2. This enables the ability to restrict access to the Google Cloud Workload Identity Federation provider using the `@refs/heads/main` for the workflows, additionally ensuring only approved and merged workflows can impersonate the highly privileged service account. This prevents someone from copying the workflows with the `pull_request` trigger and using the service account however they want.

Because `pull_request_target` runs in the context of the head commit on the repository default branch, we need to checkout the pull request branch for Guardian to run `terraform plan` on the proposed changes. This has been [documented as a security issue](https://securitylab.github.com/research/github-actions-preventing-pwn-requests/).

To make this process secure, Guardian should only be run in `internal` or `private` repositories and should disable the use of forks. This makes the `pull_request_target` operate with the same default behavior as `pull_request` but still ensures only the approved and merged workflow is being executed for the pull request.

## Repository Setup

### General

- Ensure repository visibility is set to `internal` or `private`
- Ensure that `Allow forking` is disabled, Guardian recommends the use of `pull_request_target`, see [above](#security)

### Branch protection

Branch protection is required to enable a safe and secure process. Ensuring these values are set properly prevents multiple pull requests from stepping on each other. By enabling `Require branches to be up to date before merging`, pull requests merged at the same time will cause one to fail and be forced to pull the changes from the default branch. This will kick of the planning process to ensure the latest changes are always merged.

- [x] Require a pull request before merging
- [x] Require approvals, minimum 1, suggested 2
- [x] Require review from Code Owners
- [x] Require status checks to pass before merging
  - [x] Require branches to be up to date before merging
  - After you create your first PR, make sure you require all your plan jobs as status checks (e.g. 'plan (directory-1)'), this is required to ensure the `Require branches to be up to date before merging` is enforced.
- [x] Require conversation resolution before merging
- [x] Require signed commits (optional)
- [x] Require linear history

### Directories

Guardian will only run Terraform commands for directories that have a Terraform [backend configuration](https://developer.hashicorp.com/terraform/language/settings/backends/configuration). This means if you add a new folder and you want Guardian to run the `terraform plan` and `terraform apply` commands from that directory as the root of the module, you should include a backend configuration within that directory.

### Creating Workflows

To use Guardian in your repository, copy over the GitHub workflow YAMLs from the `examples` directory into your `.github/workflows` folder.
