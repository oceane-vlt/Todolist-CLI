package server

import "testing"

func TestListenAddrFromEnv(t *testing.T) {
	tests := []struct {
		name       string
		listenAddr string
		port       string
		want       string
	}{
		{
			name: "no env -> loopback default unchanged",
			want: "127.0.0.1:50051",
		},
		{
			name: "PORT set -> binds all interfaces (PaaS)",
			port: "8080",
			want: "0.0.0.0:8080",
		},
		{
			name:       "TODO_LISTEN_ADDR overrides verbatim",
			listenAddr: "0.0.0.0:50051",
			want:       "0.0.0.0:50051",
		},
		{
			name:       "TODO_LISTEN_ADDR takes precedence over PORT",
			listenAddr: "127.0.0.1:9000",
			port:       "8080",
			want:       "127.0.0.1:9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvListenAddr, tt.listenAddr)
			t.Setenv(EnvPort, tt.port)

			if got := ListenAddrFromEnv(); got != tt.want {
				t.Errorf("ListenAddrFromEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsLoopbackListenAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want bool
	}{
		// Loopback: safe for the unauthenticated dev mode.
		{name: "loopback IPv4 default", addr: "127.0.0.1:50051", want: true},
		{name: "loopback IPv4 other in 127/8", addr: "127.1.2.3:50051", want: true},
		{name: "loopback IPv6", addr: "[::1]:50051", want: true},
		{name: "localhost hostname", addr: "localhost:50051", want: true},
		{name: "bare loopback IP without port", addr: "127.0.0.1", want: true},
		{name: "bare localhost without port", addr: "localhost", want: true},

		// Non-loopback: must refuse the unauthenticated dev mode.
		{name: "wildcard IPv4 (PaaS PORT form)", addr: "0.0.0.0:8080", want: false},
		{name: "wildcard IPv6", addr: "[::]:8080", want: false},
		{name: "empty host binds all interfaces", addr: ":50051", want: false},
		{name: "routable IPv4", addr: "192.168.1.10:50051", want: false},
		{name: "public hostname", addr: "todo.example.com:50051", want: false},
		{name: "empty string fails safe", addr: "", want: false},
		{name: "garbage fails safe", addr: "not an address", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLoopbackListenAddr(tt.addr); got != tt.want {
				t.Errorf("IsLoopbackListenAddr(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}
