# Copyright 2024 The Authors (see AUTHORS file)
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

package guardian.run.workflow_permissions

import rego.v1
import data.github as github
import input

# Configuration
default_allowed_commands := ["plan", "apply", "output"]
# Replace with team name. Requires GitHub Token Minter and `--include-teams`
# flag on `guardian policy fetch-data` command.
privileged_teams := []
privileged_roles := ["admin"]

# Preprocess inputs
command := split(input.command, " ")[0]

# Start of policy
default authorized := false

authorized if {
  command in default_allowed_commands
}

authorized if {
  github.actor.access_level in privileged_roles
}

authorized if {
  github_actor_privileged_teams := [team |
    some team in github.actor.teams
    team in privileged_teams
  ]
  count(github_actor_privileged_teams) > 0
}

deny contains {
    "msg": msg,
} if {
  not authorized
  msg := sprintf("command '%s' is restricted to users with the roles %s or members of teams %s", [command, privileged_roles, privileged_teams])
}
