package rbac

import "testing"

func TestPolicyHasPermission(t *testing.T) {
	p := NewPolicy(map[string][]string{
		"admin": {"user.read", "user.write", "order.manage"},
		"user":  {"user.read"},
	})

	if !p.HasPermission([]string{"admin"}, "user.write") {
		t.Fatal("expected admin to have user.write")
	}
	if p.HasPermission([]string{"user"}, "user.write") {
		t.Fatal("expected user to not have user.write")
	}
	if !p.HasPermission([]string{"guest", "admin"}, "order.manage") {
		t.Fatal("expected any-role check to pass")
	}
}

func TestPolicyHasAnyAndAllPermissions(t *testing.T) {
	p := NewPolicy(map[string][]string{
		"ops": {"svc.read", "svc.deploy"},
	})

	if !p.HasAnyPermission([]string{"ops"}, []string{"svc.delete", "svc.deploy"}) {
		t.Fatal("expected any permission to pass")
	}
	if p.HasAnyPermission([]string{"ops"}, []string{"svc.delete", "svc.scale"}) {
		t.Fatal("expected any permission to fail")
	}

	if !p.HasAllPermissions([]string{"ops"}, []string{"svc.read", "svc.deploy"}) {
		t.Fatal("expected all permissions to pass")
	}
	if p.HasAllPermissions([]string{"ops"}, []string{"svc.read", "svc.delete"}) {
		t.Fatal("expected all permissions to fail")
	}
}

func TestPolicyMutation(t *testing.T) {
	p := NewPolicy(nil)
	p.AddPermission("auditor", "audit.read")
	if !p.HasPermission([]string{"auditor"}, "audit.read") {
		t.Fatal("expected added permission")
	}

	p.SetRolePermissions("auditor", []string{"audit.export"})
	if p.HasPermission([]string{"auditor"}, "audit.read") {
		t.Fatal("expected replaced permissions to remove old one")
	}
	if !p.HasPermission([]string{"auditor"}, "audit.export") {
		t.Fatal("expected new permission after replace")
	}
}
