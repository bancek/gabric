// Package gabricssh is the SSH transport for gabric.
//
// Install registers the SSH flags and setup middleware:
//
//	func main() {
//		gabricssh.InstallDefault()
//		gabric.Main()
//	}
//
// Library code can use WithSSH as middleware or set up the context explicitly
// with NewConfigContext, NewHostContext, NewManagerContext and
// NewRunnerContext. The low-level SSH client lives in the sshremote package.
package gabricssh
