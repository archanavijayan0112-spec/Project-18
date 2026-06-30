---
# ── Example 1: Bin-Packing Pod ─────────────────────────────────────────────────
# Assigns to the most-utilised node that still fits.
# Use-case: batch jobs, CI runners — maximise hardware efficiency.
apiVersion: v1
kind: Pod
metadata:
  name: batch-job-example
  namespace: default
  annotations:
    scheduler.k8s.custom/hint: bin-packing
spec:
  schedulerName: custom-scheduler
  containers:
    - name: worker
      image: busybox:latest
      command: ["sh", "-c", "echo running && sleep 3600"]
      resources:
        requests:
          cpu: 500m
          memory: 256Mi
        limits:
          cpu: "1"
          memory: 512Mi

---
# ── Example 2: Load-Balanced Pod ───────────────────────────────────────────────
# Placed on the least-utilised node.
# Use-case: long-running microservices that need predictable latency.
apiVersion: v1
kind: Pod
metadata:
  name: api-server-example
  namespace: default
  annotations:
    scheduler.k8s.custom/hint: load-balancing
spec:
  schedulerName: custom-scheduler
  containers:
    - name: api
      image: nginx:latest
      ports:
        - containerPort: 80
      resources:
        requests:
          cpu: 200m
          memory: 128Mi
        limits:
          cpu: 500m
          memory: 256Mi

---
# ── Example 3: Affinity-Aware Pod ─────────────────────────────────────────────
# Co-locates with pods labelled app=cache and spreads across zones.
# Use-case: stateful services that need low-latency access to a sidecar.
apiVersion: v1
kind: Pod
metadata:
  name: stateful-app-example
  namespace: default
  annotations:
    scheduler.k8s.custom/hint: affinity
spec:
  schedulerName: custom-scheduler
  affinity:
    podAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
        - weight: 80
          podAffinityTerm:
            labelSelector:
              matchLabels:
                app: cache
            topologyKey: kubernetes.io/hostname
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchLabels:
              app: stateful-app
          topologyKey: kubernetes.io/hostname
  topologySpreadConstraints:
    - maxSkew: 1
      topologyKey: topology.kubernetes.io/zone
      whenUnsatisfiable: ScheduleAnyway
      labelSelector:
        matchLabels:
          app: stateful-app
  containers:
    - name: app
      image: redis:7-alpine
      resources:
        requests:
          cpu: 250m
          memory: 512Mi
        limits:
          cpu: 500m
          memory: 1Gi

---
# ── Example 4: Hybrid Pod ──────────────────────────────────────────────────────
# No annotation — equal-weight blend of all three policies.
apiVersion: v1
kind: Pod
metadata:
  name: hybrid-workload-example
  namespace: default
spec:
  schedulerName: custom-scheduler
  containers:
    - name: app
      image: python:3.11-slim
      command: ["python", "-c", "import time; time.sleep(86400)"]
      resources:
        requests:
          cpu: 100m
          memory: 64Mi
        limits:
          cpu: 200m
          memory: 128Mi
