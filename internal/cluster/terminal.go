package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/huypham37/k8sgames/internal/game"
)

type kubectlTerminal struct {
	file *os.File
	cmd  *exec.Cmd
	once sync.Once
}

func (k *Kubectl) OpenTerminal(ctx context.Context, namespace string, challenge game.Challenge, size game.TerminalSize) (game.Terminal, error) {
	if !validNamespace(namespace) {
		return nil, fmt.Errorf("invalid namespace %q", namespace)
	}
	args := []string{
		"--namespace", namespace, "exec", "-it", "toolbox", "--", "env",
		"K8SGAMES_OBJECTIVE=" + challenge.Objective, "K8SGAMES_HINT=" + challenge.Hint,
		"bash", "--rcfile", "/home/player/.bashrc", "-i",
	}
	cmd := exec.CommandContext(ctx, k.path, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	window := &pty.Winsize{Cols: size.Columns, Rows: size.Rows}
	file, err := pty.StartWithSize(cmd, window)
	if err != nil {
		return nil, err
	}
	terminal := &kubectlTerminal{file: file, cmd: cmd}
	go func() {
		_ = cmd.Wait()
		_ = terminal.Close()
	}()
	return terminal, nil
}

func (t *kubectlTerminal) Read(buffer []byte) (int, error)  { return t.file.Read(buffer) }
func (t *kubectlTerminal) Write(buffer []byte) (int, error) { return t.file.Write(buffer) }

func (t *kubectlTerminal) Resize(size game.TerminalSize) error {
	return pty.Setsize(t.file, &pty.Winsize{Cols: size.Columns, Rows: size.Rows})
}

func (t *kubectlTerminal) Close() error {
	var err error
	t.once.Do(func() {
		err = t.file.Close()
		if t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}
	})
	return err
}
