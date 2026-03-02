# Valid TF File that contains an external resource, azurerm, that isn't allowlisted.
terraform {
  required_providers {
    random = {
      source = "hashicorp/random"
    }
  }
}

provider "google" {
  project = "my-project"
}

resource "external_resource_group" "example" {
  name     = "example-resources"
  location = "West Europe"
}

resource "google_compute_instance" "default" {
  name = "test"
}

