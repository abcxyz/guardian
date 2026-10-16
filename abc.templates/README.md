# Guardian

## Prerequisites

- `abc` The latest version of the abc CLI
  ([installation guide](https://github.com/abcxyz/abc?tab=readme-ov-file#installation))

## Install Terraform Actuation Workflows

The following command installs the default Guardian workflows for Terraform
actuation in the current directory.

```shell
abc templates render \
  -input=terraform_version=<TERRAFORM_VERSION> \
  -input=tagrep_version=<TAGREP_VERSION> \
  -input=guardian_wif_provider=<WIF_PROVIDER> \
  -input=guardian_service_account=<SERVICE_ACCOUNT> \
  -input=guardian_state_bucket=<GUARDIAN_STATE_BUCKET> \
  github.com/abcxyz/guardian/abc.templates/base-workflows@latest
```

> [!TIP]
> You can find the latest version of tagrep at
> https://github.com/abcxyz/tagrep/releases

#### Optional inputs:

- `terraform_directory`: Defaults to the current directory.
- `guardian_report_threshold`: Defaults to `5`. When a pull request changes at
  least this many Terraform entrypoints, Guardian replaces the per-directory
  comments with a single aggregated summary table. Raise it to keep
  per-directory comments on larger pull requests.
- `guardian_allowed_provider_sources` (required): The comma separated
  allowlist of Terraform provider sources that Guardian allows Terraform to
  install and execute, e.g. `hashicorp/google,hashicorp/google-beta`. List the
  exact providers you trust; `terraform providers` and `.terraform.lock.hcl`
  show the ones in use. There is no default. Wildcards (`hashicorp/*`, or a
  bare `*` for every provider) admit providers that run arbitrary programs on
  the runner or read its credentials, and must only be used in test
  environments.
- `guardian_allowed_provisioner_sources`: The comma separated allowlist of
  Terraform provisioner sources, e.g. `builtin/local-exec`. Defaults to blank,
  which allows no provisioners.

## Install Drift Detection Workflows

The following command installs Guardian workflows in the current directory.

```shell
abc templates render \
  -input=gcp_organization_id=<GCP_ORG_ID> \
  -input=guardian_wif_provider=<WIF_PROVIDER> \
  -input=guardian_service_account=<SERVICE_ACCOUNT> \
  github.com/abcxyz/guardian/abc.templates/drift-workflows@latest
```
