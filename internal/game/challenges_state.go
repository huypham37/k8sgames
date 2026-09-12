package game

func stateChallenges() []Challenge {
	return []Challenge{
		{
			ID: "configmaps", Title: "ConfigMap Essentials", Chapter: "State & Config",
			Objective: "Create app-config and env-config; mount app-config and load env-config into deployment/app.",
			Hint:      "Use a ConfigMap volume for app-config and envFrom.configMapRef for env-config.",
			Manifest: baseManifest + `
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: app}
spec:
  replicas: 1
  selector: {matchLabels: {app: app}}
  template:
    metadata: {labels: {app: app}}
    spec: {containers: [{name: app, image: nginx:1.27-alpine}]}
`,
			Probes: []Probe{
				nonEmptyProbe([]string{"get", "configmap", "app-config", "-o", "jsonpath={.metadata.name}"}, "ConfigMap/app-config does not exist."),
				nonEmptyProbe([]string{"get", "configmap", "env-config", "-o", "jsonpath={.metadata.name}"}, "ConfigMap/env-config does not exist."),
				probe([]string{"get", "deployment", "app", "-o", "jsonpath={.spec.template.spec.volumes[0].configMap.name}"}, "app-config", "Deployment/app must mount ConfigMap/app-config as a volume."),
				probe([]string{"get", "deployment", "app", "-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts[0].name}"}, "app-config", "Deployment/app must mount the app-config volume in its container."),
				probe([]string{"get", "deployment", "app", "-o", "jsonpath={.spec.template.spec.containers[0].envFrom[0].configMapRef.name}"}, "env-config", "Deployment/app must load ConfigMap/env-config with envFrom."),
			},
		},
		{
			ID: "secrets", Title: "Secret Operations", Chapter: "State & Config",
			Objective: "Move DB_PASSWORD out of db-config into Secret/db-secret, create api-secret, and mount db-secret in deployment/database.",
			Hint:      "Create two generic Secrets, remove DB_PASSWORD from the ConfigMap, then add a Secret volume.",
			Manifest: baseManifest + `
---
apiVersion: v1
kind: ConfigMap
metadata: {name: db-config}
data:
  DB_HOST: database
  DB_PORT: "5432"
  DB_PASSWORD: exposed123
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: database}
spec:
  replicas: 1
  selector: {matchLabels: {app: database}}
  template:
    metadata: {labels: {app: database}}
    spec: {containers: [{name: database, image: nginx:1.27-alpine}]}
`,
			Probes: []Probe{
				probe([]string{"get", "configmap", "db-config", "-o", "jsonpath={.data.DB_PASSWORD}"}, "", "DB_PASSWORD is still exposed in ConfigMap/db-config."),
				probe([]string{"get", "secret", "db-secret", "-o", "jsonpath={.type}"}, "Opaque", "Secret/db-secret does not exist."),
				probe([]string{"get", "secret", "api-secret", "-o", "jsonpath={.type}"}, "Opaque", "Secret/api-secret does not exist."),
				probe([]string{"get", "deployment", "database", "-o", "jsonpath={.spec.template.spec.volumes[0].secret.secretName}"}, "db-secret", "Deployment/database must mount Secret/db-secret."),
				probe([]string{"get", "deployment", "database", "-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts[0].name}"}, "db-secret", "Deployment/database must mount the db-secret volume in its container."),
			},
		},
		{
			ID: "persistent-storage", Title: "Persistent Storage", Chapter: "State & Config",
			Objective: "Create PersistentVolumeClaims/data-a and data-b requesting 1Gi each.",
			Hint:      "Apply two ReadWriteOnce PVCs with resources.requests.storage: 1Gi.",
			Manifest:  baseManifest,
			Probes: []Probe{
				probe([]string{"get", "pvc", "data-a", "-o", "jsonpath={.spec.resources.requests.storage}"}, "1Gi", "PVC/data-a must request 1Gi."),
				probe([]string{"get", "pvc", "data-a", "-o", "jsonpath={.spec.accessModes[0]}"}, "ReadWriteOnce", "PVC/data-a must use ReadWriteOnce."),
				probe([]string{"get", "pvc", "data-b", "-o", "jsonpath={.spec.resources.requests.storage}"}, "1Gi", "PVC/data-b must request 1Gi."),
				probe([]string{"get", "pvc", "data-b", "-o", "jsonpath={.spec.accessModes[0]}"}, "ReadWriteOnce", "PVC/data-b must use ReadWriteOnce."),
			},
		},
		{
			ID: "statefulset", Title: "StatefulSet Database", Chapter: "State & Config",
			Objective: "Create headless Service/database and a three-replica StatefulSet/database with data volumeClaimTemplates.",
			Hint:      "Set serviceName: database, clusterIP: None, replicas: 3, and name the claim template data.",
			Manifest:  baseManifest,
			Probes: []Probe{
				probe([]string{"get", "service", "database", "-o", "jsonpath={.spec.clusterIP}"}, "None", "Service/database must be headless."),
				probe([]string{"get", "statefulset", "database", "-o", "jsonpath={.spec.serviceName}"}, "database", "StatefulSet/database must use Service/database."),
				probe([]string{"get", "statefulset", "database", "-o", "jsonpath={.spec.replicas}"}, "3", "StatefulSet/database must have three replicas."),
				probe([]string{"get", "statefulset", "database", "-o", "jsonpath={.spec.volumeClaimTemplates[0].metadata.name}"}, "data", "StatefulSet/database needs a data volumeClaimTemplate."),
				probe([]string{"get", "statefulset", "database", "-o", "jsonpath={.spec.volumeClaimTemplates[0].spec.accessModes[0]}"}, "ReadWriteOnce", "The data claim template must use ReadWriteOnce."),
				probe([]string{"get", "statefulset", "database", "-o", "jsonpath={.spec.volumeClaimTemplates[0].spec.resources.requests.storage}"}, "1Gi", "The data claim template must request 1Gi."),
			},
		},
	}
}
