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

## Install Drift Detection Workflows

The following command installs Guardian workflows in the current directory.

```shell
abc templates render \
  -input=gcp_organization_id=<GCP_ORG_ID> \
  -input=guardian_wif_provider=<WIF_PROVIDER> \
  -input=guardian_service_account=<SERVICE_ACCOUNT> \
  github.com/abcxyz/guardian/abc.templates/drift-workflows@latest
```
