# 🔱 Guardian 🔱

Guardian is a Terraform actuation and enforcement tool using GitHub actions.

**This is not an official Google product.**

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
  - PR NUMBER: Run this command for a specific PR, the PR branch will be checked out before running the command

## Repository Setup

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

### Creating Workflows

To use Guardian in your repository, copy over the GitHub workflow YAMLs from the `examples` directory into your `.github/workflows` folder.

To run for multiple working directories, you can create a matrix strategy or use multiple jobs. The example workflows provided make use of a matrix strategy. If you need to use separate service accounts to different credentials, prefer using multiple jobs and provide separate inputs.

```yaml
apply_prod:
  runs-on: "ubuntu-latest"
  steps:
    - name: "Checkout"
      uses: "actions/checkout@ac593985615ec2ede58e132d2e21d2b1cbd6127c" # ratchet:actions/checkout@v3

    - name: "Authenticate to Google Cloud"
      uses: "google-github-actions/auth@ef5d53e30bbcd8d0836f4288f5e50ff3e086997d" # ratchet:google-github-actions/auth@v1
      with:
        workload_identity_provider: "${{ vars.PROD_WIF_PROVIDER }}"
        service_account: "${{ vars.PROD_WIF_SERVICE_ACCOUNT }}"

    - name: "Guardian Apply"
      uses: "abcxyz/guardian/.github/actions/apply@SHA" # TODO: pin this to the latest sha in the guardian repo
      with:
        working_directory: "${{ matrix.working_directory }}"
        bucket_name: "${{ vars.GUARDIAN_BUCKET_NAME }}"
        terraform_version: "1.3.6"

apply_nonprod:
  runs-on: "ubuntu-latest"
  steps:
    - name: "Checkout"
      uses: "actions/checkout@ac593985615ec2ede58e132d2e21d2b1cbd6127c" # ratchet:actions/checkout@v3

    - name: "Authenticate to Google Cloud"
      uses: "google-github-actions/auth@ef5d53e30bbcd8d0836f4288f5e50ff3e086997d" # ratchet:google-github-actions/auth@v1
      with:
        workload_identity_provider: "${{ vars.NON_PROD_WIF_PROVIDER }}"
        service_account: "${{ vars.NON_PROD_WIF_SERVICE_ACCOUNT }}"

    - name: "Guardian Apply"
      uses: "abcxyz/guardian/.github/actions/apply@SHA" # TODO: pin this to the latest sha in the guardian repo
      with:
        working_directory: "${{ matrix.working_directory }}"
        bucket_name: "${{ vars.GUARDIAN_BUCKET_NAME }}"
        terraform_version: "1.3.6"
```
