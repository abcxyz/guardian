package guardian.admin.workflow_permissions

import rego.v1
import data.github as github

deny_all contains {
    "msg": msg,
} if {
    allowed_roles = ["admin"]

    authorized := github.user_access_level in allowed_roles

    not authorized
    msg := sprintf("guardian_admin is restricted to users with the roles: %s", [allowed_roles])
}
