package gabrictest

import "github.com/bancek/gabric"

// FileKind restricts an Exists probe to a path kind.
type FileKind = gabric.FileKind

const (
	FileKindAny  = gabric.FileKindAny
	FileKindDir  = gabric.FileKindDir
	FileKindFile = gabric.FileKindFile
	FileKindLink = gabric.FileKindLink
)
