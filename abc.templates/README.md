# Guardian

## Prerequisites

- `abc` CLI
  ([installation guide](https://github.com/abcxyz/abc?tab=readme-ov-file#installation))

## Installation with abc CLI

The following command installs Guardian workflows in the current directory.

```shell
abc templates render \
  -input=terraform_version=<TERRAFORM_VERSION> \
  -input=guardian_version=<GUARDIAN_VERSION> \
  -input=guardian_wif_provider=<WIF_PROVIDER> \
  -input=guardian_service_account=<SERVICE_ACCOUNT> \
  -input=guardian_state_bucket=<GUARDIAN_STATE_BUCKET> \
  github.com/abcxyz/guardian.git//abc.templates/base-workflows
```

#### Optional inputs:

- `terraform_directory`: Defaults to the current directory.
