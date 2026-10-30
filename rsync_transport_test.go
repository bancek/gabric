package gabric

import (
	"bytes"
	"strings"
	"testing"
)

func TestRsyncRshCommand(t *testing.T) {
	cmd := rsyncRshCommand()

	if !strings.HasSuffix(cmd, rsyncSidecarArg) {
		t.Errorf("rsyncRshCommand() = %q, want it to end with %q", cmd, rsyncSidecarArg)
	}

	if !strings.Contains(cmd, " ") {
		t.Errorf("rsyncRshCommand() = %q, want the executable and the sidecar argument separated by a space", cmd)
	}

	if strings.ContainsAny(rsyncSidecarArg, " \t'\"") {
		t.Errorf("rsyncSidecarArg = %q, must not contain whitespace or quotes (openrsync splits on whitespace only)", rsyncSidecarArg)
	}
}

func TestReadRsyncServerCommand(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
		err   bool
	}{
		{
			name:  "with user",
			parts: []string{"7", "-l", "deploy", "example.com", "rsync", "--server", ".", "/home/app"},
			want:  "rsync --server . /home/app",
		},
		{
			name:  "without user",
			parts: []string{"5", "example.com", "rsync", "--server", ".", "/home/app"},
			want:  "rsync --server . /home/app",
		},
		{
			name:  "sudo rsync path",
			parts: []string{"6", "-l", "deploy", "example.com", "sudo rsync", "--server", "/home/app"},
			want:  "sudo rsync --server /home/app",
		},
		{
			name:  "missing command",
			parts: []string{"3", "-l", "deploy", "example.com"},
			err:   true,
		},
		{
			name:  "missing host",
			parts: []string{"1", "-l"},
			err:   true,
		},
		{
			name:  "empty",
			parts: []string{},
			err:   true,
		},
		{
			name:  "count mismatch",
			parts: []string{"9", "a", "b"},
			err:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readRsyncServerCommand(nulArgs(tt.parts...))
			if tt.err {
				if err != nil {
					return
				}

				t.Fatalf("readRsyncServerCommand() = %q, want error", got)
			}

			if err != nil {
				t.Fatalf("readRsyncServerCommand() error: %v", err)
			}

			if got != tt.want {
				t.Errorf("readRsyncServerCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadRsyncServerCommandMalformed(t *testing.T) {
	if _, err := readRsyncServerCommand(bytes.NewReader([]byte("2\x00a\x00b"))); err == nil {
		t.Error("readRsyncServerCommand() without trailing NUL = nil, want error")
	}
}

// nulArgs encodes an argument list the way the rsh helper writes it: each
// argument followed by a NUL byte.
func nulArgs(parts ...string) *bytes.Reader {
	var b bytes.Buffer

	for _, part := range parts {
		b.WriteString(part)
		b.WriteByte(0)
	}

	return bytes.NewReader(b.Bytes())
}
