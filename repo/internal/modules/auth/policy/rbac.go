package policy

// Role constants — kept in sync with the roles seed.
const (
	RoleAdmin              = "admin"
	RoleAuditor            = "auditor"
	RoleNurse              = "nurse"
	RoleFrontDesk          = "front_desk"
	RoleTherapist          = "therapist"
	RoleAide               = "aide"
	RoleTrainingCoordinator = "training_coordinator"
	RoleFinanceClerk       = "finance_clerk"
)

// HasRole reports whether the provided role list contains the target role.
func HasRole(roles []string, target string) bool {
	for _, r := range roles {
		if r == target {
			return true
		}
	}
	return false
}

// HasAnyRole reports whether roles contains at least one of the targets.
func HasAnyRole(roles []string, targets ...string) bool {
	for _, t := range targets {
		if HasRole(roles, t) {
			return true
		}
	}
	return false
}

// IsAdmin is a convenience check.
func IsAdmin(roles []string) bool { return HasRole(roles, RoleAdmin) }
