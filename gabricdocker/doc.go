// Package gabricdocker is the Docker transport for gabric.
//
// Install registers the Docker flag and setup middleware:
//
//	func main() {
//		gabricdocker.InstallDefault()
//		gabric.Main()
//	}
//
// The transport runs commands with docker exec and exposes the current
// container through context helpers. It does not provide an interactive shell.
package gabricdocker
