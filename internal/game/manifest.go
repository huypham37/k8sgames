package game

const baseManifest = `
apiVersion: v1
kind: ResourceQuota
metadata:
  name: player-quota
spec:
  hard:
    requests.cpu: "8"
    requests.memory: 8Gi
    limits.cpu: "16"
    limits.memory: 16Gi
    pods: "50"
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
  - apiGroups: [""]
    resources: ["pods", "pods/log", "pods/exec", "events", "services", "endpoints", "configmaps", "secrets", "serviceaccounts", "persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "deployments/scale", "replicasets", "replicasets/scale", "statefulsets", "statefulsets/scale", "daemonsets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses", "networkpolicies"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings"]
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
