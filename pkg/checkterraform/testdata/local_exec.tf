# Valid TF File that contains valid providers, but a disallowed provisioner.
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

resource "terraform_data" "default" {

  provisioner "local-exec" {
    command = "echo hello world"
  }
}

