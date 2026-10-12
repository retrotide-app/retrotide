package role

type (
	Role string
)

const (
	SuperAdmin Role = "super_admin"
	Admin      Role = "admin"
	Support    Role = "support"
)

func (r Role) IsSiteStaff() bool {
	return r.Rank() > 0
}

func (r Role) BypassesRestrictions() bool {
	return r == SuperAdmin
}

func (r Role) Rank() int {
	switch r {
	case SuperAdmin:
		return 3
	case Admin:
		return 2
	case Support:
		return 1
	}

	return 0
}
