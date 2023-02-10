terraform {
  required_version = ">= 1.0.0"

  required_providers {
    google = {
      version = ">= 4.45"
    }
  }

  backend "gcs" {
    bucket = "verbanicm-dev-terraform"
    prefix = "state/projects"
  }
}

module "app-a" {
  source = "./app-a"
}
