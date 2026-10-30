# Reconcile inputs

## Overview

Reconcile module builders take `input.Input[T]` values instead of plain Go
values. An input is read during a run, so a path, mode, owner, flag or list can
be static or come from a data node.

Inputs live in the standalone `github.com/bancek/gabric/input` package. The
package imports only the standard library. It does not import `reconcile` or any
module, so other builders and applications can use it too.

`*reconcile.Node[T]` satisfies `input.Input[T]`. A data node can therefore be
passed directly wherever a builder expects an input.

## The Input interface

```go
package input

type Input[T any] interface {
	Value(ctx context.Context) (T, error)
	Name() string
}
```

`Value` reads the value. `Name` returns a name used in resource names and error
messages. The interface does not distinguish static inputs from dynamic ones.

## Static values

`input.Static` wraps a constant. It does not create a run node, so it has no
memoization, dependency edge or invalidation.

```go
modules.File(input.Static("/etc/motd")).
	Content(input.Static([]byte("managed\n"))).
	Mode(modules.FileMode(0o644)).
	Build()
```

Application code can define a short alias:

```go
func S[T any](value T) input.Input[T] {
	return input.Static(value)
}
```

```go
modules.File(S("/etc/motd")).
	Content(S([]byte("managed\n"))).
	Mode(modules.FileMode(0o644)).
	Build()
```

`S` is an application alias. The library call is `input.Static`.

`input.Static` derives a name from the value:

| Value type     | Name                                   |
| -------------- | -------------------------------------- |
| `string`       | the string                             |
| `bool`         | `true` or `false`                      |
| `[]byte`       | `bytes`                                |
| `[]string`     | the joined values, capped at 64 chars  |
| `fmt.Stringer` | `String()`                             |
| anything else  | the Go type, for example `os.FileMode` |

## Reading inputs

| Helper           | Signature                                             | Behavior                                                     |
| ---------------- | ----------------------------------------------------- | ------------------------------------------------------------ |
| `Static`         | `Static[T](value T) Input[T]`                         | A constant input.                                            |
| `List`           | `List[T](values ...T) Input[[]T]`                     | A static list, shorthand for `Static(values)`.               |
| `True` / `False` | `Input[bool]`                                         | Static bool inputs for behavior flags.                       |
| `Value`          | `Value[T](ctx, in Input[T]) (T, error)`               | Reads `in`; a nil input yields the zero value.               |
| `ValueOr`        | `ValueOr[T](ctx, in Input[T], fallback T) (T, error)` | Reads `in`; a nil input yields `fallback`.                   |
| `Map`            | `Map[T, U](in Input[T], fn) Input[U]`                 | Applies `fn` to the input value.                             |
| `WithName`       | `WithName[T](in Input[T], name string) Input[T]`      | Wraps `in` and reports `name` from `Name()`.                 |
| `Flag`           | `Flag(flags ...Input[bool]) Input[bool]`              | Resolves the zero-or-one argument of a behavior flag method. |

`Value` and `ValueOr` read optional inputs. A nil input means unset, so
`Value` returns the zero value and `ValueOr` returns the fallback.

`Map` transforms an input without creating a run node. A node input is still
memoized once per run; the transform runs when the mapped input is read.

```go
authorizedKeys := input.Map(home, func(ctx context.Context, home string) (string, error) {
	return filepath.Join(home, ".ssh", "authorized_keys"), nil
})
```

`Flag` panics when it receives a nil input or more than one input. A builder
method should pass its variadic flag argument to `Flag` and store the result.

## Builder inputs

Every desired-state value on a module builder is an input. There are no separate
path, mode or owner node methods.

```go
func File(pathInput input.Input[string]) *FileBuilder
func (b *FileBuilder) Content(contentInput input.Input[[]byte]) *FileBuilder
func (b *FileBuilder) Mode(modeInput input.Input[os.FileMode]) *FileBuilder
func (b *FileBuilder) User(userInput input.Input[string]) *FileBuilder
func (b *FileBuilder) Group(groupInput input.Input[string]) *FileBuilder
func (b *FileBuilder) CreateOnly(flag ...input.Input[bool]) *FileBuilder
```

`Content` accepts a data node directly. `TemplateData` takes the template text
and its data as inputs and already returns a node, so it satisfies
`input.Input[[]byte]`:

```go
modules.File(S(composePath)).
	Content(modules.TemplateData(S(composePath), S(tmpl), data)).
	Build()
```

Use `modules.FileMode` for a file mode constant. It is a static input that avoids
an ambiguous untyped constant:

```go
modules.File(pathInput).Mode(modules.FileMode(0o644))
```

`Build()` is the only zero-argument terminal on a module resource builder.
Effect builders use `Effect()`.

## Lists and flags

List values are not variadic on builder methods. Pass one input for the whole
list:

```go
rsync.Sync(S("./srv")).
	Remote(S("/srv")).
	Delete().
	Exclude(input.List(".env", ".state")).
	Build()
```

Use `input.List` for a list literal and `input.Static(slice)` for an existing
slice:

```go
Exclude(input.List(".env", ".state"))

patterns := []string{".env", ".state"}
Exclude(input.Static(patterns))
```

Boolean behavior flags take zero or one bool input:

```go
modules.File(pathInput).CreateOnly()             // true
modules.File(pathInput).CreateOnly(input.False)  // false
modules.File(pathInput).CreateOnly(enabledNode)  // dynamic
```

Calling the method with no argument means true. Calling it with one input uses
that input. Not calling it leaves the module default. The same convention applies
to `Recursive`, `Force`, `Delete`, `ForceWrite`, `BuildImages`, `Latest`,
`System`, `CreateHome`, `Route`, `Enabled` and `Started`. For `service.Enabled`
the module default is true, because a service is enabled unless told otherwise.

## Resource names

A builder names its resource from its primary input:

```go
modules.File(input.WithName(pathInput, "motd"))
```

`input.Static("/etc/motd").Name()` is the path, so the default resource name is
`modules.file:/etc/motd`. Wrapping the path input with `WithName` makes it
`modules.file:motd`. `WithName` changes only the name; the value and the input's
identity are unchanged.

A node used as the primary input contributes its node name. Builders that have a
composed name, such as `ufw.Rule`, use their component input names.

## Relationship to reconcile

The reuse works in both directions.

- `input` is usable outside Reconcile. A builder can take an `input.Input[T]`
  without importing `reconcile` or opening a run.
- Reconcile is usable without `gabric` or modules. It imports only the standard
  library and `gabric/dryrun`, and it remains a small, reusable engine for nodes,
  topics, effects, and runs.

The engine helpers stay node-based: `reconcile.Map`, `ValueMatches`,
`ValueEquals` and `NodesEqual` take `*reconcile.Node[T]`. The `input` helpers are
for builder values and application code.

## Examples

A static file:

```go
modules.File(S("/etc/motd")).
	Content(S([]byte("managed\n"))).
	Mode(modules.FileMode(0o644)).
	Build()
```

A dynamic path from a data node:

```go
passwd := linux.UserPasswdData(S("deploy"))

authorizedKeys := input.Map(passwd, func(ctx context.Context, info linux.UserPasswd) (string, error) {
	return filepath.Join(info.Home, ".ssh", "authorized_keys"), nil
})

modules.File(authorizedKeys).
	Content(S([]byte(publicKey))).
	Mode(modules.FileMode(0o600)).
	Build()
```

`passwd` is a data node. `input.Map` derives the path from its value, and the
`File` builder reads the mapped input during the run.

Dynamic content:

```go
content := reconcile.Data("motd", func(ctx context.Context) ([]byte, error) {
	return renderMOTD(ctx)
}).Node()

modules.File(S("/etc/motd")).
	Content(content).
	Mode(modules.FileMode(0o644)).
	Build()
```

A boolean flag:

```go
modules.File(S("/etc/app/config.yaml")).
	Content(content).
	CreateOnly().
	Build()

modules.File(S("/etc/app/config.yaml")).
	Content(content).
	CreateOnly(input.False).
	Build()
```

A non-variadic list:

```go
rsync.Sync(S("./srv/www")).
	Remote(S("/srv/www")).
	Delete().
	Exclude(input.List(".env", ".state")).
	Build()
```

## Related docs

- [Reconcile architecture](architecture.md): the node engine and run model.
- [Reconcile API](api.md): node builders and engine helpers.
- [Modules](modules.md): module builders and their inputs.
- [Cookbook](cookbook.md): recipes using inputs with modules.
