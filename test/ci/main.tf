terraform {
  required_version = ">= 1.0.0"

  required_providers {
    google = {
      version = ">= 4.45"
    }
  }

  backend "gcs" {
    bucket = "verbanicm-dev-terraform"
    prefix = "state/ci"
  }
}

locals {
  project_id = "verbanicm-dev"
  name       = "guardian"

  repos = {
    "abcxyz-guardian" : {
      repo_full_name : "abcxyz/guardian"
      repo_id : "599811719" # hard coding to avoid github provider and tokens
    }
  }
}

# Project Services
resource "google_project_service" "services" {
  for_each = toset([
    "artifactregistry.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "serviceusage.googleapis.com",
    "storage.googleapis.com",
    "sts.googleapis.com",
  ])

  project                    = local.project_id
  service                    = each.value
  disable_on_destroy         = false
  disable_dependent_services = false
}

# Storage Bucket for Actuation state
resource "google_storage_bucket" "default" {
  project                  = "verbanicm-dev"
  name                     = "verbanicm-dev-terraform"
  location                 = "US"
  public_access_prevention = "enforced"
}

resource "google_storage_bucket_iam_binding" "binding" {
  bucket  = google_storage_bucket.default.name
  role    = "roles/storage.objectAdmin"
  members = [google_service_account.ci_service_account.member]
}

# Workload Identity Federation
resource "google_iam_workload_identity_pool" "github_pool" {
  project                   = local.project_id
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub WIF Pool"
  description               = "Identity pool for CI environment"

  depends_on = [
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_iam_workload_identity_pool_provider" "github_provider" {
  for_each = local.repos

  project                            = local.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider-${each.key}"
  display_name                       = "GitHub WIF ${each.value.repo_full_name}"
  description                        = "GitHub OIDC identity provider for ${each.value.repo_full_name} CI environment"
  attribute_mapping = {
    "google.subject" : "assertion.sub"
    "attribute.actor" : "assertion.actor"
    "attribute.aud" : "assertion.aud"
    "attribute.event_name" : "assertion.event_name"
    "attribute.repository_owner_id" : "assertion.repository_owner_id"
    "attribute.repository" : "assertion.repository"
    "attribute.repository_id" : "assertion.repository_id"
    "attribute.workflow" : "assertion.workflow"
  }

  attribute_condition = "attribute.repository == \"${each.value.repo_full_name}\" && attribute.repository_id == \"${each.value.repo_id}\""

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  depends_on = [
    google_iam_workload_identity_pool.github_pool
  ]
}

resource "google_service_account" "ci_service_account" {
  project      = local.project_id
  account_id   = "${local.name}-ci-sa"
  display_name = "${local.name} CI Service Account"
}

resource "google_service_account_iam_member" "wif_github_iam" {
  service_account_id = google_service_account.ci_service_account.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/*"
}

resource "google_project_iam_member" "ci_sa_owner" {
  project = local.project_id
  role    = "roles/owner"
  member  = google_service_account.ci_service_account.member
}
