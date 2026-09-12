package game

func productionChallenges() []Challenge {
	return []Challenge{
		{
			ID: "resource-limits", Title: "Resource Discipline", Chapter: "Production",
			Objective: "Give deployment/worker CPU and memory requests and limits.",
			Hint:      "Set requests cpu=200m,memory=128Mi and limits cpu=500m,memory=256Mi.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: worker}
spec:
  replicas: 1
  selector: {matchLabels: {app: worker}}
  template:
    metadata: {labels: {app: worker}}
    spec: {containers: [{name: worker, image: nginx:1.27-alpine}]}
`,
			Probes: []Probe{
				probe([]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.requests.cpu}"}, "200m", "The CPU request must be 200m."),
				probe([]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.requests.memory}"}, "128Mi", "The memory request must be 128Mi."),
				probe([]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.cpu}"}, "500m", "The CPU limit must be 500m."),
				probe([]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.memory}"}, "256Mi", "The memory limit must be 256Mi."),
			},
		},
		{
			ID: "production-readiness", Title: "Production Readiness", Chapter: "Production",
			Objective: "Add resources, HTTP liveness/readiness probes, and HPA/web-autoscaler to deployment/web.",
			Hint:      "Use requests cpu=100m,memory=64Mi; limits cpu=250m,memory=128Mi; probe path /; HPA min=2,max=5,CPU=70%.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  replicas: 2
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec: {containers: [{name: web, image: nginx:1.27-alpine}]}
`,
			Probes: []Probe{
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.template.spec.containers[0].resources.requests.cpu}"}, "100m", "Deployment/web needs a 100m CPU request."),
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.template.spec.containers[0].resources.requests.memory}"}, "64Mi", "Deployment/web needs a 64Mi memory request."),
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.cpu}"}, "250m", "Deployment/web needs a 250m CPU limit."),
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.memory}"}, "128Mi", "Deployment/web needs a 128Mi memory limit."),
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.template.spec.containers[0].livenessProbe.httpGet.path}"}, "/", "Deployment/web needs an HTTP liveness probe on /."),
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.template.spec.containers[0].readinessProbe.httpGet.path}"}, "/", "Deployment/web needs an HTTP readiness probe on /."),
				probe([]string{"get", "hpa", "web-autoscaler", "-o", "jsonpath={.spec.minReplicas}"}, "2", "HPA/web-autoscaler must have minReplicas 2."),
				probe([]string{"get", "hpa", "web-autoscaler", "-o", "jsonpath={.spec.maxReplicas}"}, "5", "HPA/web-autoscaler must have maxReplicas 5."),
				probe([]string{"get", "hpa", "web-autoscaler", "-o", "jsonpath={.spec.metrics[0].resource.target.averageUtilization}"}, "70", "HPA/web-autoscaler must target 70% CPU."),
			},
		},
		{
			ID: "rbac-fortress", Title: "RBAC Fortress", Chapter: "Production",
			Objective: "Replace the overprivileged Role/legacy-admin with two least-privilege ServiceAccounts, Roles, and RoleBindings.",
			Hint:      "Create viewer and editor identities; grant viewer get/list and editor get/list/update on ConfigMaps only.",
			Manifest: baseManifest + `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: {name: legacy-admin}
rules:
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
`,
			Probes: []Probe{
				probe([]string{"get", "role", "legacy-admin", "--ignore-not-found=true", "-o", "name"}, "", "Delete the overprivileged Role/legacy-admin."),
				probe([]string{"get", "serviceaccount", "viewer", "-o", "jsonpath={.metadata.name}"}, "viewer", "ServiceAccount/viewer does not exist."),
				probe([]string{"get", "serviceaccount", "editor", "-o", "jsonpath={.metadata.name}"}, "editor", "ServiceAccount/editor does not exist."),
				unorderedProbe([]string{"get", "role", "viewer", "-o", "jsonpath={.rules[*].resources[*]}"}, []string{"configmaps"}, "Role/viewer must target only ConfigMaps."),
				unorderedProbe([]string{"get", "role", "viewer", "-o", "jsonpath={.rules[*].verbs[*]}"}, []string{"get", "list"}, "Role/viewer must grant only get and list."),
				unorderedProbe([]string{"get", "role", "editor", "-o", "jsonpath={.rules[*].resources[*]}"}, []string{"configmaps"}, "Role/editor must target only ConfigMaps."),
				unorderedProbe([]string{"get", "role", "editor", "-o", "jsonpath={.rules[*].verbs[*]}"}, []string{"get", "list", "update"}, "Role/editor must grant only get, list, and update."),
				probe([]string{"get", "rolebinding", "viewer", "-o", "jsonpath={.roleRef.name}"}, "viewer", "RoleBinding/viewer must bind Role/viewer."),
				probe([]string{"get", "rolebinding", "viewer", "-o", "jsonpath={.subjects[0].name}"}, "viewer", "RoleBinding/viewer must bind ServiceAccount/viewer."),
				probe([]string{"get", "rolebinding", "editor", "-o", "jsonpath={.roleRef.name}"}, "editor", "RoleBinding/editor must bind Role/editor."),
				probe([]string{"get", "rolebinding", "editor", "-o", "jsonpath={.subjects[0].name}"}, "editor", "RoleBinding/editor must bind ServiceAccount/editor."),
			},
		},
		{
			ID: "outage-resilience", Title: "Outage Resilience", Chapter: "Production",
			Objective: "Scale deployment/web to four replicas and protect it with PDB/web-budget minAvailable 3.",
			Hint:      "Scale the Deployment, then create a policy/v1 PodDisruptionBudget selecting app=web.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  replicas: 1
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec: {containers: [{name: web, image: nginx:1.27-alpine}]}
`,
			Probes: []Probe{
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.replicas}"}, "4", "Deployment/web must have four replicas."),
				probe([]string{"get", "pdb", "web-budget", "-o", "jsonpath={.spec.minAvailable}"}, "3", "PDB/web-budget must keep three Pods available."),
				probe([]string{"get", "pdb", "web-budget", "-o", "jsonpath={.spec.selector.matchLabels.app}"}, "web", "PDB/web-budget must select app=web."),
			},
		},
		threeTierChallenge(),
		blackFridayChallenge(),
		fullProductionChallenge(),
	}
}

func threeTierChallenge() Challenge {
	return Challenge{
		ID: "three-tier", Title: "Three-Tier Web App", Chapter: "Production",
		Objective: "Create frontend, backend, and database Deployments and matching Services.",
		Hint:      "Name each Deployment and Service after its app and use matching app labels/selectors.",
		Manifest:  baseManifest,
		Probes: []Probe{
			probe([]string{"get", "deployment", "frontend", "-o", "jsonpath={.spec.template.metadata.labels.app}"}, "frontend", "Deployment/frontend needs label app=frontend."),
			probe([]string{"get", "deployment", "backend", "-o", "jsonpath={.spec.template.metadata.labels.app}"}, "backend", "Deployment/backend needs label app=backend."),
			probe([]string{"get", "deployment", "database", "-o", "jsonpath={.spec.template.metadata.labels.app}"}, "database", "Deployment/database needs label app=database."),
			probe([]string{"get", "service", "frontend", "-o", "jsonpath={.spec.selector.app}"}, "frontend", "Service/frontend must select app=frontend."),
			probe([]string{"get", "service", "backend", "-o", "jsonpath={.spec.selector.app}"}, "backend", "Service/backend must select app=backend."),
			probe([]string{"get", "service", "database", "-o", "jsonpath={.spec.selector.app}"}, "database", "Service/database must select app=database."),
		},
	}
}

func blackFridayChallenge() Challenge {
	return Challenge{
		ID: "black-friday", Title: "Black Friday Scaling", Chapter: "Production",
		Objective: "Scale web to 10 and api to 8 replicas; create web-autoscaler and api-autoscaler HPAs.",
		Hint:      "Use kubectl scale, then kubectl autoscale with min 2, max 10, CPU 70%.",
		Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  replicas: 2
  selector: {matchLabels: {app: web}}
  template:
    metadata: {labels: {app: web}}
    spec: {containers: [{name: web, image: nginx:1.27-alpine}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: api}
spec:
  replicas: 2
  selector: {matchLabels: {app: api}}
  template:
    metadata: {labels: {app: api}}
    spec: {containers: [{name: api, image: nginx:1.27-alpine}]}
`,
		Probes: []Probe{
			probe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.replicas}"}, "10", "Deployment/web must have 10 replicas."),
			probe([]string{"get", "deployment", "api", "-o", "jsonpath={.spec.replicas}"}, "8", "Deployment/api must have 8 replicas."),
			probe([]string{"get", "hpa", "web-autoscaler", "-o", "jsonpath={.spec.scaleTargetRef.name}"}, "web", "HPA/web-autoscaler must target deployment/web."),
			probe([]string{"get", "hpa", "web-autoscaler", "-o", "jsonpath={.spec.minReplicas}"}, "2", "HPA/web-autoscaler must have minReplicas 2."),
			probe([]string{"get", "hpa", "web-autoscaler", "-o", "jsonpath={.spec.maxReplicas}"}, "10", "HPA/web-autoscaler must have maxReplicas 10."),
			probe([]string{"get", "hpa", "web-autoscaler", "-o", "jsonpath={.spec.metrics[0].resource.target.averageUtilization}"}, "70", "HPA/web-autoscaler must target 70% CPU."),
			probe([]string{"get", "hpa", "api-autoscaler", "-o", "jsonpath={.spec.scaleTargetRef.name}"}, "api", "HPA/api-autoscaler must target deployment/api."),
			probe([]string{"get", "hpa", "api-autoscaler", "-o", "jsonpath={.spec.minReplicas}"}, "2", "HPA/api-autoscaler must have minReplicas 2."),
			probe([]string{"get", "hpa", "api-autoscaler", "-o", "jsonpath={.spec.maxReplicas}"}, "10", "HPA/api-autoscaler must have maxReplicas 10."),
			probe([]string{"get", "hpa", "api-autoscaler", "-o", "jsonpath={.spec.metrics[0].resource.target.averageUtilization}"}, "70", "HPA/api-autoscaler must target 70% CPU."),
		},
	}
}

func fullProductionChallenge() Challenge {
	return Challenge{
		ID: "full-production", Title: "Full Production Readiness", Chapter: "Production",
		Objective: "Build a three-tier stack with Services, an Ingress, two NetworkPolicies, a Secret, HPA, and health probes.",
		Hint:      "Use names frontend, backend, database; Services with matching selectors; Ingress/prod; Secret/db-credentials; HPA/frontend-autoscaler.",
		Manifest:  baseManifest,
		Probes: []Probe{
			probe([]string{"get", "deployment", "frontend", "-o", "jsonpath={.spec.template.spec.containers[0].livenessProbe.httpGet.path}"}, "/", "Deployment/frontend needs an HTTP liveness probe on /."),
			probe([]string{"get", "deployment", "frontend", "-o", "jsonpath={.spec.template.spec.containers[0].readinessProbe.httpGet.path}"}, "/", "Deployment/frontend needs an HTTP readiness probe on /."),
			probe([]string{"get", "deployment", "backend", "-o", "jsonpath={.spec.template.metadata.labels.app}"}, "backend", "Deployment/backend needs label app=backend."),
			probe([]string{"get", "deployment", "database", "-o", "jsonpath={.spec.template.metadata.labels.app}"}, "database", "Deployment/database needs label app=database."),
			probe([]string{"get", "service", "frontend", "-o", "jsonpath={.spec.selector.app}"}, "frontend", "Service/frontend must select app=frontend."),
			probe([]string{"get", "service", "backend", "-o", "jsonpath={.spec.selector.app}"}, "backend", "Service/backend must select app=backend."),
			probe([]string{"get", "service", "database", "-o", "jsonpath={.spec.selector.app}"}, "database", "Service/database must select app=database."),
			probe([]string{"get", "secret", "db-credentials", "-o", "jsonpath={.type}"}, "Opaque", "Secret/db-credentials does not exist."),
			probe([]string{"get", "ingress", "prod", "-o", "jsonpath={.spec.rules[0].http.paths[0].path}"}, "/", "Ingress/prod must route path /."),
			probe([]string{"get", "ingress", "prod", "-o", "jsonpath={.spec.rules[0].http.paths[0].backend.service.name}"}, "frontend", "Ingress/prod must route to Service/frontend."),
			probe([]string{"get", "networkpolicy", "default-deny", "-o", "jsonpath={.spec.policyTypes[0]}"}, "Ingress", "NetworkPolicy/default-deny must restrict ingress."),
			probe([]string{"get", "networkpolicy", "default-deny", "-o", "jsonpath={.spec.ingress}"}, "", "NetworkPolicy/default-deny must not allow ingress."),
			probe([]string{"get", "networkpolicy", "allow-backend", "-o", "jsonpath={.spec.podSelector.matchLabels.app}"}, "database", "NetworkPolicy/allow-backend must select database Pods."),
			probe([]string{"get", "networkpolicy", "allow-backend", "-o", "jsonpath={.spec.ingress[0].from[0].podSelector.matchLabels.app}"}, "backend", "NetworkPolicy/allow-backend must allow backend Pods."),
			probe([]string{"get", "hpa", "frontend-autoscaler", "-o", "jsonpath={.spec.scaleTargetRef.name}"}, "frontend", "HPA/frontend-autoscaler must target deployment/frontend."),
		},
	}
}
