package rbac

import "sync"

// Policy stores role-permission mappings for RBAC checks.
type Policy struct {
	mu       sync.RWMutex
	rolePerm map[string]map[string]struct{}
}

// NewPolicy creates a policy from role to permissions mapping.
func NewPolicy(bindings map[string][]string) *Policy {
	p := &Policy{rolePerm: make(map[string]map[string]struct{})}
	for role, perms := range bindings {
		p.SetRolePermissions(role, perms)
	}
	return p
}

// SetRolePermissions replaces permissions for one role.
func (p *Policy) SetRolePermissions(role string, permissions []string) {
	if role == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	set := make(map[string]struct{}, len(permissions))
	for _, perm := range permissions {
		if perm == "" {
			continue
		}
		set[perm] = struct{}{}
	}
	p.rolePerm[role] = set
}

// AddPermission appends one permission to a role.
func (p *Policy) AddPermission(role, permission string) {
	if role == "" || permission == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.rolePerm[role]; !ok {
		p.rolePerm[role] = map[string]struct{}{}
	}
	p.rolePerm[role][permission] = struct{}{}
}

// HasPermission returns true if any role has the required permission.
func (p *Policy) HasPermission(roles []string, permission string) bool {
	if permission == "" {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, role := range roles {
		if perms, ok := p.rolePerm[role]; ok {
			if _, exists := perms[permission]; exists {
				return true
			}
		}
	}
	return false
}

// HasAnyPermission returns true if roles satisfy any permission in the list.
func (p *Policy) HasAnyPermission(roles []string, permissions []string) bool {
	for _, perm := range permissions {
		if p.HasPermission(roles, perm) {
			return true
		}
	}
	return false
}

// HasAllPermissions returns true only if roles satisfy all permissions in the list.
func (p *Policy) HasAllPermissions(roles []string, permissions []string) bool {
	for _, perm := range permissions {
		if !p.HasPermission(roles, perm) {
			return false
		}
	}
	return true
}
