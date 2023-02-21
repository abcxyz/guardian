# 🔱 Guardian 🔱

Guardian is a Terraform actuation and enforcement tool using GitHub actions.

**This is not an official Google product.**

## Architecture

When submitting a PR, Guardian will create a lockfile using Google Cloud Storage for the repository based on its ID. A lock_id metadata param will be used to identify the pull request by ID. If another PR is submitted and a lock file exists, a comment will be issued and the PR will have to wait for the unlock.

```bash
gs://<BUCKET_NAME>/guardian-locks/<OWNER>/<REPO>/<REPO_ID>.tflock
```

Upon successful completion of a terraform plan, all plan files for each directory will be stored in Google Cloud Storage for later apply. Each successful plan run will create a unique plan file per directory, per PR git commit SHA.

```bash
gs://<BUCKET_NAME>/guardian-plans/<OWNER>/<REPO>/<PR_NUMBER>/<TERRAFORM_DIR_PATH>/<PR_HEAD_SHA>.tfplan
```

Upon completeion of a terraform apply, regardless of success or failure, the lock file will be deleted and all files under the `gs://<BUCKET_NAME>/guardian-plans/<OWNER>/<REPO>/<PR_NUMBER>/<TERRAFORM_DIR_PATH>/*` prefix will be deleted.

## Developer workflow

- Create a PR to propose changes
- Guardian will attempt to lock the terraform state by adding a remote lockfile for that given repository
  - If a lockfile exists for another PR, the workflow will fail and create a comment mentioning who has the state lock
  - If no lockfile exists or exists for current PR, Guardian will automatically run terraform plan and post the results as PR comments for the configured terraform directories
- Have your PR reviewed and approved
- When your PR is mergeable, merged the PR and Guardian will automatically run terraform apply for the proposed plan(s) and post the results as PR comments
- Regardless of success or failure of apply, Guardian will delete all plan and lockfiles for the next PR
  - If the apply fails, another PR should be submitted to fix the failed state for the environment

**Exceptions**

- If you need to re-run your plan, push new changes or re-run the workflow to generate new plan files
  - Use `git commit --allow-empty -m "sync" && git push origin BRANCH` to push an empty commit to re-trigger
- If you need to unlock your plan, add a comment with the message body `unlock` to trigger the unlock process
  - This process can only be done by someone with Maintainer or Admin permissions
  - This process can only be with a comment on the PR who holds the lock
- If an apply has errors after merge, a subsequent PR should be created to rectify the issues
- If the issue can be resolved by issuing another apply manually, the `Guardian Admin` workflow can be run on the main branch
  - This process can only be done by someone with Admin permissions (may require a breakglass)
  - Navigate to the Actions tab and select `Guardian Admin`
  - Click the `Run workflow` drop down in the top right area
  - Fill out the inputs
    - BRANCH: Only works from `main`
    - COMMAND: The terraform command to run e.g. `apply -input=false -auto-approve`
    - DIRECTORIES: The terraform directories to run in, e.g. `projects`, if blank it will run for all configured directories
    - PR NUMBER: Run this command for a specific PR

## Config

A `.guardian` file should exist in the root of the repository listing all terraform directories

Example

```shell
['bootstrap', 'org','resources']
```

## Repository Setup

### General

Pull Requests

- [ ] Allow merge commits
- [x] Allow squash merging
  - Default to pull request title and description
- [ ] Allow rebase merging

  After pull requests are merged...

- [x] Automatically delete head branches

### Branch protection

Branch protection is required to enable a safe and secure process.

- [x] Require a pull request before merging
- [x] Require approvals, minimum 1, suggested 2
- [x] Require review from Code Owners
- [x] Require status checks to pass before merging
  - [x] Require branches to be up to date before merging
- [x] Require conversation resolution before merging
- [x] Require signed commits (optional)
- [x] Require linear history
