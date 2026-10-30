package gabrice2e

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bancek/gabric"
)

// Image describes a Docker image used by a test target.
type Image struct {
	name        string
	dockerfile  []byte
	contextDir  string
	existingRef string
}

// ImageFromDockerfile returns an image built from the Dockerfile at path. The
// Dockerfile directory is used as the build context.
func ImageFromDockerfile(path string) Image {
	content, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("gabrice2e: read Dockerfile %s: %v", path, err))
	}

	image := ImageFromDockerfileBytes(filepath.Base(path), content)
	image.contextDir = filepath.Dir(path)

	return image
}

// ImageFromDockerfileBytes returns an image built from content. The build
// context is empty, so the Dockerfile cannot COPY files.
func ImageFromDockerfileBytes(name string, content []byte) Image {
	if name == "" {
		panic("gabrice2e: empty image name")
	}
	if len(content) == 0 {
		panic("gabrice2e: empty Dockerfile")
	}

	return Image{
		name:       name,
		dockerfile: append([]byte(nil), content...),
	}
}

// ImageFromName returns an image that already exists.
func ImageFromName(ref string) Image {
	if ref == "" {
		panic("gabrice2e: empty image reference")
	}

	return Image{
		name:        ref,
		existingRef: ref,
	}
}

// Name returns the image name used in logs and tags.
func (i Image) Name() string {
	return i.name
}

// Append returns a new image with dockerfile appended to the image Dockerfile.
// It panics for an existing image reference.
func (i Image) Append(dockerfile string) Image {
	if i.existingRef != "" {
		panic("gabrice2e: cannot append to an existing image reference")
	}

	content := append([]byte(nil), i.dockerfile...)

	if len(content) > 0 && content[len(content)-1] != '\n' {
		content = append(content, '\n')
	}

	content = append(content, dockerfile...)

	if len(dockerfile) > 0 && dockerfile[len(dockerfile)-1] != '\n' {
		content = append(content, '\n')
	}

	i.dockerfile = content

	return i
}

type imageBuildState struct {
	once sync.Once
	ref  string
	err  error
}

var imageBuilds sync.Map

// build returns the Docker image reference, building the image when needed.
func (i Image) build(t testing.TB) string {
	t.Helper()

	if i.existingRef != "" {
		return i.existingRef
	}

	hash := i.hash()
	key := i.name + "\x00" + hash

	state := &imageBuildState{}
	if actual, loaded := imageBuilds.LoadOrStore(key, state); loaded {
		state = actual.(*imageBuildState)
	}

	state.once.Do(func() {
		state.ref, state.err = i.buildImage(t, imageTag(i.name, hash))
	})

	if state.err != nil {
		t.Fatalf("gabrice2e: build image %s: %v", i.name, state.err)
	}

	return state.ref
}

func (i Image) buildImage(t testing.TB, tag string) (string, error) {
	t.Helper()

	if _, err := runDocker(t.Context(), "image", "inspect", tag); err == nil {
		return tag, nil
	}

	dir := t.TempDir()
	dockerfilePath := filepath.Join(dir, "Dockerfile")

	if err := os.WriteFile(dockerfilePath, i.dockerfile, 0o644); err != nil {
		return "", fmt.Errorf("write Dockerfile: %w", err)
	}

	contextDir := i.contextDir
	if contextDir == "" {
		contextDir = dir
	}

	e := gabric.Local(t.Context()).Exec(
		"docker", "build",
		"-f", dockerfilePath,
		"-t", tag,
		contextDir,
	).Capture()

	if err := e.RunErr(); err != nil {
		return "", fmt.Errorf("%w\nstdout:\n%s\nstderr:\n%s", err, e.StdoutStr(), e.StderrStr())
	}

	return tag, nil
}

func (i Image) hash() string {
	sum := sha256.New()
	sum.Write(i.dockerfile)
	sum.Write([]byte{0})
	sum.Write([]byte(i.contextDir))

	return hex.EncodeToString(sum.Sum(nil))
}

func imageTag(name, hash string) string {
	safe := strings.ToLower(name)
	safe = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '.', r == '_':
			return r
		default:
			return '-'
		}
	}, safe)
	safe = strings.Trim(safe, "-._")
	if safe == "" {
		safe = "image"
	}

	return "gabrice2e-" + safe + ":" + hash[:12]
}
