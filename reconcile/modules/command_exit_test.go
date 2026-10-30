package modules

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bancek/gabric"
)

func TestIsExitStatus(t *testing.T) {
	transportErr := errors.New("transport")

	tests := []struct {
		name     string
		err      error
		statuses []int
		want     bool
	}{
		{name: "nil", err: nil, statuses: []int{1}, want: false},
		{name: "match", err: &gabric.ExecExitError{Status: 1}, statuses: []int{1}, want: true},
		{name: "different status", err: &gabric.ExecExitError{Status: 2}, statuses: []int{1}, want: false},
		{name: "one of several", err: &gabric.ExecExitError{Status: 4}, statuses: []int{1, 4}, want: true},
		{name: "wrapped", err: fmt.Errorf("run: %w", &gabric.ExecExitError{Status: 3}), statuses: []int{3}, want: true},
		{name: "transport", err: transportErr, statuses: []int{1}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsExitStatus(tt.err, tt.statuses...); got != tt.want {
				t.Errorf("IsExitStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
