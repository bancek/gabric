package github

import (
	"context"
	"net/url"
	"os"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/httpfile"
)

// DefaultBaseURL is the GitHub raw file host.
const DefaultBaseURL = "https://raw.githubusercontent.com"

// RawFileBuilder configures a pinned GitHub raw file download.
type RawFileBuilder struct {
	ownerInput   input.Input[string]
	repoInput    input.Input[string]
	pathInput    input.Input[string]
	refInput     input.Input[string]
	baseURLInput input.Input[string]
	destInput    input.Input[string]
	refFileInput input.Input[string]
	modeInput    input.Input[os.FileMode]
	userInput    input.Input[string]
	groupInput   input.Input[string]
}

// RawFile downloads owner/repo's path at a pinned ref to a target dest. It
// records the installed ref in a ref file next to dest and re-downloads only
// when the ref changes or dest is missing.
func RawFile(ownerInput, repoInput, pathInput, refInput input.Input[string]) *RawFileBuilder {
	return &RawFileBuilder{ownerInput: ownerInput, repoInput: repoInput, pathInput: pathInput, refInput: refInput}
}

// BaseURL overrides the raw file base URL. The default is DefaultBaseURL. This
// is useful for GitHub Enterprise and local e2e servers.
func (b *RawFileBuilder) BaseURL(baseURLInput input.Input[string]) *RawFileBuilder {
	b.baseURLInput = baseURLInput
	return b
}

// Dest sets the target destination path (required).
func (b *RawFileBuilder) Dest(pathInput input.Input[string]) *RawFileBuilder {
	b.destInput = pathInput
	return b
}

// Mode sets the downloaded file's mode.
func (b *RawFileBuilder) Mode(modeInput input.Input[os.FileMode]) *RawFileBuilder {
	b.modeInput = modeInput
	return b
}

// User sets the owning user.
func (b *RawFileBuilder) User(userInput input.Input[string]) *RawFileBuilder {
	b.userInput = userInput
	return b
}

// Group sets the owning group.
func (b *RawFileBuilder) Group(groupInput input.Input[string]) *RawFileBuilder {
	b.groupInput = groupInput
	return b
}

// RefFile sets the path of the ref marker. It defaults to Dest + ".ref".
func (b *RawFileBuilder) RefFile(pathInput input.Input[string]) *RawFileBuilder {
	b.refFileInput = pathInput
	return b
}

// RawFileGroup is the pinned download as nodes. Reading the cover (a group of
// download and writeRef) ensures dest is present at ref and the marker is up to
// date. It is not memoized; start a new Run to force a full re-check.
type RawFileGroup struct {
	*reconcile.Node[reconcile.Void]

	Ref      *reconcile.Node[string] // the installed ref (the ref file's content)
	Download *reconcile.Node[bool]   // downloads when the ref changed or dest is missing
	WriteRef *reconcile.Node[bool]   // writes the ref marker after a download
}

// Build hands off to the reconcile node builder. It panics if a required input
// is nil or if BaseURL cannot be joined.
func (b *RawFileBuilder) Build() *RawFileGroup {
	if b.ownerInput == nil || b.repoInput == nil || b.pathInput == nil {
		panic("modules: RawFile: owner, repo and path are required")
	}
	if b.refInput == nil {
		panic("modules: RawFile: nil ref")
	}
	if b.destInput == nil {
		panic("modules: RawFile: nil dest")
	}

	ownerInput, repoInput, pathInput, refInput := b.ownerInput, b.repoInput, b.pathInput, b.refInput
	destInput, modeInput, userInput, groupInput := b.destInput, b.modeInput, b.userInput, b.groupInput

	baseURLInput := b.baseURLInput
	if baseURLInput == nil {
		baseURLInput = input.Static(DefaultBaseURL)
	}

	rawURL := reconcile.Data("github.raw-file-url:"+destInput.Name(), func(ctx context.Context) (string, error) {
		baseURL, err := baseURLInput.Value(ctx)
		if err != nil {
			return "", err
		}
		owner, err := ownerInput.Value(ctx)
		if err != nil {
			return "", err
		}
		repo, err := repoInput.Value(ctx)
		if err != nil {
			return "", err
		}
		ref, err := refInput.Value(ctx)
		if err != nil {
			return "", err
		}
		path, err := pathInput.Value(ctx)
		if err != nil {
			return "", err
		}

		return url.JoinPath(baseURL, owner, repo, ref, path)
	}).Node()

	refFileInput := b.refFileInput
	if refFileInput == nil {
		refFileInput = input.Map(destInput, func(ctx context.Context, dest string) (string, error) {
			return dest + ".ref", nil
		})
	}

	refData := modules.FileContentData(refFileInput)
	destExists := modules.FileExistsData(destInput)

	downloadMaybe := httpfile.File(rawURL).
		Path(destInput).
		Mode(modeInput).
		User(userInput).
		Group(groupInput).
		ForceWrite().
		Build().
		Node()

	download := reconcile.Filter(downloadMaybe, func(ctx context.Context) (bool, error) {
		cur, err := refData.Value(ctx)
		if err != nil {
			return false, err
		}

		ref, err := refInput.Value(ctx)
		if err != nil {
			return false, err
		}

		if cur != ref {
			return true, nil
		}

		exists, err := destExists.Value(ctx)
		if err != nil {
			return false, err
		}

		return !exists, nil
	})

	writeRef := reconcile.IfChanged(
		modules.File(refFileInput).
			Content(input.Map(refInput, func(ctx context.Context, ref string) ([]byte, error) {
				return []byte(ref), nil
			})).
			Mode(modules.FileMode(0o644)).
			Build().
			Node(),
		download,
	)

	node := reconcile.Group("github.raw-file:"+destInput.Name()).
		Contains(download, writeRef).
		Node()

	return &RawFileGroup{
		Node:     node,
		Ref:      refData,
		Download: download,
		WriteRef: writeRef,
	}
}
