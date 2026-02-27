# Valid TF File that contains only allowed providers.
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

resource "google_compute_instance" "default" {
  name = "test"
}

resource "terraform_data" "default" {
  provisioner "safe-provisioner" {
    command = "echo hello world"
  }
}