package shelltest

import (
	"context"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runChown(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	endOptions := false
	owner := ""
	paths := []string{}

	for _, arg := range cmd.Args {
		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case strings.HasPrefix(arg, "-"):
				fmt.Fprintf(hc.Stderr, "chown: invalid option: %s\n", arg)
				return interp.ExitStatus(1)
			}
		}

		if owner == "" {
			owner = arg
			continue
		}

		paths = append(paths, arg)
	}

	if owner == "" {
		fmt.Fprintln(hc.Stderr, "chown: missing operand")
		return interp.ExitStatus(1)
	}

	if err := validateOwner(owner); err != nil {
		fmt.Fprintf(hc.Stderr, "chown: %v\n", err)
		return interp.ExitStatus(1)
	}

	if len(paths) == 0 {
		fmt.Fprintln(hc.Stderr, "chown: missing operand")
		return interp.ExitStatus(1)
	}

	user, group, found := strings.Cut(owner, ":")
	if !found {
		group = ""
	}

	status := 0

	for _, p := range paths {
		currentUser, currentGroup, err := c.fs.ownerAt(hc.Dir, p, true)
		if err != nil {
			fmt.Fprintf(hc.Stderr, "chown: %v\n", err)
			status = 1
			continue
		}

		newUser, newGroup := user, group
		if newUser == "" {
			newUser = currentUser
		}
		if newGroup == "" {
			newGroup = currentGroup
		}

		if err := c.fs.setOwnerAt(hc.Dir, p, newUser, newGroup); err != nil {
			fmt.Fprintf(hc.Stderr, "chown: %v\n", err)
			status = 1
		}
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}

func validateOwner(owner string) error {
	user, group, found := strings.Cut(owner, ":")

	if found && strings.Contains(group, ":") {
		return fmt.Errorf("invalid owner: %s", owner)
	}

	if user == "" && group == "" {
		return fmt.Errorf("invalid owner: %s", owner)
	}

	return nil
}
