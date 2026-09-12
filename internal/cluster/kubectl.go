package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/huypham37/k8sgames/internal/game"
)

type Kubectl struct {
	path         string
	timeout      time.Duration
	toolboxImage string
}

func NewKubectl(path string, timeout time.Duration, toolboxImage string) *Kubectl {
	return &Kubectl{path: path, timeout: timeout, toolboxImage: toolboxImage}
}

func (k *Kubectl) Provision(ctx context.Context, namespace string, challenge game.Challenge) error {
	if !validNamespace(namespace) {
		return fmt.Errorf("invalid namespace %q", namespace)
	}
	if result := k.run(ctx, "", nil, []string{"create", "namespace", namespace}); result.ExitCode != 0 {
		return fmt.Errorf("create namespace: %s", result.Output)
	}
	labels := []string{"label", "namespace", namespace, "app.kubernetes.io/managed-by=k8sgames", "k8sgames.io/managed=true"}
	if result := k.run(ctx, "", nil, labels); result.ExitCode != 0 {
		_ = k.Delete(context.Background(), namespace)
		return fmt.Errorf("label namespace: %s", result.Output)
	}
	manifest := strings.ReplaceAll(challenge.Manifest, "{{TOOLBOX_IMAGE}}", k.toolboxImage)
	args := []string{"--namespace", namespace, "apply", "-f", "-"}
	if result := k.run(ctx, manifest, nil, args); result.ExitCode != 0 {
		_ = k.Delete(context.Background(), namespace)
		return fmt.Errorf("apply challenge: %s", result.Output)
	}
	for _, setup := range challenge.Setup {
		if result := k.Inspect(ctx, namespace, setup); result.ExitCode != 0 {
			_ = k.Delete(context.Background(), namespace)
			return fmt.Errorf("prepare challenge: %s", result.Output)
		}
	}
	wait := []string{"--namespace", namespace, "wait", "--for=condition=Ready", "pod/toolbox", "--timeout=90s"}
	if result := k.runTimeout(ctx, 95*time.Second, "", nil, wait); result.ExitCode != 0 {
		_ = k.Delete(context.Background(), namespace)
		return fmt.Errorf("start toolbox: %s", result.Output)
	}
	return nil
}

func (k *Kubectl) Inspect(ctx context.Context, namespace string, args []string) game.Result {
	return k.run(ctx, "", []string{"--namespace", namespace}, args)
}

func (k *Kubectl) Delete(ctx context.Context, namespace string) error {
	if !validNamespace(namespace) {
		return fmt.Errorf("invalid namespace %q", namespace)
	}
	result := k.run(ctx, "", nil, []string{"delete", "namespace", namespace, "--wait=false"})
	if result.ExitCode != 0 && !strings.Contains(result.Output, "NotFound") {
		return fmt.Errorf("delete namespace: %s", result.Output)
	}
	return nil
}

func (k *Kubectl) Cleanup(ctx context.Context) error {
	args := []string{"get", "namespaces", "-l", "k8sgames.io/managed=true", "-o", "jsonpath={range .items[*]}{.metadata.name}{'\\n'}{end}"}
	result := k.run(ctx, "", nil, args)
	if result.ExitCode != 0 {
		return fmt.Errorf("list managed namespaces: %s", result.Output)
	}
	for _, namespace := range strings.Fields(result.Output) {
		if err := k.Delete(ctx, namespace); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kubectl) Ping(ctx context.Context) error {
	result := k.run(ctx, "", nil, []string{"version", "--request-timeout=5s"})
	if result.ExitCode != 0 {
		return fmt.Errorf("kubectl: %s", result.Output)
	}
	return nil
}

func (k *Kubectl) run(ctx context.Context, input string, prefix, args []string) game.Result {
	return k.runTimeout(ctx, k.timeout, input, prefix, args)
}

func (k *Kubectl) runTimeout(ctx context.Context, timeout time.Duration, input string, prefix, args []string) game.Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	all := append(append([]string{}, prefix...), args...)
	cmd := exec.CommandContext(ctx, k.path, all...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if err == nil {
		return game.Result{Output: output.String()}
	}
	if ctx.Err() != nil {
		return game.Result{Output: "command timed out\n", ExitCode: 124}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return game.Result{Output: output.String(), ExitCode: exitErr.ExitCode()}
	}
	return game.Result{Output: err.Error() + "\n", ExitCode: 1}
}

func validNamespace(namespace string) bool {
	if !strings.HasPrefix(namespace, "k8sgames-") || len(namespace) > 63 {
		return false
	}
	for _, char := range namespace {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}
