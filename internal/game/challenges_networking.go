package game

func networkingChallenges() []Challenge {
	return []Challenge{
		{
			ID: "service-discovery", Title: "Services & Service Discovery", Chapter: "Networking",
			Objective: "Expose backend with ClusterIP Service/backend and frontend with NodePort Service/frontend.",
			Hint:      "Use selectors app=backend and app=frontend; expose container port 80.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend}
spec:
  replicas: 2
  selector: {matchLabels: {app: frontend}}
  template:
    metadata: {labels: {app: frontend}}
    spec: {containers: [{name: frontend, image: nginx:1.27-alpine}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: backend}
spec:
  replicas: 2
  selector: {matchLabels: {app: backend}}
  template:
    metadata: {labels: {app: backend}}
    spec: {containers: [{name: backend, image: nginx:1.27-alpine}]}
`,
			Probes: []Probe{
				probe([]string{"get", "service", "backend", "-o", "jsonpath={.spec.selector.app}"}, "backend", "Service/backend must select app=backend."),
				probe([]string{"get", "service", "backend", "-o", "jsonpath={.spec.type}"}, "ClusterIP", "Service/backend must be a ClusterIP Service."),
				probe([]string{"get", "service", "backend", "-o", "jsonpath={.spec.ports[0].port}"}, "80", "Service/backend must expose port 80."),
				probe([]string{"get", "service", "frontend", "-o", "jsonpath={.spec.selector.app}"}, "frontend", "Service/frontend must select app=frontend."),
				probe([]string{"get", "service", "frontend", "-o", "jsonpath={.spec.type}"}, "NodePort", "Service/frontend must be a NodePort Service."),
				probe([]string{"get", "service", "frontend", "-o", "jsonpath={.spec.ports[0].port}"}, "80", "Service/frontend must expose port 80."),
				nonEmptyProbe([]string{"get", "service", "frontend", "-o", "jsonpath={.spec.ports[0].nodePort}"}, "Service/frontend does not have a NodePort yet."),
			},
		},
		{
			ID: "service-selector", Title: "Missing Endpoints", Chapter: "Networking",
			Objective: "Fix service/backend so it selects the backend Pods.",
			Hint:      "Compare Pod labels with the Service selector, then patch selector app=backend.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: backend}
spec:
  replicas: 2
  selector: {matchLabels: {app: backend}}
  template:
    metadata: {labels: {app: backend}}
    spec: {containers: [{name: backend, image: nginx:1.27-alpine}]}
---
apiVersion: v1
kind: Service
metadata: {name: backend}
spec:
  selector: {app: wrong}
  ports: [{port: 80}]
`,
			Probes: []Probe{
				probe([]string{"get", "service", "backend", "-o", "jsonpath={.spec.selector.app}"}, "backend", "Service/backend still has the wrong selector."),
				nonEmptyProbe([]string{"get", "endpoints", "backend", "-o", "jsonpath={.subsets[0].addresses[0].ip}"}, "Service/backend still has no ready endpoints."),
			},
		},
		{
			ID: "ingress-tls", Title: "Ingress with TLS", Chapter: "Networking",
			Objective: "Create Secret/web-tls and Ingress/edge routing /app to app-svc and /api to api-svc with TLS.",
			Hint:      "Use networking.k8s.io/v1, pathType Prefix, and host games.local.",
			Manifest: baseManifest + `
---
apiVersion: v1
kind: Service
metadata: {name: app-svc}
spec:
  selector: {app: app}
  ports: [{port: 80}]
---
apiVersion: v1
kind: Service
metadata: {name: api-svc}
spec:
  selector: {app: api}
  ports: [{port: 3000, targetPort: 80}]
`,
			Probes: []Probe{
				probe([]string{"get", "secret", "web-tls", "-o", "jsonpath={.type}"}, "kubernetes.io/tls", "Secret/web-tls must have type kubernetes.io/tls."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.rules[0].host}"}, "games.local", "Ingress/edge must use host games.local."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.rules[0].http.paths[0].path}"}, "/app", "The first Ingress path must be /app."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.rules[0].http.paths[0].pathType}"}, "Prefix", "Path /app must use pathType Prefix."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.rules[0].http.paths[0].backend.service.name}"}, "app-svc", "Path /app must route to app-svc."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.rules[0].http.paths[1].path}"}, "/api", "The second Ingress path must be /api."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.rules[0].http.paths[1].pathType}"}, "Prefix", "Path /api must use pathType Prefix."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.rules[0].http.paths[1].backend.service.name}"}, "api-svc", "Path /api must route to api-svc."),
				probe([]string{"get", "ingress", "edge", "-o", "jsonpath={.spec.tls[0].secretName}"}, "web-tls", "Ingress/edge must use Secret/web-tls."),
			},
		},
		{
			ID: "network-segmentation", Title: "NetworkPolicies: Zero Trust", Chapter: "Networking",
			Objective: "Create database-deny-all, database-allow-backend, and frontend-egress NetworkPolicies.",
			Hint:      "Select Pods by tier labels. Deny database ingress by default, then allow only tier=backend.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: frontend}
spec:
  replicas: 1
  selector: {matchLabels: {tier: frontend}}
  template:
    metadata: {labels: {tier: frontend}}
    spec: {containers: [{name: app, image: nginx:1.27-alpine}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: backend}
spec:
  replicas: 1
  selector: {matchLabels: {tier: backend}}
  template:
    metadata: {labels: {tier: backend}}
    spec: {containers: [{name: app, image: nginx:1.27-alpine}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: database}
spec:
  replicas: 1
  selector: {matchLabels: {tier: database}}
  template:
    metadata: {labels: {tier: database}}
    spec: {containers: [{name: app, image: nginx:1.27-alpine}]}
`,
			Probes: []Probe{
				probe([]string{"get", "networkpolicy", "database-deny-all", "-o", "jsonpath={.spec.podSelector.matchLabels.tier}"}, "database", "Policy/database-deny-all must select tier=database."),
				probe([]string{"get", "networkpolicy", "database-deny-all", "-o", "jsonpath={.spec.policyTypes[0]}"}, "Ingress", "Policy/database-deny-all must restrict ingress."),
				probe([]string{"get", "networkpolicy", "database-deny-all", "-o", "jsonpath={.spec.ingress}"}, "", "Policy/database-deny-all must not allow any ingress."),
				probe([]string{"get", "networkpolicy", "database-allow-backend", "-o", "jsonpath={.spec.ingress[0].from[0].podSelector.matchLabels.tier}"}, "backend", "Policy/database-allow-backend must allow tier=backend."),
				probe([]string{"get", "networkpolicy", "frontend-egress", "-o", "jsonpath={.spec.podSelector.matchLabels.tier}"}, "frontend", "Policy/frontend-egress must select tier=frontend."),
				probe([]string{"get", "networkpolicy", "frontend-egress", "-o", "jsonpath={.spec.policyTypes[0]}"}, "Egress", "Policy/frontend-egress must restrict egress."),
			},
		},
		{
			ID: "dns-debugging", Title: "DNS Debugging", Chapter: "Networking",
			Objective: "Create NetworkPolicy/allow-dns permitting UDP port 53 egress from app Pods.",
			Hint:      "A deny-all egress policy already selects app=client; add an allow policy for UDP/53.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: client}
spec:
  replicas: 1
  selector: {matchLabels: {app: client}}
  template:
    metadata: {labels: {app: client}}
    spec: {containers: [{name: client, image: busybox:1.36, command: ["sh", "-c", "sleep infinity"]}]}
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: {name: strict-deny}
spec:
  podSelector: {matchLabels: {app: client}}
  policyTypes: [Egress]
  egress: []
`,
			Probes: []Probe{
				probe([]string{"get", "networkpolicy", "allow-dns", "-o", "jsonpath={.spec.podSelector.matchLabels.app}"}, "client", "Policy/allow-dns must select app=client."),
				probe([]string{"get", "networkpolicy", "allow-dns", "-o", "jsonpath={.spec.egress[0].ports[0].protocol}"}, "UDP", "Policy/allow-dns must allow UDP."),
				probe([]string{"get", "networkpolicy", "allow-dns", "-o", "jsonpath={.spec.egress[0].ports[0].port}"}, "53", "Policy/allow-dns must allow port 53."),
			},
		},
	}
}
