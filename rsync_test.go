package gabric

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestBuildRsyncArgs(t *testing.T) {
	tests := []struct {
		name string
		opts rsyncOptions
		want []string
	}{
		{
			name: "upload",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
			},
			want: []string{"rsync", "-pthrvz", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "download",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionDownload,
			},
			want: []string{"rsync", "-pthrvz", "--rsh=ssh", "deploy@example.com:/home/app", "./app"},
		},
		{
			name: "delete",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				delete:     true,
			},
			want: []string{"rsync", "--delete", "-pthrvz", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "delete with excludes",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				delete:     true,
				excludes:   []string{"**/.env", ".state/**"},
			},
			want: []string{"rsync", "--delete", "--exclude=**/.env", "--exclude=.state/**", "-pthrvz", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "excludes without delete",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				excludes:   []string{".state/**"},
			},
			want: []string{"rsync", "--exclude=.state/**", "-pthrvz", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "flags override",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				flags:      []string{"-a", "--checksum"},
			},
			want: []string{"rsync", "-a", "--checksum", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "flags cleared",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				flags:      []string{},
			},
			want: []string{"rsync", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "delete excludes and flags",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				delete:     true,
				excludes:   []string{"**/.env"},
				flags:      []string{"-a"},
			},
			want: []string{"rsync", "--delete", "--exclude=**/.env", "-a", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "sudo",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				sudo:       &Sudo{},
			},
			want: []string{"rsync", "-pthrvz", "--rsync-path=sudo rsync", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "sudo user and flags",
			opts: rsyncOptions{
				remoteSpec: "deploy@example.com",
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				sudo:       &Sudo{User: "deploy", Flags: []string{"-n"}},
			},
			want: []string{"rsync", "-pthrvz", "--rsync-path=sudo -n -u deploy rsync", "--rsh=ssh", "./app", "deploy@example.com:/home/app"},
		},
		{
			name: "native placeholder",
			opts: rsyncOptions{
				remoteSpec: rsyncPlaceholderHost,
				localPath:  "./app",
				remotePath: "/home/app",
				direction:  RsyncDirectionUpload,
				rsh:        rsyncRshCommand(),
			},
			want: []string{"rsync", "-pthrvz", "--rsh=" + rsyncRshCommand(), "./app", rsyncPlaceholderHost + ":/home/app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			if opts.remoteSpec == "" {
				opts.remoteSpec = "deploy@example.com"
			}
			if opts.rsh == "" {
				opts.rsh = "ssh"
			}
			if opts.flags == nil {
				opts.flags = []string{"-pthrvz"}
			}

			got := buildRsyncArgs(opts)
			if !slices.Equal(got, tt.want) {
				t.Errorf("buildRsyncArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRsyncRshContext(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		ctx := testContext(t)

		if HasRsyncRsh(ctx) {
			t.Error("HasRsyncRsh() = true, want false")
		}
		if _, err := GetRsyncRsh(ctx); !errors.Is(err, ErrNoRsyncRsh) {
			t.Fatalf("GetRsyncRsh() error = %v, want ErrNoRsyncRsh", err)
		}
	})

	t.Run("NoRsyncRsh", func(t *testing.T) {
		ctx := NewRsyncRshContext(testContext(t), NoRsyncRsh)

		if !HasRsyncRsh(ctx) {
			t.Error("HasRsyncRsh() = false, want true")
		}
		if _, err := GetRsyncRsh(ctx); !errors.Is(err, ErrNoRsyncRsh) {
			t.Fatalf("GetRsyncRsh() error = %v, want ErrNoRsyncRsh", err)
		}
	})

	t.Run("resolver", func(t *testing.T) {
		want := RsyncRsh{HostSpec: "deploy@example.com", Rsh: "ssh"}
		ctx := NewRsyncRshContext(testContext(t), func(context.Context) (RsyncRsh, error) {
			return want, nil
		})

		if !HasRsyncRsh(ctx) {
			t.Error("HasRsyncRsh() = false, want true")
		}

		got, err := GetRsyncRsh(ctx)
		if err != nil {
			t.Fatalf("GetRsyncRsh() error: %v", err)
		}
		if got != want {
			t.Errorf("GetRsyncRsh() = %#v, want %#v", got, want)
		}
	})

	t.Run("nil resolver panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("NewRsyncRshContext() did not panic")
			}
		}()

		NewRsyncRshContext(testContext(t), nil)
	})
}

func TestRsyncOpsRunErrValidation(t *testing.T) {
	tests := []struct {
		name string
		ops  *RsyncOps
	}{
		{
			name: "missing direction",
			ops:  Remote(context.Background()).Rsync().Local("./app").Remote("/home/app"),
		},
		{
			name: "missing local path",
			ops:  Remote(context.Background()).Rsync().Remote("/home/app").Upload(),
		},
		{
			name: "missing remote path",
			ops:  Remote(context.Background()).Rsync().Local("./app").Upload(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ops.RunErr(); err == nil {
				t.Fatal("RunErr() = nil, want error")
			}
		})
	}
}

func TestRsyncOpsRunPanics(t *testing.T) {
	tests := []struct {
		name string
		ops  *RsyncOps
	}{
		{
			name: "missing direction",
			ops:  Remote(context.Background()).Rsync().Local("./app").Remote("/home/app"),
		},
		{
			name: "missing local path",
			ops:  Remote(context.Background()).Rsync().Remote("/home/app").Upload(),
		},
		{
			name: "missing remote path",
			ops:  Remote(context.Background()).Rsync().Local("./app").Upload(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("Run() did not panic")
				}
			}()

			tt.ops.Run()
		})
	}
}

func TestRsyncOpsFlags(t *testing.T) {
	ops := Remote(context.Background()).Rsync()

	if !slices.Equal(ops.flags, []string{"-pthrvz"}) {
		t.Fatalf("default flags = %#v, want [-pthrvz]", ops.flags)
	}

	ops.AddFlags("--info")
	if !slices.Equal(ops.flags, []string{"-pthrvz", "--info"}) {
		t.Errorf("AddFlags() flags = %#v, want [-pthrvz --info]", ops.flags)
	}

	ops.SetFlags("-a", "--checksum")
	if !slices.Equal(ops.flags, []string{"-a", "--checksum"}) {
		t.Errorf("SetFlags() flags = %#v, want [-a --checksum]", ops.flags)
	}

	ops.SetFlags()
	if len(ops.flags) != 0 {
		t.Errorf("SetFlags() flags = %#v, want empty", ops.flags)
	}
}

func TestRsyncOpsCaptureFlags(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(r *RsyncOps)
		wantStdout bool
		wantStderr bool
	}{
		{name: "none", configure: func(r *RsyncOps) {}, wantStdout: false, wantStderr: false},
		{name: "stdout", configure: func(r *RsyncOps) { r.CaptureStdout() }, wantStdout: true, wantStderr: false},
		{name: "stderr", configure: func(r *RsyncOps) { r.CaptureStderr() }, wantStdout: false, wantStderr: true},
		{name: "both", configure: func(r *RsyncOps) { r.Capture() }, wantStdout: true, wantStderr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := Remote(context.Background()).Rsync()
			tt.configure(ops)

			if ops.captureStdout != tt.wantStdout || ops.captureStderr != tt.wantStderr {
				t.Errorf("capture flags = stdout %v, stderr %v; want stdout %v, stderr %v",
					ops.captureStdout, ops.captureStderr, tt.wantStdout, tt.wantStderr)
			}
		})
	}
}

func TestRsyncOpsExclude(t *testing.T) {
	ops := Remote(context.Background()).Rsync().Exclude("**/.env", ".state/**").Exclude("acme.json")

	if !slices.Equal(ops.excludes, []string{"**/.env", ".state/**", "acme.json"}) {
		t.Errorf("excludes = %#v, want [**/.env .state/** acme.json]", ops.excludes)
	}
}
