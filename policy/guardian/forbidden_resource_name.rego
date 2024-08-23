package guardian.forbidden_resource_name

import rego.v1
import input as tfplan

missing_approval contains {
    "msg": msg,
    "assign_user_reviewers": assign_user_reviewers,
    "assign_team_reviewers": assign_team_reviewers
} if {

    some i
    tfplan.resource_changes[i].name == "null"

    allowed_teams := []
    allowed_users := ["verbanicm"]

    team_approvals := [team |
        some team in allowed_teams
        team in data.teams
    ]
    count(team_approvals) == 0

    user_approvals := [user |
        some user in allowed_users
        user in data.users
    ]
    count(user_approvals) == 0

    missing_team_approvals := [team |
        some team in allowed_teams
        not team in data.teams
    ]
    missing_user_approvals := [user |
        some user in allowed_users
        not user in data.users
    ]
    msg := sprintf("missing required approval for org policy changes, must be one of [users: %s, teams: %s]", [allowed_users, allowed_teams])
    assign_team_reviewers := missing_team_approvals
    assign_user_reviewers := missing_user_approvals
}
