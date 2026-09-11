package game

import "fmt"

type Probe struct {
	Args    []string
	Equals  string
	Pending string
}

type Challenge struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Objective string  `json:"objective"`
	Hint      string  `json:"hint"`
	Manifest  string  `json:"-"`
	Probes    []Probe `json:"-"`
}

type Catalog struct {
	items []Challenge
	byID  map[string]Challenge
}

func NewCatalog() *Catalog {
	items := []Challenge{
		{
			ID:        "broken-image",
			Title:     "Broken Image",
			Objective: "Make both replicas of deployment/web available.",
			Hint:      "Inspect the pods, then set the web container image to nginx:1.27-alpine.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 2
  selector:
    matchLabels: {app: web}
  template:
    metadata:
      labels: {app: web}
    spec:
      containers:
        - name: web
          image: nginx:image-does-not-exist
`,
			Probes: []Probe{
				{[]string{"get", "deployment", "web", "-o", "jsonpath={.status.availableReplicas}"}, "2", "Both replicas are not available yet."},
			},
		},
		{
			ID:        "scale-up",
			Title:     "Scale Up",
			Objective: "Scale deployment/api from one replica to three.",
			Hint:      "Use kubectl scale deployment api --replicas=3.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  replicas: 1
  selector:
    matchLabels: {app: api}
  template:
    metadata:
      labels: {app: api}
    spec:
      containers:
        - name: api
          image: nginx:1.27-alpine
`,
			Probes: []Probe{
				{[]string{"get", "deployment", "api", "-o", "jsonpath={.spec.replicas}"}, "3", "deployment/api does not have three desired replicas yet."},
			},
		},
		{
			ID:        "service-selector",
			Title:     "Missing Endpoints",
			Objective: "Fix service/backend so it selects the backend pods.",
			Hint:      "Compare pod labels with the Service selector, then patch selector app=backend.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
spec:
  replicas: 2
  selector:
    matchLabels: {app: backend}
  template:
    metadata:
      labels: {app: backend}
    spec:
      containers:
        - name: backend
          image: nginx:1.27-alpine
---
apiVersion: v1
kind: Service
metadata:
  name: backend
spec:
  selector: {app: wrong}
  ports:
    - port: 80
`,
			Probes: []Probe{
				{[]string{"get", "service", "backend", "-o", "jsonpath={.spec.selector.app}"}, "backend", "service/backend still has the wrong selector."},
			},
		},
		{
			ID:        "resource-limits",
			Title:     "Resource Discipline",
			Objective: "Give deployment/worker CPU and memory requests and limits.",
			Hint:      "Use kubectl set resources deployment worker with requests cpu=200m,memory=128Mi and limits cpu=500m,memory=256Mi.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
spec:
  replicas: 1
  selector:
    matchLabels: {app: worker}
  template:
    metadata:
      labels: {app: worker}
    spec:
      containers:
        - name: worker
          image: nginx:1.27-alpine
`,
			Probes: []Probe{
				{[]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.requests.cpu}"}, "200m", "The CPU request must be 200m."},
				{[]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.requests.memory}"}, "128Mi", "The memory request must be 128Mi."},
				{[]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.cpu}"}, "500m", "The CPU limit must be 500m."},
				{[]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.memory}"}, "256Mi", "The memory limit must be 256Mi."},
			},
		},
	}
	byID := make(map[string]Challenge, len(items))
	for _, challenge := range items {
		byID[challenge.ID] = challenge
	}
	return &Catalog{items: items, byID: byID}
}

func (c *Catalog) Find(id string) (Challenge, error) {
	challenge, ok := c.byID[id]
	if !ok {
		return Challenge{}, fmt.Errorf("unknown challenge %q", id)
	}
	return challenge, nil
}

func (c *Catalog) All() []Challenge {
	return append([]Challenge(nil), c.items...)
}

const baseManifest = `
apiVersion: v1
kind: ResourceQuota
metadata:
  name: player-quota
spec:
  hard:
    requests.cpu: "1"
    requests.memory: 1Gi
    limits.cpu: "2"
    limits.memory: 2Gi
    pods: "10"
---
apiVersion: v1
kind: LimitRange
metadata:
  name: player-defaults
spec:
  limits:
    - type: Container
      default: {cpu: 250m, memory: 256Mi}
      defaultRequest: {cpu: 100m, memory: 128Mi}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: player
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: player
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io", "autoscaling"]
    resources: ["pods", "pods/log", "pods/exec", "events", "services", "configmaps", "deployments", "deployments/scale", "replicasets", "replicasets/scale", "statefulsets", "statefulsets/scale", "daemonsets", "jobs", "cronjobs", "ingresses", "networkpolicies", "horizontalpodautoscalers"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: player
subjects:
  - kind: ServiceAccount
    name: player
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: player
---
apiVersion: v1
kind: Pod
metadata:
  name: toolbox
  labels: {app: toolbox}
spec:
  serviceAccountName: player
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
    seccompProfile: {type: RuntimeDefault}
  containers:
    - name: toolbox
      image: {{TOOLBOX_IMAGE}}
      command: ["bash", "-lc"]
      args:
        - |
          set -eu
          mkdir -p "$HOME/.kube"
          kubectl config set-cluster game --server="https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT}" --certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt >/dev/null
          kubectl config set-credentials player --token="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" >/dev/null
          kubectl config set-context game --cluster=game --user=player --namespace="$POD_NAMESPACE" >/dev/null
          kubectl config use-context game >/dev/null
          cat > "$HOME/.bashrc" <<'EOF'
          [[ -r /usr/share/bash-completion/bash_completion ]] && source /usr/share/bash-completion/bash_completion
          if ! declare -F _get_comp_words_by_ref >/dev/null; then
            _get_comp_words_by_ref() {
              while [[ "$1" == -* ]]; do
                [[ "$1" == "-n" ]] && shift
                shift
              done
              local name
              for name in "$@"; do
                case "$name" in
                  cur) printf -v "$name" '%s' "${COMP_WORDS[COMP_CWORD]}" ;;
                  prev) printf -v "$name" '%s' "${COMP_WORDS[COMP_CWORD-1]}" ;;
                  words) eval "$name=(\"\${COMP_WORDS[@]}\")" ;;
                  cword) printf -v "$name" '%s' "$COMP_CWORD" ;;
                esac
              done
            }
          fi
          if ! declare -F _filedir >/dev/null; then
            _filedir() { COMPREPLY=( $(compgen -f -- "$cur") ); }
          fi
          source <(kubectl completion bash)
          alias k=kubectl
          complete -o default -F __start_kubectl k
          hint() { printf '%s\n' "$K8SGAMES_HINT"; }
          objective() { printf '%s\n' "$K8SGAMES_OBJECTIVE"; }
          check() { printf '\036K8SGAMES_CHECK\037'; }
          export HISTFILE="$HOME/.bash_history"
          export PS1="player@${POD_NAMESPACE}:\w$ "
          EOF
          touch /tmp/ready
          exec sleep infinity
      env:
        - name: HOME
          value: /home/player
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef: {fieldPath: metadata.namespace}
      readinessProbe:
        exec: {command: ["test", "-f", "/tmp/ready"]}
      resources:
        requests: {cpu: 50m, memory: 64Mi}
        limits: {cpu: 200m, memory: 256Mi}
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: ["ALL"]}
        readOnlyRootFilesystem: true
      volumeMounts:
        - {name: home, mountPath: /home/player}
        - {name: tmp, mountPath: /tmp}
  volumes:
    - name: home
      emptyDir: {}
    - name: tmp
      emptyDir: {}
`
