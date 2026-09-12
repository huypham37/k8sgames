package game

func foundationChallenges() []Challenge {
	return []Challenge{
		{
			ID: "first-pod", Title: "Your First Pod", Chapter: "Foundations",
			Objective: "Create a running Pod named learner using nginx:1.27-alpine.",
			Hint:      "Run: kubectl run learner --image=nginx:1.27-alpine",
			Manifest:  baseManifest,
			Probes: []Probe{
				probe([]string{"get", "pod", "learner", "-o", "jsonpath={.spec.containers[0].image}"}, "nginx:1.27-alpine", "Pod/learner does not use nginx:1.27-alpine."),
				probe([]string{"get", "pod", "learner", "-o", "jsonpath={.status.phase}"}, "Running", "Pod/learner is not running yet."),
			},
		},
		{
			ID: "scale-up", Title: "Deployments & ReplicaSets", Chapter: "Foundations",
			Objective: "Scale deployment/api from one replica to three.",
			Hint:      "Run: kubectl scale deployment api --replicas=3",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: api}
spec:
  replicas: 1
  selector: {matchLabels: {app: api}}
  template:
    metadata: {labels: {app: api}}
    spec:
      containers:
        - {name: api, image: nginx:1.27-alpine}
`,
			Probes: []Probe{
				probe([]string{"get", "deployment", "api", "-o", "jsonpath={.spec.replicas}"}, "3", "Deployment/api does not have three replicas yet."),
			},
		},
		{
			ID: "scheduling", Title: "Scheduling Constraints", Chapter: "Foundations",
			Objective: "Fix deployment/scheduled-app so both replicas can be scheduled and become available.",
			Hint:      "Inspect its nodeSelector, then remove the impossible training=blocked constraint.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: scheduled-app}
spec:
  replicas: 2
  selector: {matchLabels: {app: scheduled-app}}
  template:
    metadata: {labels: {app: scheduled-app}}
    spec:
      nodeSelector: {training: blocked}
      containers:
        - {name: app, image: nginx:1.27-alpine}
`,
			Probes: []Probe{
				probe([]string{"get", "deployment", "scheduled-app", "-o", "jsonpath={.spec.template.spec.nodeSelector}"}, "", "Deployment/scheduled-app still has a nodeSelector."),
				probe([]string{"get", "deployment", "scheduled-app", "-o", "jsonpath={.status.availableReplicas}"}, "2", "Both replicas are not available yet."),
			},
		},
		{
			ID: "broken-image", Title: "Broken Image", Chapter: "Foundations",
			Objective: "Make both replicas of deployment/web available.",
			Hint:      "Inspect the pods, then set the web container image to a working nginx image.",
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
    spec:
      containers:
        - {name: web, image: nginx:image-does-not-exist}
`,
			Probes: []Probe{
				probe([]string{"get", "deployment", "web", "-o", "jsonpath={.status.availableReplicas}"}, "2", "Both replicas are not available yet."),
			},
		},
		{
			ID: "crash-loop", Title: "CrashLoopBackOff", Chapter: "Foundations",
			Objective: "Remove the crashing command from deployment/payment-service and restore both replicas.",
			Hint:      "Remove /spec/template/spec/containers/0/command with a JSON patch.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: payment-service}
spec:
  replicas: 2
  selector: {matchLabels: {app: payment-service}}
  template:
    metadata: {labels: {app: payment-service}}
    spec:
      containers:
        - name: payment-service
          image: nginx:1.27-alpine
          command: ["sh", "-c", "exit 1"]
`,
			Probes: []Probe{
				probe([]string{"get", "deployment", "payment-service", "-o", "jsonpath={.spec.template.spec.containers[0].command}"}, "", "The crashing command is still configured."),
				probe([]string{"get", "deployment", "payment-service", "-o", "jsonpath={.status.availableReplicas}"}, "2", "Both payment-service replicas are not available yet."),
			},
		},
		{
			ID: "self-healing", Title: "Self-Healing & Availability", Chapter: "Foundations",
			Objective: "Give deployments web, api, and worker at least two replicas each.",
			Hint:      "Scale each Deployment to 2 replicas so one Pod failure does not cause an outage.",
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
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: api}
spec:
  replicas: 1
  selector: {matchLabels: {app: api}}
  template:
    metadata: {labels: {app: api}}
    spec: {containers: [{name: api, image: nginx:1.27-alpine}]}
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
				atLeastProbe([]string{"get", "deployment", "web", "-o", "jsonpath={.spec.replicas}"}, 2, "Deployment/web needs at least two replicas."),
				atLeastProbe([]string{"get", "deployment", "api", "-o", "jsonpath={.spec.replicas}"}, 2, "Deployment/api needs at least two replicas."),
				atLeastProbe([]string{"get", "deployment", "worker", "-o", "jsonpath={.spec.replicas}"}, 2, "Deployment/worker needs at least two replicas."),
			},
		},
	}
}
