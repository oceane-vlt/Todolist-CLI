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
