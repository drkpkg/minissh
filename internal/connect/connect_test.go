package connect

import (
	"reflect"
	"strings"
	"testing"

	"github.com/danieluremix/minissh/internal/model"
)

func TestArgs(t *testing.T) {
	cases := []struct {
		name string
		host model.Host
		want []string
	}{
		{
			name: "bare address, default port",
			host: model.Host{Address: "10.0.0.1", Port: 22},
			want: []string{"10.0.0.1"},
		},
		{
			name: "username and non-default port",
			host: model.Host{Address: "10.0.0.1", Port: 2222, Username: "root"},
			want: []string{"-p", "2222", "root@10.0.0.1"},
		},
		{
			name: "key identity",
			host: model.Host{
				Address: "10.0.0.1", Port: 22, Username: "root",
				Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "/home/user/.ssh/id_ed25519"},
			},
			want: []string{"-i", "/home/user/.ssh/id_ed25519", "root@10.0.0.1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Args(tc.host)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Args() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCommandBuildsExpectedArgv(t *testing.T) {
	h := model.Host{Address: "10.0.0.1", Port: 2222, Username: "root"}
	cmd, err := Command(h)
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if !strings.HasSuffix(cmd.Path, "ssh") {
		t.Fatalf("expected ssh binary, got %q", cmd.Path)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "root@10.0.0.1") || !strings.Contains(joined, "-p 2222") {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
}

func TestCommandLeavesStdioUnsetForCallerToWire(t *testing.T) {
	// tea.ExecProcess only fills in Stdin/Stdout/Stderr when they're nil —
	// if Command preset them itself, that wiring would be skipped.
	cmd, err := Command(model.Host{Address: "10.0.0.1"})
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
		t.Fatal("expected Command to leave Stdin/Stdout/Stderr unset")
	}
}
