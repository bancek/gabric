package rsync

import (
	"errors"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func TestSatisfiedData(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   bool
	}{
		{"converged", "", true},
		{"drift", ">file\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().HandleRsync(func(gabric.RsyncRequest) gabrictest.Result {
				return gabrictest.Result{Stdout: tt.stdout}
			})

			if got := gt.RunNode(gt.Context(), SatisfiedData(input.Static("/srv/src"), input.Static("/srv/dst"), input.False, nil)); got != tt.want {
				t.Errorf("SatisfiedData() = %v, want %v", got, tt.want)
			}

			want := gabric.RsyncRequest{
				LocalPath:  "/srv/src",
				RemotePath: "/srv/dst",
				Direction:  gabric.RsyncDirectionUpload,
				Flags:      []string{"-pthrvz", "-n", "-i"},
			}
			gt.Remote().AssertCalls(gabrictest.Call{Rsync: &want})
		})
	}
}

func TestSyncProbe(t *testing.T) {
	tests := []struct {
		name string
		node *reconcile.Node[bool]
		want gabric.RsyncRequest
	}{
		{
			name: "default",
			node: Sync(input.Static("/srv/src")).Remote(input.Static("/srv/dst")).Build().Node(),
			want: gabric.RsyncRequest{
				LocalPath:  "/srv/src",
				RemotePath: "/srv/dst",
				Direction:  gabric.RsyncDirectionUpload,
				Flags:      []string{"-pthrvz", "-n", "-i"},
			},
		},
		{
			name: "delete",
			node: Sync(input.Static("/srv/src")).Remote(input.Static("/srv/dst")).Delete().Build().Node(),
			want: gabric.RsyncRequest{
				LocalPath:  "/srv/src",
				RemotePath: "/srv/dst",
				Direction:  gabric.RsyncDirectionUpload,
				Delete:     true,
				Flags:      []string{"-pthrvz", "-n", "-i"},
			},
		},
		{
			name: "exclude",
			node: Sync(input.Static("/srv/src")).Remote(input.Static("/srv/dst")).Exclude(input.List("**/.env", ".state/**")).Build().Node(),
			want: gabric.RsyncRequest{
				LocalPath:  "/srv/src",
				RemotePath: "/srv/dst",
				Direction:  gabric.RsyncDirectionUpload,
				Excludes:   []string{"**/.env", ".state/**"},
				Flags:      []string{"-pthrvz", "-n", "-i"},
			},
		},
		{
			name: "delete and exclude",
			node: Sync(input.Static("/srv/src")).Remote(input.Static("/srv/dst")).Delete().Exclude(input.List("**/.env")).Build().Node(),
			want: gabric.RsyncRequest{
				LocalPath:  "/srv/src",
				RemotePath: "/srv/dst",
				Direction:  gabric.RsyncDirectionUpload,
				Delete:     true,
				Excludes:   []string{"**/.env"},
				Flags:      []string{"-pthrvz", "-n", "-i"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.RunNode(gt.Context(), tt.node)

			gt.Remote().AssertRsyncCalled(tt.want)
			gt.Remote().AssertCalls(gabrictest.Call{Rsync: &tt.want})
			gt.Local().AssertCalls()
			gt.Remote().AssertCommands()
			gt.Local().AssertCommands()
		})
	}
}

func TestSyncApply(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().HandleRsync(func(gabric.RsyncRequest) gabrictest.Result {
		return gabrictest.Result{Stdout: ">file\n"}
	})

	node := Sync(input.Static("/srv/src")).Remote(input.Static("/srv/dst")).Build().Node()
	gt.RunNode(gt.Context(), node)

	probe := gabric.RsyncRequest{
		LocalPath:  "/srv/src",
		RemotePath: "/srv/dst",
		Direction:  gabric.RsyncDirectionUpload,
		Flags:      []string{"-pthrvz", "-n", "-i"},
	}
	apply := gabric.RsyncRequest{
		LocalPath:  "/srv/src",
		RemotePath: "/srv/dst",
		Direction:  gabric.RsyncDirectionUpload,
		Flags:      []string{"-pthrvz"},
	}

	gt.Remote().AssertCalls(
		gabrictest.Call{Rsync: &probe},
		gabrictest.Call{Rsync: &apply},
	)
}

func TestSyncLocalTargetUnsupported(t *testing.T) {
	gt := gabrictest.New(t)
	ctx := gabric.NewTargetContext(gt.Context(), gabric.LocalTarget)

	_, err := gt.RunNodeErr(ctx, SatisfiedData(input.Static("/srv/src"), input.Static("/srv/dst"), input.False, nil))
	if !errors.Is(err, gabric.ErrNoRsyncTarget) {
		t.Fatalf("SatisfiedData() error = %v, want ErrNoRsyncTarget", err)
	}
}

func TestSyncLocalRemoteTarget(t *testing.T) {
	gt := gabrictest.New(t)
	ctx := gabric.NewTargetContext(gt.Context(), gabric.LocalRemoteTarget)

	gt.RunNode(ctx, SatisfiedData(input.Static("/srv/src"), input.Static("/srv/dst"), input.False, nil))

	want := gabric.RsyncRequest{
		LocalPath:  "/srv/src",
		RemotePath: "/srv/dst",
		Direction:  gabric.RsyncDirectionUpload,
		Flags:      []string{"-pthrvz", "-n", "-i"},
	}

	gt.Remote().AssertRsyncCalled(want)
}

func TestSyncProbeDryRun(t *testing.T) {
	gt := gabrictest.New(t)

	node := Sync(input.Static("/srv/src")).Remote(input.Static("/srv/dst")).Build().Node()
	gt.RunNode(dryrun.With(gt.Context()), node)

	want := gabric.RsyncRequest{
		LocalPath:  "/srv/src",
		RemotePath: "/srv/dst",
		Direction:  gabric.RsyncDirectionUpload,
		Flags:      []string{"-pthrvz", "-n", "-i"},
	}

	gt.Remote().AssertRsyncCalled(want)
	gt.Remote().AssertCalls(gabrictest.Call{Rsync: &want})
}
