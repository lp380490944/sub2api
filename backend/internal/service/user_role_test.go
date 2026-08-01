package service

import "testing"

func TestUserCanAccessAdminPanel(t *testing.T) {
	cases := []struct {
		name        string
		role        string
		wantPanel   bool
		wantIsAdmin bool
	}{
		{name: "admin", role: RoleAdmin, wantPanel: true, wantIsAdmin: true},
		{name: "readonly_admin", role: RoleReadonlyAdmin, wantPanel: true, wantIsAdmin: false},
		{name: "user", role: RoleUser, wantPanel: false, wantIsAdmin: false},
		{name: "unknown", role: "something-else", wantPanel: false, wantIsAdmin: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := &User{Role: tc.role}
			if got := u.CanAccessAdminPanel(); got != tc.wantPanel {
				t.Fatalf("CanAccessAdminPanel() = %v, want %v", got, tc.wantPanel)
			}
			// IsAdmin 语义严禁被放宽：readonly_admin 必须仍然不是 admin。
			if got := u.IsAdmin(); got != tc.wantIsAdmin {
				t.Fatalf("IsAdmin() = %v, want %v", got, tc.wantIsAdmin)
			}
		})
	}
}
