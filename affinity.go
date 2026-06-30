---
# ── Namespace ──────────────────────────────────────────────────────────────────
apiVersion: v1
kind: Namespace
metadata:
  name: custom-scheduler
  labels:
    app.kubernetes.io/managed-by: custom-scheduler

---
# ── ServiceAccount ─────────────────────────────────────────────────────────────
apiVersion: v1
kind: ServiceAccount
metadata:
  name: custom-scheduler
  namespace: custom-scheduler

---
# ── ClusterRole ────────────────────────────────────────────────────────────────
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom-scheduler
rules:
  # Core scheduling permissions
  - apiGroups: [""]
    resources: [pods]
    verbs: [get, list, watch]
  - apiGroups: [""]
    resources: [pods/binding]
    verbs: [create]
  - apiGroups: [""]
    resources: [nodes]
    verbs: [get, list, watch]
  - apiGroups: [""]
    resources: [events]
    verbs: [create, patch, update]
  # For reading resource quotas / limits
  - apiGroups: [""]
    resources: [namespaces, resourcequotas, persistentvolumeclaims]
    verbs: [get, list, watch]
  # Storage
  - apiGroups: [storage.k8s.io]
    resources: [storageclasses, csinodes, csidrivers, csiStorageCapacities]
    verbs: [get, list, watch]
  # For leader election
  - apiGroups: [coordination.k8s.io]
    resources: [leases]
    verbs: [create, get, list, update]

---
# ── ClusterRoleBinding ─────────────────────────────────────────────────────────
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: custom-scheduler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: custom-scheduler
subjects:
  - kind: ServiceAccount
    name: custom-scheduler
    namespace: custom-scheduler

---
# ── ConfigMap ─────────────────────────────────────────────────────────────────
apiVersion: v1
kind: ConfigMap
metadata:
  name: custom-scheduler-config
  namespace: custom-scheduler
data:
  policy: "bin-packing"
  log-level: "info"
  resync-period: "30s"

---
# ── Deployment ─────────────────────────────────────────────────────────────────
apiVersion: apps/v1
kind: Deployment
metadata:
  name: custom-scheduler
  namespace: custom-scheduler
  labels:
    app: custom-scheduler
    version: "1.0.0"
spec:
  replicas: 2           # HA — both instances use leader election
  selector:
    matchLabels:
      app: custom-scheduler
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  template:
    metadata:
      labels:
        app: custom-scheduler
        version: "1.0.0"
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: custom-scheduler
      priorityClassName: system-cluster-critical
      containers:
        - name: scheduler
          image: ghcr.io/your-org/custom-scheduler:latest
          imagePullPolicy: Always
          args:
            - --scheduler-name=custom-scheduler
            - --policy=$(SCHEDULER_POLICY)
            - --log-level=$(LOG_LEVEL)
            - --metrics-addr=:9090
          env:
            - name: SCHEDULER_POLICY
              valueFrom:
                configMapKeyRef:
                  name: custom-scheduler-config
                  key: policy
            - name: LOG_LEVEL
              valueFrom:
                configMapKeyRef:
                  name: custom-scheduler-config
                  key: log-level
          ports:
            - name: metrics
              containerPort: 9090
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: 9090
            initialDelaySeconds: 15
            periodSeconds: 20
          readinessProbe:
            httpGet:
              path: /readyz
              port: 9090
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            runAsNonRoot: true
            capabilities:
              drop: [ALL]
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: custom-scheduler
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: custom-scheduler
                topologyKey: kubernetes.io/hostname

---
# ── Service (metrics scraping) ─────────────────────────────────────────────────
apiVersion: v1
kind: Service
metadata:
  name: custom-scheduler-metrics
  namespace: custom-scheduler
  labels:
    app: custom-scheduler
spec:
  selector:
    app: custom-scheduler
  ports:
    - name: metrics
      port: 9090
      targetPort: 9090
