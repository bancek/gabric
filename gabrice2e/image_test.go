package gabrice2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageAppend(t *testing.T) {
	image := ImageFromDockerfileBytes("test", []byte("FROM alpine\n"))
	appended := image.Append("RUN true")

	if got := string(image.dockerfile); got != "FROM alpine\n" {
		t.Errorf("original Dockerfile = %q, want %q", got, "FROM alpine\n")
	}

	if got, want := string(appended.dockerfile), "FROM alpine\nRUN true\n"; got != want {
		t.Errorf("appended Dockerfile = %q, want %q", got, want)
	}

	if appended.Name() != "test" {
		t.Errorf("Name() = %q, want %q", appended.Name(), "test")
	}
}

func TestImageFromDockerfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Dockerfile")
	content := []byte("FROM alpine\n")

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	image := ImageFromDockerfile(path)

	if got := string(image.dockerfile); got != string(content) {
		t.Errorf("Dockerfile = %q, want %q", got, content)
	}
	if image.contextDir != dir {
		t.Errorf("contextDir = %q, want %q", image.contextDir, dir)
	}
}

func TestImageFromNameAppendPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Append() did not panic")
		}
	}()

	ImageFromName("alpine:3.24").Append("RUN true")
}

func TestImageTag(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "alpine-rsync", want: "gabrice2e-alpine-rsync:"},
		{name: "Docker SSH/Test", want: "gabrice2e-docker-ssh-test:"},
		{name: "---", want: "gabrice2e-image:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := imageTag(tt.name, strings.Repeat("a", 64))
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("imageTag() = %q, want prefix %q", got, tt.want)
			}
		})
	}
}
