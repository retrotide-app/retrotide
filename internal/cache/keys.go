package cache

import (
	"strings"
	"time"
)

type (
	Namespace struct {
		Prefix string
		TTL    time.Duration
	}
)

var (
	Setting = Namespace{Prefix: "setting:"}

	UserRole        = Namespace{Prefix: "role:", TTL: time.Minute}
	RolePermissions = Namespace{Prefix: "authz:role-perms", TTL: time.Minute}

	OrgMembership  = Namespace{Prefix: "org:member:", TTL: time.Minute}
	TeamMembership = Namespace{Prefix: "team:member:", TTL: time.Minute}
	TeamHeader     = Namespace{Prefix: "team:header:", TTL: time.Minute}

	Subscription         = Namespace{Prefix: "billing:sub:", TTL: 5 * time.Minute}
	EntitlementOverrides = Namespace{Prefix: "billing:overrides:", TTL: 5 * time.Minute}

	OGMeta = Namespace{Prefix: "og:meta:", TTL: 5 * time.Minute}
)

func (n Namespace) Key(parts ...string) string {
	return n.Prefix + strings.Join(parts, ":")
}
