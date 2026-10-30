package modules

import (
	"errors"

	"github.com/bancek/gabric"
)

// IsExitStatus reports whether err is a command exit error with one of statuses.
func IsExitStatus(err error, statuses ...int) bool {
	exitErr, ok := errors.AsType[*gabric.ExecExitError](err)
	if !ok {
		return false
	}

	for _, status := range statuses {
		if exitErr.Status == status {
			return true
		}
	}

	return false
}
