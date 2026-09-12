package game

func workloadChallenges() []Challenge {
	return []Challenge{
		{
			ID: "daemonset", Title: "DaemonSets: One Pod Per Node", Chapter: "Workloads",
			Objective: "Create DaemonSet/log-agent with a ready Pod on every eligible node.",
			Hint:      "Apply a DaemonSet named log-agent using busybox:1.36 and command: [sh, -c, sleep infinity].",
			Manifest:  baseManifest,
			Probes: []Probe{
				probe([]string{"get", "daemonset", "log-agent", "-o", "jsonpath={.spec.template.spec.containers[0].image}"}, "busybox:1.36", "DaemonSet/log-agent must use busybox:1.36."),
				nonEmptyProbe([]string{"get", "daemonset", "log-agent", "-o", "jsonpath={.status.numberReady}"}, "DaemonSet/log-agent has no ready Pods yet."),
				probe([]string{"get", "daemonset", "log-agent", "-o", "jsonpath={.status.numberUnavailable}"}, "", "DaemonSet/log-agent still has unavailable Pods."),
			},
		},
		{
			ID: "batch-workloads", Title: "Jobs & CronJobs", Chapter: "Workloads",
			Objective: "Complete Jobs batch-a and batch-b, then create hourly CronJob/cleanup.",
			Hint:      "Use busybox:1.36 with /bin/true for both Jobs and the CronJob.",
			Manifest:  baseManifest,
			Probes: []Probe{
				probe([]string{"get", "job", "batch-a", "-o", "jsonpath={.status.succeeded}"}, "1", "Job/batch-a has not completed."),
				probe([]string{"get", "job", "batch-b", "-o", "jsonpath={.status.succeeded}"}, "1", "Job/batch-b has not completed."),
				probe([]string{"get", "cronjob", "cleanup", "-o", "jsonpath={.spec.schedule}"}, "0 * * * *", "CronJob/cleanup must run hourly."),
			},
		},
		{
			ID: "rolling-rollback", Title: "Rolling Updates & Rollbacks", Chapter: "Workloads",
			Objective: "Rollback deployment/api-server from its broken image and restore three replicas.",
			Hint:      "Inspect rollout history, then run: kubectl rollout undo deployment/api-server",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: api-server}
spec:
  replicas: 3
  revisionHistoryLimit: 3
  selector: {matchLabels: {app: api-server}}
  template:
    metadata: {labels: {app: api-server}}
    spec:
      containers:
        - {name: api, image: nginx:1.27-alpine}
`,
			Setup: [][]string{
				{"set", "image", "deployment/api-server", "api=nginx:image-does-not-exist"},
			},
			Probes: []Probe{
				probe([]string{"get", "deployment", "api-server", "-o", "jsonpath={.spec.template.spec.containers[0].image}"}, "nginx:1.27-alpine", "Deployment/api-server still uses the broken image."),
				probe([]string{"get", "deployment", "api-server", "-o", "jsonpath={.status.availableReplicas}"}, "3", "Deployment/api-server has not recovered all replicas."),
			},
		},
	}
}
