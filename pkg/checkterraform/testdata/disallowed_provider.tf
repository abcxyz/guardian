# Valid TF File that contains additional providers, such as azurerm, that aren't allowlisted.
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.0"
    }
    random = {
      source = "hashicorp/random"
    }
  }
}

provider "google" {
  project = "my-project"
}

resource "google_compute_instance" "default" {
  name = "test"
}

resource "external_data" "default" {
  name = "test"
}
