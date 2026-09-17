package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func setRestartOnClientDisable(t *testing.T, value bool) {
	t.Helper()
	if err := (&SettingService{}).SetRestartXrayOnClientDisable(value); err != nil {
		t.Fatalf("SetRestartXrayOnClientDisable(%v): %v", value, err)
	}
}

// RemoveUser drops the credential only, so a dropped client needs the restart the
// setting asks for; an edit re-adds the email and needs none.
func TestRestartToDropClients(t *testing.T) {
	cases := []struct {
		name        string
		diff        *xray.HotDiff
		setSetting  *bool
		wantRestart bool
	}{
		{
			// The default is what makes this reachable for a plain install.
			"disabled client, setting untouched",
			&xray.HotDiff{RemovedUsers: []xray.UserOp{{Tag: "in-443", Email: "a@x"}}},
			nil, true,
		},
		{
			"disabled client, setting on",
			&xray.HotDiff{RemovedUsers: []xray.UserOp{{Tag: "in-443", Email: "a@x"}}},
			new(true), true,
		},
		{
			"disabled client, setting off",
			&xray.HotDiff{RemovedUsers: []xray.UserOp{{Tag: "in-443", Email: "a@x"}}},
			new(false), false,
		},
		{
			"edited client",
			&xray.HotDiff{
				RemovedUsers: []xray.UserOp{{Tag: "in-443", Email: "a@x"}},
				AddedUsers:   []xray.UserOp{{Tag: "in-443", Email: "a@x"}},
			},
			new(true), false,
		},
		{
			"unrelated change",
			&xray.HotDiff{AddedInbounds: [][]byte{[]byte(`{}`)}},
			new(true), false,
		},
		{
			"nil diff",
			nil,
			new(true), false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupConflictDB(t)
			if tc.setSetting != nil {
				setRestartOnClientDisable(t, *tc.setSetting)
			}
			got := (&XrayService{}).restartToDropClients(tc.diff)
			if got != tc.wantRestart {
				t.Fatalf("restartToDropClients = %v, want %v", got, tc.wantRestart)
			}
		})
	}
}
