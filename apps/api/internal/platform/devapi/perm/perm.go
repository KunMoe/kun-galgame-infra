package perm

import "api/internal/platform/authz"

const (
	Manage       authz.Permission = "devapi.manage"
	PolicyManage authz.Permission = "devapi.policy_manage"
)

var NonDelegable = authz.NonDelegable{
	PolicyManage: true,
}

var adminPerms = []authz.Permission{Manage}

var renPerms = append(append([]authz.Permission{}, adminPerms...), PolicyManage)

var Bundles = authz.Bundles{
	"admin": adminPerms,
	"ren":   renPerms,
}

var Resolver = authz.NewHolder(Bundles)
