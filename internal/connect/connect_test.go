package connect

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/drkpkg/minissh/internal/model"
)

// writeFakeExecutable creates an empty, executable file named name in dir —
// enough for exec.LookPath to find it; buildCommand/sshpassCommand never
// actually run it, they only construct an argv.
func writeFakeExecutable(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
		t.Fatalf("writeFakeExecutable(%q): %v", name, err)
	}
}

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

func TestSshpassCommandUsesPromptOverrideForPassphrase(t *testing.T) {
	dir := t.TempDir()
	writeFakeExecutable(t, dir, "ssh")
	writeFakeExecutable(t, dir, "sshpass")
	t.Setenv("PATH", dir)

	h := model.Host{Address: "10.0.0.1", Identity: model.Identity{Kind: model.IdentityKey, KeyPath: "/x"}}
	bin, argv, ok := sshpassCommand("s3cret", "passphrase", h)
	if !ok {
		t.Fatal("expected ok=true when sshpass and ssh are both on PATH")
	}
	if !strings.HasSuffix(bin, "sshpass") {
		t.Fatalf("expected sshpass binary, got %q", bin)
	}
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "-P passphrase") {
		t.Fatalf("expected a -P passphrase prompt override in argv, got %v", argv)
	}
	if !strings.Contains(joined, "-p s3cret") {
		t.Fatalf("expected -p s3cret in argv, got %v", argv)
	}
}

func TestSshpassCommandOmitsPromptOverrideForPassword(t *testing.T) {
	dir := t.TempDir()
	writeFakeExecutable(t, dir, "ssh")
	writeFakeExecutable(t, dir, "sshpass")
	t.Setenv("PATH", dir)

	h := model.Host{Address: "10.0.0.1"}
	_, argv, ok := sshpassCommand("hunter2", "", h)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if strings.Contains(strings.Join(argv, " "), "-P") {
		t.Fatalf("expected no -P override for sshpass's default password-prompt match, got %v", argv)
	}
}

func TestSshpassCommandFallsBackWhenSshpassMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty: neither sshpass nor ssh present
	_, _, ok := sshpassCommand("hunter2", "", model.Host{Address: "10.0.0.1"})
	if ok {
		t.Fatal("expected ok=false when sshpass isn't on PATH")
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
