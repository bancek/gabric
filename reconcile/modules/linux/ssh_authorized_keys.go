package linux

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
)

// SSHAuthorizedKeysGroup is the user's .ssh directory and authorized_keys file.
// Reading the group runs the directory and file resources in order.
type SSHAuthorizedKeysGroup struct {
	*reconcile.Node[reconcile.Void]

	Dir  *reconcile.Node[bool]
	File *reconcile.Node[bool]
}

// SSHAuthorizedKeys ensures user's ~/.ssh/authorized_keys contains keys. The .ssh
// directory is mode 0700 and the file mode 0600, both owned by the user.
func SSHAuthorizedKeys(userInput input.Input[string], keysInput input.Input[[]string]) *SSHAuthorizedKeysGroup {
	if userInput == nil {
		panic("linux: SSHAuthorizedKeys: nil user")
	}
	if keysInput == nil {
		panic("linux: SSHAuthorizedKeys: nil keys")
	}

	content := SSHAuthorizedKeysContentData(input.WithName(keysInput, userInput.Name()))

	passwd := UserPasswdData(userInput)
	group := UserGroupData(userInput)
	groupName := reconcile.Map(group, func(ctx context.Context, name string) (string, error) {
		if name != "" {
			return name, nil
		}

		return userInput.Value(ctx)
	})

	dirPath := SSHAuthorizedKeysDirPathData(passwd)
	filePath := SSHAuthorizedKeysFilePathData(dirPath)

	dir := modules.Dir(input.WithName(dirPath, userInput.Name())).
		Mode(modules.FileMode(0o700)).
		User(userInput).
		Group(groupName).
		Build().
		Node()

	file := modules.File(input.WithName(filePath, userInput.Name())).
		Content(content).
		Mode(modules.FileMode(0o600)).
		User(userInput).
		Group(groupName).
		Build().
		ApplyNeeds(dir).
		Node()

	node := reconcile.Group("linux.ssh-authorized-keys:"+userInput.Name()).
		Contains(dir, file).
		Node()

	return &SSHAuthorizedKeysGroup{
		Node: node,
		Dir:  dir,
		File: file,
	}
}

// SSHAuthorizedKeysContentData renders keys as authorized_keys content, one
// trimmed non-empty key per line, as a data node (dry-run safe, memoized once
// per run). It yields nil when no keys remain.
func SSHAuthorizedKeysContentData(keysInput input.Input[[]string]) *reconcile.Node[[]byte] {
	if keysInput == nil {
		panic("linux: SSHAuthorizedKeysContentData: nil keys")
	}

	return reconcile.Data("linux.ssh-authorized-keys-content:"+keysInput.Name(), func(ctx context.Context) ([]byte, error) {
		keys, err := keysInput.Value(ctx)
		if err != nil {
			return nil, err
		}

		return sshAuthorizedKeysContent(keys), nil
	}).Node()
}

// SSHAuthorizedKeysDirPathData reads the user's .ssh directory path from a passwd
// data node. It errors when the user or home directory is missing.
func SSHAuthorizedKeysDirPathData(passwd *reconcile.Node[UserPasswd]) *reconcile.Node[string] {
	if passwd == nil {
		panic("linux: nil ssh authorized keys passwd node")
	}

	return reconcile.Data("linux.ssh-authorized-keys-dir-path:"+passwd.Name(), func(ctx context.Context) (string, error) {
		info, err := passwd.Value(ctx)
		if err != nil {
			return "", err
		}
		if !info.Exists || info.Home == "" {
			return "", fmt.Errorf("linux.ssh-authorized-keys: user %q not found", info.Name)
		}

		return path.Join(info.Home, ".ssh"), nil
	}).Node()
}

// SSHAuthorizedKeysFilePathData reads the authorized_keys path from the .ssh
// directory path data node.
func SSHAuthorizedKeysFilePathData(dir *reconcile.Node[string]) *reconcile.Node[string] {
	if dir == nil {
		panic("linux: nil ssh authorized keys directory node")
	}

	return reconcile.Data("linux.ssh-authorized-keys-file-path:"+dir.Name(), func(ctx context.Context) (string, error) {
		dir, err := dir.Value(ctx)
		if err != nil {
			return "", err
		}

		return path.Join(dir, "authorized_keys"), nil
	}).Node()
}

func sshAuthorizedKeysContent(keys []string) []byte {
	var lines []string

	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" {
			lines = append(lines, key)
		}
	}

	if len(lines) == 0 {
		return nil
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}
