# Guardian

### Installation with abc CLI

The following command installs Guardian workflows in the current directory.

```shell
abc templates render \
  -input=guardian_wif_provider=<WIF_PROVIDER> \
  -input=guardian_service_account=<SERVICE_ACCOUNT> \
  -input=guardian_state_bucket=<GUARDIAN_STATE_BUCKET> \
  github.com/abcxyz/guardian.git//abc.templates/workflows
```

#### Optional inputs:
- `terraform_version`: Defaults to `1.5.6`.
- `terraform_directory`: Defaults to the current directory.
