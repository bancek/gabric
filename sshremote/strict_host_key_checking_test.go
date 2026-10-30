package sshremote

import "testing"

func TestParseStrictHostKeyChecking(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want StrictHostKeyChecking
	}{
		{name: "empty", in: "", want: StrictHostKeyCheckingYes},
		{name: "yes", in: "yes", want: StrictHostKeyCheckingYes},
		{name: "yes uppercase", in: "YES", want: StrictHostKeyCheckingYes},
		{name: "ask", in: "ask", want: StrictHostKeyCheckingAsk},
		{name: "accept-new", in: "accept-new", want: StrictHostKeyCheckingAcceptNew},
		{name: "accept-new uppercase", in: "ACCEPT-NEW", want: StrictHostKeyCheckingAcceptNew},
		{name: "no", in: "no", want: StrictHostKeyCheckingNo},
		{name: "off", in: "off", want: StrictHostKeyCheckingNo},
		{name: "false", in: "false", want: StrictHostKeyCheckingNo},
		{name: "unknown", in: "maybe", want: StrictHostKeyCheckingYes},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStrictHostKeyChecking(tt.in); got != tt.want {
				t.Errorf("parseStrictHostKeyChecking(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestStrictHostKeyCheckingVerifyKnownHosts(t *testing.T) {
	tests := []struct {
		policy StrictHostKeyChecking
		want   bool
	}{
		{policy: "", want: true},
		{policy: StrictHostKeyCheckingYes, want: true},
		{policy: StrictHostKeyCheckingAsk, want: true},
		{policy: StrictHostKeyCheckingAcceptNew, want: true},
		{policy: StrictHostKeyCheckingNo, want: false},
	}

	for _, tt := range tests {
		if got := tt.policy.verifyKnownHosts(); got != tt.want {
			t.Errorf("%q.verifyKnownHosts() = %v, want %v", tt.policy, got, tt.want)
		}
	}
}
