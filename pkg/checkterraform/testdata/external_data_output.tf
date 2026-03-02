# Valid TF File that contains an external data source.
terraform {
  required_providers {
  }
}

data "external" "test" {
  program = ["echo", "{\"hello\": \"world\"}"]
}

output "test" {
  value = data.external.test.result
}

