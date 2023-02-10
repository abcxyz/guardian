locals {
  project_id               = "verbanicm-dev"
  name                     = "test-app-a"
  ci_service_account_email = "autotf-ci-sa@verbanicm-dev.iam.gserviceaccount.com"
}

resource "google_service_account" "run_service_account" {
  project      = local.project_id
  account_id   = "${local.name}-sa"
  display_name = "${local.name}-sa Cloud Run Service Account"
}

module "cloud_run" {
  source                = "git::https://github.com/abcxyz/terraform-modules.git//modules/cloud_run?ref=246cd2f48ca0e2f9c34492ceb16833f2279f64e7"
  project_id            = local.project_id
  name                  = local.name
  image                 = "gcr.io/cloudrun/hello:latest"
  service_account_email = google_service_account.run_service_account.email
}
