package sshremote

import (
	"flag"
	"io"
	"net"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
)

type sshProxyCommand struct {
	dest           string
	user           string
	port           int
	keys           []string
	forward        string   // -W value ("%h:%p" or host:port); empty when unset
	ncHost         string   // nc trailer host; empty when unset
	ncPort         string   // nc trailer port; empty when unset
	sessionCommand []string // trailing command to run on dest; nil when unset
}

// parseSSHProxyCommand parses a ProxyCommand that invokes ssh into its
// destination and the supported options. It reports false for anything it does
// not fully recognize: unknown flags, a missing destination, or a malformed -W
// value.
func parseSSHProxyCommand(value string) (sshProxyCommand, bool) {
	tokens, err := shellquote.Split(value)
	if err != nil || len(tokens) == 0 {
		return sshProxyCommand{}, false
	}

	if filepath.Base(tokens[0]) != "ssh" {
		return sshProxyCommand{}, false
	}

	fs := flag.NewFlagSet("ssh", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var parsed sshProxyCommand
	var forward string

	fs.StringVar(&parsed.user, "l", "", "")
	fs.IntVar(&parsed.port, "p", 0, "")
	fs.Func("i", "", func(v string) error {
		parsed.keys = append(parsed.keys, v)
		return nil
	})
	fs.StringVar(&forward, "W", "", "")

	if err := fs.Parse(tokens[1:]); err != nil {
		return sshProxyCommand{}, false
	}

	rest := fs.Args()

	switch {
	case forward != "":
		if len(rest) != 1 {
			return sshProxyCommand{}, false
		}
		if forward != "%h:%p" {
			if _, _, err := net.SplitHostPort(forward); err != nil {
				return sshProxyCommand{}, false
			}
		}
		parsed.dest = rest[0]
		parsed.forward = forward
	case len(rest) == 1:
		parsed.dest = rest[0]
	case len(rest) == 4 && rest[1] == "nc" && validNetcatPort(rest[3]):
		parsed.dest = rest[0]
		parsed.ncHost = rest[2]
		parsed.ncPort = rest[3]
	case len(rest) >= 2:
		parsed.dest = rest[0]
		parsed.sessionCommand = rest[1:]
	default:
		return sshProxyCommand{}, false
	}

	return parsed, true
}

// validNetcatPort reports whether ncPort is "%p" or a TCP port number.
func validNetcatPort(ncPort string) bool {
	if ncPort == "%p" {
		return true
	}

	p, err := strconv.Atoi(ncPort)

	return err == nil && p > 0 && p <= 65535
}

// parseNetcatCommand parses a ProxyCommand that invokes nc, returning the
// connection host and port. Flags are ignored and the last two non-flag tokens
// are the host and port; "%h"/"%p" are returned as-is.
func parseNetcatCommand(value string) (ncHost string, ncPort string, ok bool) {
	tokens, err := shellquote.Split(value)
	if err != nil || len(tokens) == 0 {
		return "", "", false
	}

	if filepath.Base(tokens[0]) != "nc" {
		return "", "", false
	}

	var positional []string

	for _, tok := range tokens[1:] {
		if strings.HasPrefix(tok, "-") {
			continue
		}

		positional = append(positional, tok)
	}

	if len(positional) < 2 {
		return "", "", false
	}

	return positional[len(positional)-2], positional[len(positional)-1], true
}

// resolveProxyCommand resolves a ProxyCommand into a Transport. The returned
// bool reports whether the value was recognized (a recognized value with a nil
// Transport means a direct connection, e.g. "nc %h %p").
func (c *Config) resolveProxyCommand(value string, host *Host, seen map[string]bool) (Transport, bool, error) {
	if strings.TrimSpace(value) == "-" {
		return StdioTransport{}, true, nil
	}

	if parsed, ok := parseSSHProxyCommand(value); ok {
		jump, err := c.resolveHost(parsed.dest, true, seen)
		if err != nil {
			return nil, false, err
		}

		if parsed.user != "" {
			jump.user = parsed.user
		}
		if parsed.port != 0 {
			jump.port = parsed.port
		}
		if len(parsed.keys) > 0 {
			jump.privateKeysPaths = append(slices.Clone(parsed.keys), jump.privateKeysPaths...)
		}
		jump.finish()

		if parsed.sessionCommand != nil {
			return SessionCommandTransport{Host: jump, Command: shellquote.Join(parsed.sessionCommand...)}, true, nil
		}

		addr := ""

		switch {
		case parsed.forward != "" && parsed.forward != "%h:%p":
			addr = parsed.forward
		case parsed.ncHost != "":
			a, ok := netcatAddr(parsed.ncHost, parsed.ncPort, host)
			if !ok {
				return nil, false, nil
			}
			if a != host.GetHost() {
				addr = a
			}
		}

		if addr == "" {
			return JumpTransport{Host: jump}, true, nil
		}

		return ForwardTransport{Host: jump, Addr: addr}, true, nil
	}

	if ncHost, ncPort, ok := parseNetcatCommand(value); ok {
		addr, ok := netcatAddr(ncHost, ncPort, host)
		if !ok {
			return nil, false, nil
		}

		if addr == host.GetHost() {
			// "nc %h %p" is a direct connection to the host.
			return nil, true, nil
		}

		return DialTransport{Addr: addr}, true, nil
	}

	return nil, false, nil
}

// netcatAddr expands the nc host/port tokens (%h/%p) and validates the port.
func netcatAddr(ncHost, ncPort string, host *Host) (string, bool) {
	if ncHost == "%h" {
		ncHost = host.Hostname()
	}

	port := 0

	if ncPort == "%p" {
		port = host.Port()
	} else {
		p, err := strconv.Atoi(ncPort)
		if err != nil || p <= 0 || p > 65535 {
			return "", false
		}

		port = p
	}

	return net.JoinHostPort(ncHost, strconv.Itoa(port)), true
}
