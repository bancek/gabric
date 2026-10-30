// Package gabric is a Go-native task and deployment runner inspired by
// Fabric 1.x.
//
// A Gabric task body is an ordinary Go function:
//
//	func Deploy(ctx context.Context) {
//		gabric.Local(ctx).Exec("go", "build", "./cmd/app").Run()
//		gabric.Remote(ctx).Exec("systemctl", "restart", "app").Run()
//	}
//
// Tasks compose as normal Go function calls. There is no subprocess boundary
// between them.
//
// # Two layers
//
// Gabric is built from a small core of plain functions that operate on a
// context.Context. They can be used from any Go program with a main of your
// own:
//
//	gabric.Local(ctx).Exec("echo", "hello").Run()
//	gabric.Remote(ctx).Exec("echo", "hello").Run()
//	gabric.Remote(ctx).Rsync().Local("./app").Remote("/srv/app").Upload().Run()
//
// Execution state (the remote runner, sudo, local and remote exec environments,
// working directories, task arguments and the logger) lives in context.Context.
// The package-level Default holds the task registry and runner configuration,
// not per-run state, so tasks can be scoped and run concurrently.
//
// Transports are installed by integration packages. The SSH transport lives in
// gabricssh:
//
//	func main() {
//		gabricssh.InstallDefault()
//		gabric.Main()
//	}
//
// Without a transport, gabric.Main runs tasks that use Local(ctx); the default
// target remains Remote and needs an installed runner.
//
// On top of that core is an optional task registry and command-line runner:
//
//	var Hello = gabric.Task("hello", func(ctx context.Context) {
//		gabric.Local(ctx).Exec("echo", "Hello").Run()
//	})
//
//	func main() {
//		gabric.Main()
//	}
//
// The package-level Task, TaskErr, With, Use, ParseFlags, ParseTasks, Run and
// Main functions operate on Default. Use New for an isolated runner and task
// registry.
//
// # Reconcile
//
// Reconcile is a separate package that provides lazy, memoized nodes for
// idempotent desired state. The root package integrates it through
// WithReconcile, WithReport, WithReconcileReport, NodeTask and NodeTaskErr.
// Reconcile modules use TargetOps to choose local or remote operations. Main and
// Run install a remote target by default. See docs/reconcile/README.md.
//
// See docs/architecture.md for the execution model and
// docs/getting-started.md for a walkthrough.
package gabric
