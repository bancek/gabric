package shelltest

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	errNotDir         = errors.New("not a directory")
	errIsDir          = errors.New("is a directory")
	errDirNotEmpty    = errors.New("directory not empty")
	errMoveIntoItself = errors.New("cannot move a directory into itself")
	errRemoveRoot     = errors.New("cannot remove root")
	errSymlinkLoop    = errors.New("too many levels of symbolic links")
)

const maxSymlinkDepth = 40

// FS is an in-memory filesystem for shell tests.
type FS struct {
	mu   sync.Mutex
	root *node
}

type node struct {
	mode     fs.FileMode
	data     []byte
	children map[string]*node
	link     string
	uid, gid int
	user     string
	group    string
	modTime  time.Time
}

// NewFS returns an empty in-memory filesystem.
func NewFS() *FS {
	return &FS{
		root: &node{
			mode:     fs.ModeDir | 0o755,
			children: map[string]*node{},
		},
	}
}

// Resolve returns the cleaned remote path for name, relative to cwd.
func (f *FS) Resolve(cwd, name string) (string, error) {
	if cwd == "" {
		cwd = "/"
	}

	cwd = virtualName(cwd)
	name = virtualName(name)

	if !path.IsAbs(cwd) {
		return "", fmt.Errorf("cwd %q is not absolute", cwd)
	}

	var remotePath string
	if path.IsAbs(name) {
		remotePath = path.Clean(name)
	} else {
		remotePath = path.Clean(path.Join(cwd, name))
	}

	if remotePath == "." {
		return "/", nil
	}

	return remotePath, nil
}

// ReadFile returns the content of name.
func (f *FS) ReadFile(name string) ([]byte, error) {
	return f.readFileAt("/", name)
}

func (f *FS) readFileAt(cwd, name string) ([]byte, error) {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	n, err := f.nodeLocked(remotePath)
	if err != nil {
		return nil, &fs.PathError{Op: "read", Path: name, Err: err}
	}

	if n.mode.IsDir() {
		return nil, &fs.PathError{Op: "read", Path: name, Err: errIsDir}
	}

	return slices.Clone(n.data), nil
}

// WriteFile writes data to name, creating parent directories.
func (f *FS) WriteFile(name string, data []byte, mode fs.FileMode) error {
	remotePath, err := f.Resolve("/", name)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	n, resolved, err := f.lookupLocked(remotePath, true)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return &fs.PathError{Op: "write", Path: name, Err: err}
		}

		parentPath := path.Dir(resolved)
		base := path.Base(resolved)

		if base == "/" || base == "." {
			return &fs.PathError{Op: "write", Path: name, Err: errIsDir}
		}

		parent, err := f.mkdirAllLocked(parentPath, 0o777)
		if err != nil {
			return &fs.PathError{Op: "write", Path: name, Err: err}
		}

		parent.children[base] = &node{
			mode:    mode.Perm(),
			data:    slices.Clone(data),
			modTime: now,
		}

		return nil
	}

	if n.mode.IsDir() {
		return &fs.PathError{Op: "write", Path: name, Err: errIsDir}
	}

	n.mode = mode.Perm()
	n.data = slices.Clone(data)
	n.modTime = now

	return nil
}

// Symlink creates a symbolic link at link pointing at target.
func (f *FS) Symlink(target, link string) error {
	target = virtualName(target)
	if target == "" {
		return &fs.PathError{Op: "symlink", Path: link, Err: fs.ErrInvalid}
	}

	linkPath, err := f.Resolve("/", link)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	parentPath := path.Dir(linkPath)
	base := path.Base(linkPath)

	if base == "/" || base == "." {
		return &fs.PathError{Op: "symlink", Path: link, Err: fs.ErrExist}
	}

	parent, err := f.nodeLocked(parentPath)
	if err != nil {
		return &fs.PathError{Op: "symlink", Path: link, Err: err}
	}

	if !parent.mode.IsDir() {
		return &fs.PathError{Op: "symlink", Path: link, Err: errNotDir}
	}

	if _, ok := parent.children[base]; ok {
		return &fs.PathError{Op: "symlink", Path: link, Err: fs.ErrExist}
	}

	parent.children[base] = &node{
		mode:    fs.ModeSymlink | 0o777,
		link:    target,
		modTime: time.Now(),
	}

	return nil
}

// ReadDir returns the entries of directory name, sorted by name.
func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	return f.readDirAt("/", name)
}

func (f *FS) readDirAt(cwd, name string) ([]fs.DirEntry, error) {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	n, err := f.nodeLocked(remotePath)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}

	if !n.mode.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errNotDir}
	}

	entries := make([]fs.DirEntry, 0, len(n.children))

	for childName, child := range n.children {
		entries = append(entries, &dirEntry{name: childName, node: child})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	return entries, nil
}

// Stat returns information about name.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	return f.statAt("/", name, true)
}

// Lstat returns information about name without following symlinks.
func (f *FS) Lstat(name string) (fs.FileInfo, error) {
	return f.statAt("/", name, false)
}

func (f *FS) statAt(cwd, name string, followSymlinks bool) (fs.FileInfo, error) {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	n, _, err := f.lookupLocked(remotePath, followSymlinks)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	base := path.Base(remotePath)
	if remotePath == "/" {
		base = "/"
	}

	return &fileInfo{name: base, node: n}, nil
}

// SetOwner sets the owner labels of name.
func (f *FS) SetOwner(name, user, group string) error {
	return f.setOwnerAt("/", name, user, group)
}

func (f *FS) setOwnerAt(cwd, name, user, group string) error {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	n, err := f.nodeLocked(remotePath)
	if err != nil {
		return &fs.PathError{Op: "chown", Path: name, Err: err}
	}

	n.user, n.group = user, group

	return nil
}

// Owner returns the owner labels of name.
func (f *FS) Owner(name string) (string, string, error) {
	return f.ownerAt("/", name, true)
}

func (f *FS) ownerAt(cwd, name string, followSymlinks bool) (string, string, error) {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return "", "", err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	n, _, err := f.lookupLocked(remotePath, followSymlinks)
	if err != nil {
		return "", "", &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	return n.user, n.group, nil
}

// MkdirAll creates name and any missing parents.
func (f *FS) MkdirAll(name string, mode fs.FileMode) error {
	return f.mkdirAllAt("/", name, mode)
}

// Mkdir creates name. The parent directory must already exist.
func (f *FS) Mkdir(name string, mode fs.FileMode) error {
	return f.mkdirAt("/", name, mode)
}

func (f *FS) mkdirAllAt(cwd, name string, mode fs.FileMode) error {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, err := f.mkdirAllLocked(remotePath, mode); err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}

	return nil
}

func (f *FS) mkdirAt(cwd, name string, mode fs.FileMode) error {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if remotePath == "/" {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	parentPath := path.Dir(remotePath)
	base := path.Base(remotePath)

	parent, err := f.nodeLocked(parentPath)
	if err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: err}
	}

	if !parent.mode.IsDir() {
		return &fs.PathError{Op: "mkdir", Path: name, Err: errNotDir}
	}

	if _, ok := parent.children[base]; ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}

	parent.children[base] = &node{
		mode:     fs.ModeDir | mode.Perm(),
		children: map[string]*node{},
		modTime:  time.Now(),
	}

	return nil
}

func (f *FS) chmodAt(cwd, name string, mode fs.FileMode) error {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	n, err := f.nodeLocked(remotePath)
	if err != nil {
		return &fs.PathError{Op: "chmod", Path: name, Err: err}
	}

	const modeBits = fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky
	n.mode = (n.mode &^ modeBits) | (mode & modeBits)

	return nil
}

// Chmod changes the permission bits of name.
func (f *FS) Chmod(name string, mode fs.FileMode) error {
	return f.chmodAt("/", name, mode)
}

// Rename moves oldName to newName.
func (f *FS) Rename(oldName, newName string) error {
	return f.renameAt("/", oldName, newName)
}

func (f *FS) renameAt(cwd, oldName, newName string) error {
	oldPath, err := f.Resolve(cwd, oldName)
	if err != nil {
		return err
	}

	newPath, err := f.Resolve(cwd, newName)
	if err != nil {
		return err
	}

	if oldPath == newPath {
		return &fs.PathError{Op: "rename", Path: oldName, Err: errMoveIntoItself}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if oldPath == "/" {
		return &fs.PathError{Op: "rename", Path: oldName, Err: errMoveIntoItself}
	}

	if strings.HasPrefix(newPath, oldPath+"/") {
		return &fs.PathError{Op: "rename", Path: oldName, Err: errMoveIntoItself}
	}

	oldParentPath := path.Dir(oldPath)
	oldBase := path.Base(oldPath)

	oldParent, err := f.nodeLocked(oldParentPath)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: oldName, Err: err}
	}

	n, ok := oldParent.children[oldBase]
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldName, Err: fs.ErrNotExist}
	}

	newParentPath := path.Dir(newPath)
	newBase := path.Base(newPath)

	newParent, err := f.nodeLocked(newParentPath)
	if err != nil {
		return &fs.PathError{Op: "rename", Path: newName, Err: err}
	}

	if !newParent.mode.IsDir() {
		return &fs.PathError{Op: "rename", Path: newName, Err: errNotDir}
	}

	if existing, ok := newParent.children[newBase]; ok && existing.mode.IsDir() {
		return &fs.PathError{Op: "rename", Path: newName, Err: errIsDir}
	}

	delete(oldParent.children, oldBase)
	newParent.children[newBase] = n

	return nil
}

// Remove removes name. A final symlink is removed without following it, and a
// non-empty directory cannot be removed.
func (f *FS) Remove(name string) error {
	return f.removeAt("/", name)
}

func (f *FS) removeAt(cwd, name string) error {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if remotePath == "/" {
		return &fs.PathError{Op: "remove", Path: name, Err: errRemoveRoot}
	}

	parentPath := path.Dir(remotePath)
	base := path.Base(remotePath)

	parent, err := f.nodeLocked(parentPath)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}

	if !parent.mode.IsDir() {
		return &fs.PathError{Op: "remove", Path: name, Err: errNotDir}
	}

	n, ok := parent.children[base]
	if !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}

	if n.mode.IsDir() && len(n.children) > 0 {
		return &fs.PathError{Op: "remove", Path: name, Err: errDirNotEmpty}
	}

	delete(parent.children, base)

	return nil
}

// RemoveAll removes name and its children. A final symlink is removed without
// following it, and a missing path is not an error.
func (f *FS) RemoveAll(name string) error {
	return f.removeAllAt("/", name)
}

func (f *FS) removeAllAt(cwd, name string) error {
	remotePath, err := f.Resolve(cwd, name)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if remotePath == "/" {
		return &fs.PathError{Op: "removeall", Path: name, Err: errRemoveRoot}
	}

	parentPath := path.Dir(remotePath)
	base := path.Base(remotePath)

	parent, err := f.nodeLocked(parentPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return &fs.PathError{Op: "removeall", Path: name, Err: err}
	}

	if !parent.mode.IsDir() {
		return &fs.PathError{Op: "removeall", Path: name, Err: errNotDir}
	}

	delete(parent.children, base)

	return nil
}

func (f *FS) nodeLocked(remotePath string) (*node, error) {
	n, _, err := f.lookupLocked(remotePath, true)

	return n, err
}

// lookupLocked resolves remotePath to a node. Intermediate symlinks are always
// followed; the final component is followed only when followFinal is true. The
// returned path is the link target for a dangling final symlink, so a caller
// that creates the missing entry writes the target instead of replacing the
// link.
func (f *FS) lookupLocked(remotePath string, followFinal bool) (*node, string, error) {
	n := f.root
	resolved := "/"
	parts := splitRemotePath(remotePath)
	depth := 0

	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]

		if !n.mode.IsDir() {
			return nil, resolved, errNotDir
		}

		child, ok := n.children[part]
		if !ok {
			return nil, path.Join(append([]string{resolved, part}, parts...)...), fs.ErrNotExist
		}

		childPath := path.Join(resolved, part)

		if child.mode&fs.ModeSymlink != 0 && (len(parts) > 0 || followFinal) {
			depth++
			if depth > maxSymlinkDepth {
				return nil, childPath, errSymlinkLoop
			}

			target := child.link
			if !path.IsAbs(target) {
				target = path.Join(path.Dir(childPath), target)
			}

			parts = append(splitRemotePath(path.Clean(target)), parts...)
			n = f.root
			resolved = "/"

			continue
		}

		n = child
		resolved = childPath
	}

	return n, resolved, nil
}

func (f *FS) mkdirAllLocked(remotePath string, mode fs.FileMode) (*node, error) {
	n, resolved, err := f.lookupLocked(remotePath, true)
	if err == nil {
		if !n.mode.IsDir() {
			return nil, errNotDir
		}

		return n, nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if final, _, err := f.lookupLocked(remotePath, false); err == nil && final.mode&fs.ModeSymlink != 0 {
		return nil, fs.ErrExist
	}

	return f.mkdirAllDirectLocked(resolved, mode)
}

func (f *FS) mkdirAllDirectLocked(remotePath string, mode fs.FileMode) (*node, error) {
	parts := splitRemotePath(remotePath)
	n := f.root

	for _, part := range parts {
		if !n.mode.IsDir() {
			return nil, errNotDir
		}

		child, ok := n.children[part]
		if !ok {
			child = &node{
				mode:     fs.ModeDir | mode.Perm(),
				children: map[string]*node{},
				modTime:  time.Now(),
			}
			n.children[part] = child
		} else if !child.mode.IsDir() {
			return nil, errNotDir
		}

		n = child
	}

	return n, nil
}

func splitRemotePath(remotePath string) []string {
	if remotePath == "/" {
		return nil
	}

	return strings.Split(strings.TrimPrefix(remotePath, "/"), "/")
}

type fileInfo struct {
	name string
	node *node
}

func (fi *fileInfo) Name() string {
	return fi.name
}

func (fi *fileInfo) Size() int64 {
	return int64(len(fi.node.data))
}

func (fi *fileInfo) Mode() fs.FileMode {
	return fi.node.mode
}

func (fi *fileInfo) ModTime() time.Time {
	return fi.node.modTime
}

func (fi *fileInfo) IsDir() bool {
	return fi.node.mode.IsDir()
}

func (fi *fileInfo) Sys() any {
	return fi.node
}

type dirEntry struct {
	name string
	node *node
}

func (de *dirEntry) Name() string {
	return de.name
}

func (de *dirEntry) IsDir() bool {
	return de.node.mode.IsDir()
}

func (de *dirEntry) Type() fs.FileMode {
	return de.node.mode.Type()
}

func (de *dirEntry) Info() (fs.FileInfo, error) {
	return &fileInfo{name: de.name, node: de.node}, nil
}
