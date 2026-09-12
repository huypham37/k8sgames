# K8s Games

Twenty-five terminal-based Kubernetes challenges backed by real, isolated namespaces. The server prepares each scenario, proxies `kubectl` as a namespace-scoped ServiceAccount, grades the result, and removes the environment when the player leaves.

## Player

Build the CLI and point it at a deployed server:

```sh
go build -o k8sgames ./cmd/k8sgames
./k8sgames -server https://games.example.com -list
./k8sgames -server https://games.example.com
```

Without flags, the CLI shows the challenge picker:

```text
K8s Games — Challenges

Foundations
   1. [ ] Your First Pod
   2. [✓] Deployments & ReplicaSets
   3. [ ] Scheduling Constraints
   ...

Production
  25. [ ] Full Production Readiness

Progress: 1/25
Select a number, (a)ll remaining, or (q)uit:
```

The catalog covers Foundations, Workloads, Networking, State & Config, and Production. It ports the original campaign and unique drills, plus three terminal-native diagnostics, to real Kubernetes resources. Cluster-wide exercises use namespace-safe equivalents so concurrent players remain isolated.

A selected session starts immediately:

```text
K8s Games — Broken Image
Objective: Make both replicas of deployment/web available.

k8sgames-a1b2c3 $ kubectl get pods
k8sgames-a1b2c3 $ kubectl describe pod web-...
k8sgames-a1b2c3 $ kubectl set image deployment/web web=nginx:1.27-alpine
```

No kubeconfig, kubectl installation, or local Kubernetes cluster is given to the player. Only the CLI is required. The remote shell supports native Tab completion, history, pipes, Ctrl+C, and interactive commands.

Run `check` for immediate feedback. Confirmed completions return to the picker and are stored in the operating system's user config directory. Set `K8SGAMES_PROGRESS` to use another progress file. Select `a` to play every unfinished challenge, or bypass the picker with `-challenge <id>`.

## Server

The backend needs `kubectl` and cluster-admin-equivalent provisioning permissions. For local development, use your current kubeconfig:

```sh
go run ./cmd/k8sgames-server
```

Deploy in Kubernetes after publishing the image referenced by the manifest:

```sh
docker build -t ghcr.io/huypham37/k8sgames:latest .
docker build -f Dockerfile.toolbox -t ghcr.io/huypham37/k8sgames-toolbox:latest .
kubectl apply -f deploy/kubernetes.yaml
```

Each session receives:

- A dedicated `k8sgames-*` namespace
- A ResourceQuota and LimitRange
- A namespace-only player ServiceAccount and Role
- A dedicated Bash and kubectl toolbox Pod
- Scenario-specific starting resources
- A random bearer token and a 30-minute expiration

The CLI streams the local terminal over WebSocket to a PTY attached to the toolbox Pod. The Pod uses the player ServiceAccount, so Kubernetes RBAC enforces namespace isolation without exposing backend credentials.

## Commands

```sh
go test ./...
go build ./cmd/...
```

Server options:

```text
-listen=:8080
-kubectl=kubectl
-session-ttl=30m
-max-sessions=100
-toolbox-image=alpine/k8s:1.35.0
```

## License

Apache-2.0
