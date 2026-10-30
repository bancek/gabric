package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
)

func TestSeal(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Local().WriteFile("/secrets/recipients.txt", []byte("age1recipient\n"), 0o644)
	gt.Local().WriteFile("/secrets.plain/app.env", []byte("KEY=value\n"), 0o600)
	gt.Local().Handle("age", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	Seal(gt.Context())

	gt.Local().AssertCalls(
		gabrictest.Call{Get: &gabric.GetRequest{Path: "/secrets/recipients.txt"}},
		gabrictest.Call{List: &gabric.ListRequest{Path: "/secrets.plain"}},
	)
	gt.Local().AssertCommands(gt.Command("age",
		"-e",
		"-o", filepath.Join("secrets", "app.env.age"),
		"-r", "age1recipient",
		filepath.Join("secrets.plain", "app.env"),
	))
}

func TestUnseal(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Local().WriteFile("/secrets/sample-identity.txt", []byte("identity"), 0o600)
	gt.Local().WriteFile("/secrets/app.env.age", []byte("ciphertext"), 0o600)
	gt.Local().Handle("age", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	Unseal(gt.Context())

	gt.Local().AssertCalls(
		gabrictest.Call{Exists: &gabric.ExistsRequest{
			Path: "/secrets/sample-identity.txt",
			Kind: gabric.FileKindFile,
		}},
		gabrictest.Call{Mkdir: &gabric.MkdirRequest{
			Path: "/secrets.plain",
			Mode: 0o700,
		}},
		gabrictest.Call{List: &gabric.ListRequest{Path: "/secrets"}},
	)
	gt.Local().AssertCommands(gt.Command("age",
		"-d",
		"-i", "secrets/sample-identity.txt",
		"-o", filepath.Join("secrets.plain", "app.env"),
		filepath.Join("secrets", "app.env.age"),
	))
}

func TestAppEnv(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Local().WriteFile("/secrets/sample-identity.txt", []byte("identity"), 0o600)
	gt.Local().WriteFile("/secrets/app.env.age", []byte("ciphertext"), 0o600)
	gt.Local().Handle("age", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "KEY=value\n"}
	})

	gt.RunNode(gt.Context(), appEnv)

	gt.Local().AssertCalls(
		gabrictest.Call{Exists: &gabric.ExistsRequest{
			Path: "/secrets/sample-identity.txt",
			Kind: gabric.FileKindFile,
		}},
	)
	gt.Local().AssertCommands(gt.Command("age",
		"-d",
		"-i", "secrets/sample-identity.txt",
		"secrets/app.env.age",
	))
	gt.Remote().AssertCalls(
		gabrictest.Call{Exists: &gabric.ExistsRequest{
			Path: "/srv/app/.env",
			Kind: gabric.FileKindAny,
		}},
		gabrictest.Call{Put: &gabric.PutRequest{
			Path:         "/srv/app/.env",
			Content:      bytes.NewBufferString("KEY=value\n"),
			Mode:         0o600,
			EnsureParent: true,
		}},
	)
}

func TestAgeIdentityError(t *testing.T) {
	sentinel := errors.New("stat")
	gt := gabrictest.New(t)
	ctx := gabric.NewLocalBackendContext(gt.Context(), gabric.LocalBackendFunc{
		ExistsFunc: func(context.Context, gabric.ExistsRequest) (bool, error) {
			return false, sentinel
		},
	})

	if _, err := ageIdentity(ctx); !errors.Is(err, sentinel) {
		t.Errorf("ageIdentity() error = %v, want %v", err, sentinel)
	}
}
