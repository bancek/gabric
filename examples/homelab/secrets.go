package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

const (
	secretsDir      = "secrets"
	secretsPlainDir = "secrets.plain"
	recipientsFile  = secretsDir + "/recipients.txt"
	sampleIdentity  = secretsDir + "/sample-identity.txt"
)

// Seal encrypts every plaintext file in secrets.plain/ into secrets/<name>.age
// using the age recipients listed in secrets/recipients.txt.
var Seal = gabric.Task("seal", func(ctx context.Context) {
	recipients := readRecipients(ctx, recipientsFile)

	entries := gabric.Local(ctx).List(secretsPlainDir)

	for _, entry := range entries {
		if entry.IsDir || strings.HasPrefix(entry.Name, ".") {
			continue
		}

		in := filepath.Join(secretsPlainDir, entry.Name)
		out := filepath.Join(secretsDir, entry.Name+".age")

		args := []string{"age", "-e", "-o", out}
		for _, recipient := range recipients {
			args = append(args, "-r", recipient)
		}
		args = append(args, in)

		gabric.Local(ctx).Exec(args...).Run()

		gabric.Logger(ctx).Info("sealed", "file", out)
	}
})

// Unseal decrypts every secrets/<name>.age back into secrets.plain/<name> for
// local editing (restores plaintext on a fresh clone).
var Unseal = gabric.Task("unseal", func(ctx context.Context) {
	identity, err := ageIdentity(ctx)
	gabric.Must(err)

	gabric.Local(ctx).Mkdir(secretsPlainDir).Mode(0o700).Run()

	entries := gabric.Local(ctx).List(secretsDir)

	for _, entry := range entries {
		if entry.IsDir || !strings.HasSuffix(entry.Name, ".age") {
			continue
		}

		name := strings.TrimSuffix(entry.Name, ".age")
		src := filepath.Join(secretsDir, entry.Name)
		out := filepath.Join(secretsPlainDir, name)

		gabric.Local(ctx).Exec("age", "-d", "-i", identity, "-o", out, src).Run()

		gabric.Logger(ctx).Info("unsealed", "file", out)
	}
})

// secretFile decrypts source in memory (the age identity never leaves the
// local machine) and reconciles it to target with mode 0600.
func secretFile(target, source string) *reconcile.Node[bool] {
	return modules.File(S(target)).
		Content(reconcile.Data("secret-file:"+target, func(ctx context.Context) ([]byte, error) {
			identity, err := ageIdentity(ctx)
			if err != nil {
				return nil, err
			}

			out := gabric.Local(ctx).Exec("age", "-d", "-i", identity, source).
				CaptureStdout().Run().StdoutStr()

			return []byte(out), nil
		}).Node()).
		Mode(modules.FileMode(0o600)).
		Build().
		Node()
}

type ageIdentityContextKey struct{}

func withAgeIdentity(ctx context.Context, identity string) context.Context {
	return context.WithValue(ctx, ageIdentityContextKey{}, identity)
}

// ageIdentity resolves the age identity: -age-identity wins, then the sample
// identity committed with this example, then the local age default.
func ageIdentity(ctx context.Context) (string, error) {
	if identity, _ := ctx.Value(ageIdentityContextKey{}).(string); identity != "" {
		return identity, nil
	}

	exists, err := gabric.Local(ctx).Exists(sampleIdentity).File().RunErr()
	if err != nil {
		return "", err
	}
	if exists {
		return sampleIdentity, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "age", "keys.txt"), nil
}

func readRecipients(ctx context.Context, file string) []string {
	data, err := gabric.Local(ctx).Get(file).BytesErr()
	if err != nil {
		panic(err)
	}

	var recipients []string

	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		for field := range strings.FieldsSeq(line) {
			if strings.HasPrefix(field, "age1") {
				recipients = append(recipients, field)
			}
		}
	}

	if len(recipients) == 0 {
		panic(fmt.Sprintf("no age recipients found in %s", file))
	}

	return recipients
}
