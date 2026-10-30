package sshremote

import (
	"reflect"
	"testing"
)

func TestParseSSHProxyCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want sshProxyCommand
		ok   bool
	}{
		{name: "bare", in: "ssh jump", want: sshProxyCommand{dest: "jump"}, ok: true},
		{
			name: "flags",
			in:   "ssh -l user -p 2200 -i /key1 -i /key2 jump",
			want: sshProxyCommand{dest: "jump", user: "user", port: 2200, keys: []string{"/key1", "/key2"}},
			ok:   true,
		},
		{name: "user at host port", in: "ssh deploy@jump.example.com:2222", want: sshProxyCommand{dest: "deploy@jump.example.com:2222"}, ok: true},
		{name: "forward", in: "ssh -W %h:%p jump", want: sshProxyCommand{dest: "jump", forward: "%h:%p"}, ok: true},
		{name: "forward other", in: "ssh -W other:22 jump", want: sshProxyCommand{dest: "jump", forward: "other:22"}, ok: true},
		{name: "nc trailer", in: "ssh jump nc %h %p", want: sshProxyCommand{dest: "jump", ncHost: "%h", ncPort: "%p"}, ok: true},
		{name: "nc other", in: "ssh jump nc other 22", want: sshProxyCommand{dest: "jump", ncHost: "other", ncPort: "22"}, ok: true},
		{name: "session command", in: "ssh jump socat - TCP:%h:%p", want: sshProxyCommand{dest: "jump", sessionCommand: []string{"socat", "-", "TCP:%h:%p"}}, ok: true},
		{name: "session single", in: "ssh jump cat", want: sshProxyCommand{dest: "jump", sessionCommand: []string{"cat"}}, ok: true},
		{name: "nc bad port session", in: "ssh jump nc other notaport", want: sshProxyCommand{dest: "jump", sessionCommand: []string{"nc", "other", "notaport"}}, ok: true},
		{name: "quoted binary", in: "'/usr/bin/ssh' jump", want: sshProxyCommand{dest: "jump"}, ok: true},
		{name: "unknown flag", in: "ssh -o StrictHostKeyChecking=no jump"},
		{name: "jump chain flag", in: "ssh -J a,b jump"},
		{name: "forward bad address", in: "ssh -W other jump"},
		{name: "non ssh", in: "nc proxy 22"},
		{name: "no destination", in: "ssh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSSHProxyCommand(tt.in)
			if ok != tt.ok {
				t.Fatalf("parseSSHProxyCommand(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSSHProxyCommand(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseNetcatCommand(t *testing.T) {
	tests := []struct {
		name string
		in   string
		host string
		port string
		ok   bool
	}{
		{name: "host and port", in: "nc host.example.com 22", host: "host.example.com", port: "22", ok: true},
		{name: "flag", in: "nc -w 5 host.example.com 22", host: "host.example.com", port: "22", ok: true},
		{name: "z flag", in: "nc -z host.example.com 22", host: "host.example.com", port: "22", ok: true},
		{name: "tokens", in: "nc %h %p", host: "%h", port: "%p", ok: true},
		{name: "quoted binary", in: "'/usr/bin/nc' host 22", host: "host", port: "22", ok: true},
		{name: "missing port", in: "nc host"},
		{name: "no args", in: "nc"},
		{name: "netcat", in: "netcat host 22"},
		{name: "ssh", in: "ssh host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, ok := parseNetcatCommand(tt.in)
			if ok != tt.ok || host != tt.host || port != tt.port {
				t.Errorf("parseNetcatCommand(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.in, host, port, ok, tt.host, tt.port, tt.ok)
			}
		})
	}
}
