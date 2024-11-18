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

deny contains {
    "msg": msg,
} if {
    # Configuration
    default_allowed_commands := ["plan", "apply", "output"]
    privileged_teams := [] # replace with team name
    privileged_roles = ["admin"]

    # Start of policy
    command := split(input.command, " ")[0]
    is_default_command := command in default_allowed_commands

    has_privileged_role := github.user_access_level in privileged_roles

    github_actor_teams := [ team_name |
      some team_name
      members := github.team_memberships[team_name]
      members[_] == github.actor.username
    ]

    github_actor_privileged_teams := [team |
      some team in github_actor_teams
      team in privileged_teams
    ]
    is_privileged_team_member := count(github_actor_privileged_teams) > 0

    not is_default_command
    not has_privileged_role
    not is_privileged_team_member
    msg := sprintf("command '%s' is restricted to users with the roles %s or members of teams %s", [command, privileged_roles, privileged_teams])
}
