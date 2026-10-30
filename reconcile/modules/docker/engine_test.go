package docker

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/reconcile/modules/linux"
)

func TestDockerDistro(t *testing.T) {
	tests := []struct {
		name    string
		release linux.OSRelease
		want    string
		wantErr bool
	}{
		{"ubuntu", linux.OSRelease{ID: "ubuntu"}, "ubuntu", false},
		{"debian", linux.OSRelease{ID: "debian"}, "debian", false},
		{"ubuntu-like", linux.OSRelease{ID: "linuxmint", IDLike: []string{"ubuntu"}}, "ubuntu", false},
		{"debian-like", linux.OSRelease{ID: "custom", IDLike: []string{"debian"}}, "debian", false},
		{"unsupported", linux.OSRelease{ID: "fedora"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dockerDistro(tt.release)
			if (err != nil) != tt.wantErr {
				t.Fatalf("dockerDistro() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("dockerDistro() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDockerSourceLine(t *testing.T) {
	got := dockerSourceLine(engineRepo{Distro: "ubuntu", Codename: "resolute", Arch: "arm64"})
	want := "deb [arch=arm64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu resolute stable\n"
	if got != want {
		t.Errorf("dockerSourceLine() = %q, want %q", got, want)
	}
}

func TestEngineRepoData(t *testing.T) {
	tests := []struct {
		name      string
		osRelease string
		arch      string
		want      engineRepo
		wantErr   bool
	}{
		{
			name:      "ubuntu",
			osRelease: "ID=ubuntu\nVERSION_CODENAME=resolute\n",
			arch:      "amd64\n",
			want:      engineRepo{Distro: "ubuntu", Codename: "resolute", Arch: "amd64"},
		},
		{
			name:      "debian",
			osRelease: "ID=debian\nVERSION_CODENAME=bookworm\n",
			arch:      "arm64\n",
			want:      engineRepo{Distro: "debian", Codename: "bookworm", Arch: "arm64"},
		},
		{
			name:      "unsupported",
			osRelease: "ID=fedora\nVERSION_CODENAME=40\n",
			arch:      "amd64\n",
			wantErr:   true,
		},
		{
			name:      "missing codename",
			osRelease: "ID=ubuntu\n",
			arch:      "amd64\n",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().WriteFile("/etc/os-release", []byte(tt.osRelease), 0o644)
			gt.Remote().Handle("dpkg", func(gabrictest.Command) gabrictest.Result {
				return gabrictest.Result{Stdout: tt.arch}
			})

			got, err := gt.RunNodeErr(gt.Context(), engineRepoData())
			if (err != nil) != tt.wantErr {
				t.Fatalf("engineRepoData() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("engineRepoData() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEngineRepoDataArchError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile("/etc/os-release", []byte("ID=ubuntu\nVERSION_CODENAME=resolute\n"), 0o644)
	gt.Remote().Handle("dpkg", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), engineRepoData()); err == nil {
		t.Error("engineRepoData() error = nil, want error")
	}
}
