package gabrice2e

import "fmt"

type containerVolume struct {
	hostPath      string
	containerPath string
	readOnly      bool
}

type containerOptions struct {
	name         string
	privileged   bool
	cgroupnsHost bool
	tmpfs        []string
	ports        []int
	command      []string
	network      *Network
	user         string
	env          []string
	volumes      []containerVolume
	addHosts     []string
	stopSignal   string
	securityOpts []string
}

// ContainerOption configures RunContainer.
type ContainerOption func(*containerOptions)

func applyContainerOptions(opts []ContainerOption) *containerOptions {
	cfg := &containerOptions{}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// WithContainerName sets the container name. An empty name uses the generated
// default.
func WithContainerName(name string) ContainerOption {
	return func(cfg *containerOptions) {
		cfg.name = name
	}
}

// WithContainerPrivileged runs the container with --privileged.
func WithContainerPrivileged() ContainerOption {
	return func(cfg *containerOptions) {
		cfg.privileged = true
	}
}

// WithContainerCgroupnsHost runs the container with --cgroupns=host.
func WithContainerCgroupnsHost() ContainerOption {
	return func(cfg *containerOptions) {
		cfg.cgroupnsHost = true
	}
}

// WithContainerUser runs the container process as user.
func WithContainerUser(user string) ContainerOption {
	if user == "" {
		panic("gabrice2e: empty container user")
	}

	return func(cfg *containerOptions) {
		cfg.user = user
	}
}

// WithContainerEnv adds environment entries to the container.
func WithContainerEnv(kv ...string) ContainerOption {
	for _, entry := range kv {
		if entry == "" {
			panic("gabrice2e: empty container environment entry")
		}
	}

	return func(cfg *containerOptions) {
		cfg.env = append(cfg.env, kv...)
	}
}

// WithContainerVolume mounts hostPath at containerPath. readOnly appends :ro.
func WithContainerVolume(hostPath, containerPath string, readOnly bool) ContainerOption {
	if hostPath == "" || containerPath == "" {
		panic("gabrice2e: empty container volume path")
	}

	return func(cfg *containerOptions) {
		cfg.volumes = append(cfg.volumes, containerVolume{
			hostPath:      hostPath,
			containerPath: containerPath,
			readOnly:      readOnly,
		})
	}
}

// WithContainerAddHost adds a host-to-IP mapping.
func WithContainerAddHost(host, ip string) ContainerOption {
	if host == "" || ip == "" {
		panic("gabrice2e: empty container add-host entry")
	}

	return func(cfg *containerOptions) {
		cfg.addHosts = append(cfg.addHosts, host+":"+ip)
	}
}

// WithContainerStopSignal sets the container stop signal.
func WithContainerStopSignal(signal string) ContainerOption {
	if signal == "" {
		panic("gabrice2e: empty container stop signal")
	}

	return func(cfg *containerOptions) {
		cfg.stopSignal = signal
	}
}

// WithContainerSecurityOpt adds a security option.
func WithContainerSecurityOpt(opts ...string) ContainerOption {
	for _, opt := range opts {
		if opt == "" {
			panic("gabrice2e: empty container security option")
		}
	}

	return func(cfg *containerOptions) {
		cfg.securityOpts = append(cfg.securityOpts, opts...)
	}
}

// WithContainerTmpfs mounts a tmpfs at path.
func WithContainerTmpfs(path string) ContainerOption {
	if path == "" {
		panic("gabrice2e: empty tmpfs path")
	}

	return func(cfg *containerOptions) {
		cfg.tmpfs = append(cfg.tmpfs, path)
	}
}

// WithContainerPort publishes a container port on a random loopback host port.
func WithContainerPort(containerPort int) ContainerOption {
	if containerPort <= 0 || containerPort > 65535 {
		panic(fmt.Sprintf("gabrice2e: invalid container port %d", containerPort))
	}

	return func(cfg *containerOptions) {
		cfg.ports = append(cfg.ports, containerPort)
	}
}

// WithContainerCommand overrides the image command.
func WithContainerCommand(argv ...string) ContainerOption {
	return func(cfg *containerOptions) {
		cfg.command = append([]string(nil), argv...)
	}
}

// WithContainerNetwork runs the container on n.
func WithContainerNetwork(n *Network) ContainerOption {
	if n == nil {
		panic("gabrice2e: nil container network")
	}

	return func(cfg *containerOptions) {
		cfg.network = n
	}
}

func (o *containerOptions) runArgs(imageRef string) []string {
	args := []string{"run", "-d", "--rm"}

	if o.name != "" {
		args = append(args, "--name", o.name)
	}
	if o.privileged {
		args = append(args, "--privileged")
	}
	if o.cgroupnsHost {
		args = append(args, "--cgroupns=host")
	}
	if o.user != "" {
		args = append(args, "--user", o.user)
	}
	for _, entry := range o.env {
		args = append(args, "-e", entry)
	}
	for _, volume := range o.volumes {
		mount := volume.hostPath + ":" + volume.containerPath
		if volume.readOnly {
			mount += ":ro"
		}

		args = append(args, "-v", mount)
	}
	for _, entry := range o.addHosts {
		args = append(args, "--add-host", entry)
	}
	if o.stopSignal != "" {
		args = append(args, "--stop-signal", o.stopSignal)
	}
	for _, opt := range o.securityOpts {
		args = append(args, "--security-opt", opt)
	}
	for _, path := range o.tmpfs {
		args = append(args, "--tmpfs", path)
	}
	if o.network != nil {
		args = append(args, "--network", o.network.Name())
	}
	for _, port := range o.ports {
		args = append(args, "-p", fmt.Sprintf("127.0.0.1::%d", port))
	}

	args = append(args, imageRef)
	args = append(args, o.command...)

	return args
}
