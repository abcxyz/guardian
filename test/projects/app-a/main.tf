locals {
  project_id = "verbanicm-dev"
  name       = "test-app-a"
}

resource "google_service_account" "test_service_account" {
  project      = local.project_id
  account_id   = "${local.name}-sa"
  display_name = "${local.name}-sa Service Account"
}
