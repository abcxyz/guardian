# 🔱 Guardian 🔱

Guardian is a Terraform actuation and enforcement tool using GitHub actions.

**This is not an official Google product.**

## Developer workflow

- Create a PR to propose terraform change
- Guardian run `terraform plan` for the configured working directories and will create a pull request comment with the plan diff for easy review

  - Guardian will store the plan file remotely in a Google Cloud Storage bucket, with a unique prefix per pull request, per working directory:

    `gs://<BUCKET_NAME>/guardian-plans/<OWNER>/<REPO>/<PR_NUMBER>/<TERRAFORM_DIR_PATH>/tfplan.binary`

- Have your PR reviewed and approved
- When the PR is merged Guardian will automatically run `terraform apply` for the plan file created for each working directory and post the results as PR comments
- Regardless of success or failure of apply, Guardian will delete all plan files
  - If the apply fails, another PR should be submitted to fix the failed state for the environment

**Exceptions**

- If you need to re-run your plan, push new changes or re-run the workflow to generate new plan files
  - Use `git commit --allow-empty -m "sync" && git push origin BRANCH` to push an empty commit to re-trigger
- If an apply has errors after merge, a subsequent PR should be created to rectify the issues
- If the issue can be resolved by issuing another apply manually, the `Guardian Admin` workflow can be run on the main branch
  - This process can only be done by someone with Admin permissions (may require a breakglass)
  - Navigate to the Actions tab and select `Guardian Admin`
  - Click the `Run workflow` drop down in the top right area
  - Fill out the inputs
    - BRANCH: Only works from the default branch e.g. `main`
    - COMMAND: The terraform command to run e.g. `apply -input=false -auto-approve`
    - WORKING DIRECTORY: The terraform directory to run the command in, e.g. `terraform/projects`, if blank it will run for the repository root directory
    - PR NUMBER: Run this command for a specific PR, the PR branch will be checked out before running the command

## Repository Setup

### Branch protection

Branch protection is required to enable a safe and secure process. Ensuring these values are set properly prevents multiple pull requests from stepping on each other. By enabling `Require branches to be up to date before merging`, pull requests merged at the same time will cause one to fail and be forced to pull the changes from the default branch. This will kick of the planning process to ensure the latest changes are always merged.

- [x] Require a pull request before merging
- [x] Require approvals, minimum 1, suggested 2
- [x] Require review from Code Owners
- [x] Require status checks to pass before merging
  - [x] Require branches to be up to date before merging
- [x] Require conversation resolution before merging
- [x] Require signed commits (optional)
- [x] Require linear history

### Workflows

To use Guardian in your repository, copy over the GitHub workflow YAMLs from the `examples` directory into your `.github/workflows` folder.

The following variables should be setup at repository level or replaced in the workflow files:

```yaml
wif_provider: "${{ vars.GUARDIAN_WIF_PROVIDER}}"
wif_service_account: "${{ vars.GUARDIAN_WIF_SERVICE_ACCOUNT }}"
guardian_bucket_name: "${{ vars.GUARDIAN_BUCKET_NAME }}"
```

For more information see [Defining configuration variables for multiple workflows](https://docs.github.com/en/actions/learn-github-actions/variables#defining-configuration-variables-for-multiple-workflows)

To run for multiple working directories, you can create a matrix strategy or use multiple jobs. If you need to use separate service accounts to provide isolation between non-production and production, prefer using multiple jobs and provide separate inputs.

```yaml
jobs:
  apply_org:
    uses: "abcxyz/guardian/.github/workflows/apply.yml@main" # TODO: pin this to the latest sha in the guardian repo
    with:
      working_directory: "terraform/non-prod"
      wif_provider: "${{ vars.NON_PROD_GUARDIAN_WIF_PROVIDER}}"
      wif_service_account: "${{ vars.NON_PROD_GUARDIAN_WIF_SERVICE_ACCOUNT }}"
      guardian_bucket_name: "${{ vars.GUARDIAN_BUCKET_NAME }}"
      guardian_terraform_version: "1.3.6"

  apply_projects:
    uses: "abcxyz/guardian/.github/workflows/apply.yml@main" # TODO: pin this to the latest sha in the guardian repo
    with:
      working_directory: "terraform/prod"
      wif_provider: "${{ vars.PROD_GUARDIAN_WIF_PROVIDER}}"
      wif_service_account: "${{ vars.PROD_GUARDIAN_WIF_SERVICE_ACCOUNT }}"
      guardian_bucket_name: "${{ vars.GUARDIAN_BUCKET_NAME }}"
      guardian_terraform_version: "1.3.6"
```
