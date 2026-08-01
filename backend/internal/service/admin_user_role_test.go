package service

import "testing"

func TestNormalizeUserRoleAcceptsReadonlyAdmin(t *testing.T) {
	got, err := normalizeUserRole(RoleReadonlyAdmin, RoleUser)
	if err != nil {
		t.Fatalf("normalizeUserRole(readonly_admin) returned error: %v", err)
	}
	if got != RoleReadonlyAdmin {
		t.Fatalf("normalizeUserRole(readonly_admin) = %q, want %q", got, RoleReadonlyAdmin)
	}
}

func TestNormalizeUserRoleRejectsUnknown(t *testing.T) {
	if _, err := normalizeUserRole("superuser", RoleUser); err == nil {
		t.Fatal("normalizeUserRole(superuser) should reject unknown role, got nil error")
	}
}
