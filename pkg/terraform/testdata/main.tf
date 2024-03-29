# Copyright 2023 The Authors (see AUTHORS file)
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

locals {
  project_id = "guardian-i-50"
  name       = "test-change-1"
}

data "github_repository" "infra" {
  full_name = "abcxyz/guardian"
}

resource "google_service_account" "default" {
  project = local.project_id

  account_id   = "${local.name}-${data.github_repository.infra.visibility}"
  display_name = "${local.name} Service Account"
  disabled     = true
}

output "service_account_name" {
  description = "The service account name."
  value       = google_service_account.default.name
}

module "test_no_backend" {
  source = "./no-backend"
}
