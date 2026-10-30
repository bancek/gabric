// Package gabrictest provides a high-level, in-process test harness for gabric
// tasks and reconcile nodes.
//
// The harness owns independent local and remote sandboxes. File operations
// mutate or read the sandbox filesystem and are recorded as high-level backend
// calls. Exec runs through an in-process shell, so built-ins and the commands
// registered with Handle behave like remote commands without using SSH. Exec
// applies ExecRequest.Env and records it in Command.Env.
//
// Ownership is simulated as labels via Side.SetOwner. Real root privileges are
// not simulated. Rsync, port forwarding and the interactive shell are not
// implemented by the harness.
//
// File operations and command handlers work on Windows when they use absolute
// virtual paths. POSIX path behavior inside Exec (cd, relative paths, globs and
// pwd) is not part of the Windows guarantee because the shell core relies on
// mvdan/sh's filepath handling.
package gabrictest
