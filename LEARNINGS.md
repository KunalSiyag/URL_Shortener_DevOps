# 📚 Learning Journey - URL Shortener DevOps Project

This document captures everything I learned while building a production-ready URL shortener with full DevOps monitoring, CI/CD, and containerization. It includes every error encountered, the debugging process, and the solutions that worked.

---

## Table of Contents

1. [Docker & Container Issues](#-docker--container-issues)
2. [Prometheus & Monitoring](#-prometheus--monitoring)
3. [Grafana Dashboard Creation](#-grafana-dashboard-creation)
4. [Go Application Development](#-go-application-development)
5. [CORS & Web Security](#-cors--web-security)
6. [Docker Compose Deep Dive](#-docker-compose-deep-dive)
7. [CI/CD with GitHub Actions](#-cicd-with-github-actions)
8. [Kubernetes Concepts](#-kubernetes-concepts)
9. [Key Takeaways](#-key-takeaways)
10. [Resources](#-resources)

---

## 🐳 Docker & Container Issues

### 1. Docker Permission Denied

**Error:**
```
permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock
```

**What Happened:** When I first ran `docker compose up`, the system refused to connect to Docker.

**Root Cause:** The Docker daemon runs as root and creates a Unix socket owned by root. By default, only root or users in the `docker` group can access it.

**Solution:**
```bash
# Quick fix - use sudo
sudo docker compose up -d

# Permanent fix - add user to docker group
sudo usermod -aG docker $USER

# After adding to group, either logout/login or run:
newgrp docker
```

**Security Note:** Adding a user to the docker group gives them root-equivalent privileges. In production, use rootless Docker or proper access controls.

**What I Learned:**
- Docker's privileged architecture requires careful permission management
- The `docker` group is powerful - treat it like sudo access
- In Kubernetes, containers run as non-root by default (more secure)

---

### 2. Port Already Allocated

**Error:**
```
Bind for 0.0.0.0:6379 failed: port is already allocated
```

**What Happened:** Tried to start Redis, but something was already using port 6379.

**Debugging Process:**
```bash
# Step 1: Find what's using the port
lsof -i :6379
# Output showed another Redis container

# Step 2: Check Docker containers
sudo docker ps | grep 6379
# Found: redis-local container from earlier testing

# Step 3: Check all containers (including stopped)
sudo docker ps -a | grep redis
```

**Solution:**
```bash
# Stop the conflicting container
sudo docker stop redis-local

# Or remove it entirely
sudo docker rm redis-local

# For a clean slate, stop all project containers
sudo docker compose down
```

**Prevention Tips:**
- Always run `docker compose down` when done working
- Use unique port mappings in development
- Check `docker ps` before starting services

---

### 3. Address Already in Use (Go Server)

**Error:**
```
2026/01/18 16:42:55 Starting server on :8080
2026/01/18 16:42:55 listen: listen tcp :8080: bind: address already in use
exit status 1
```

**What Happened:** Tried to run Go application locally while the Docker container was already running on port 8080.

**Key Insight:** You can't have two processes listening on the same port. The Docker container had already bound to 8080.

**Debugging:**
```bash
# Check what's on port 8080
lsof -i :8080
# Showed Docker container

# Or check Docker directly
sudo docker ps --format "table {{.Names}}\t{{.Ports}}" | grep 8080
```

**Solution Options:**
1. Stop the Docker container: `sudo docker compose stop app`
2. Use a different port locally: `PORT=8081 go run ./cmd/server`
3. Use the container instead of running locally

**What I Learned:**
- Port conflicts are common in development
- Keep track of what's running in Docker vs locally
- Use environment variables to make ports configurable

---

## 📊 Prometheus & Monitoring

### 4. Range Vector vs Instant Vector in Grafana

**Error:** Query worked in Prometheus but showed nothing in Grafana:
```promql
promhttp_metric_handler_requests_total[1m]
```

**What Happened:** The query worked fine in Prometheus's expression browser, but Grafana refused to display it with an error about range vectors.

**The Fundamental Concept:**

| Vector Type | Description | Example | Use Case |
|-------------|-------------|---------|----------|
| **Instant Vector** | Single value per series at one point in time | `http_requests_total` | Direct visualization |
| **Range Vector** | Multiple values over a time range | `http_requests_total[5m]` | Input to functions |

**Why This Matters:**
- Grafana panels need **one value per series per timestamp** to draw a graph
- A range vector contains **multiple values per series** (every scrape in the time range)
- Range vectors are inputs to functions like `rate()`, `avg_over_time()`, etc.

**Solution - Use rate():**
```promql
# ❌ Range vector - can't be graphed directly
promhttp_metric_handler_requests_total[1m]

# ✅ Instant vector - perfect for graphs
rate(promhttp_metric_handler_requests_total[1m])
```

**How rate() Works:**
- Takes a range vector as input
- Calculates the per-second average rate of increase
- Returns an instant vector

**Visual Explanation:**
```
Raw Counter Values:     100, 105, 110, 115, 120 (over 1 minute)
rate() calculates:      (120 - 100) / 60 seconds = 0.33 req/sec
Output:                 Single value: 0.33
```

---

### 5. Understanding PromQL Functions in Depth

**The Most Important Functions:**

#### rate() - For Counters
```promql
# Per-second rate of HTTP requests
rate(http_requests_total[5m])
```
- Use with counters (values that only increase)
- Time range should be at least 4x scrape interval
- Handles counter resets automatically

#### sum() - Aggregation
```promql
# Total requests across all instances
sum(rate(http_requests_total[5m]))

# Requests grouped by status code
sum by (status) (rate(http_requests_total[5m]))
```

#### histogram_quantile() - Percentiles
```promql
# 95th percentile latency
histogram_quantile(0.95, 
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
)
```
- The `le` (less than or equal) label is required
- Returns the bucket boundary for the given quantile
- More accurate with more/smaller buckets

#### increase() vs rate()
```promql
# Total increase in last hour (absolute number)
increase(http_requests_total[1h])

# Per-second rate (normalized)
rate(http_requests_total[1h])
```

**Complete Query Examples I Used:**

```promql
# 1. Request rate per second
rate(promhttp_metric_handler_requests_total[1m])

# 2. 95th percentile latency
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))

# 3. Average response time
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])

# 4. Error rate (non-200 responses)
sum(rate(prometheus_http_requests_total{code!="200"}[5m]))

# 5. Success rate percentage
sum(rate(http_requests_total{status="200"}[5m])) / sum(rate(http_requests_total[5m])) * 100

# 6. Requests by endpoint
sum by (path) (rate(http_request_duration_seconds_count[5m]))
```

---

### 6. Prometheus Scraping Configuration

**File:** `prometheus.yml`
```yaml
global:
  scrape_interval: 15s      # How often to scrape
  evaluation_interval: 15s  # How often to evaluate rules

scrape_configs:
  - job_name: 'url-shortener'
    metrics_path: /metrics   # Endpoint to scrape
    static_configs:
      - targets: ['app:8080']  # Container name:port
```

**Key Learnings:**
- Use **container names** in Docker networking, not localhost
- Default scrape interval is 1 minute, but 15s gives better resolution
- `metrics_path` defaults to `/metrics` but can be customized
- Labels from `job_name` are added to all scraped metrics

---

## 🎨 Grafana Dashboard Creation

### 7. Manual Dashboard Creation Process

**Step-by-Step Guide I Followed:**

1. **Access Grafana:** http://localhost:3000
2. **Login:** admin/admin (skip password change for dev)
3. **Create Dashboard:** Dashboards → New Dashboard
4. **Add Panel:** Click "Add visualization"
5. **Select Data Source:** Prometheus
6. **Enter Query:** Use PromQL (with rate() for counters!)
7. **Choose Visualization:** Time series, Gauge, Stat, etc.
8. **Customize:** Titles, legends, thresholds, colors
9. **Save:** Give it a name

**Panel Types I Used:**

| Panel Type | Best For | Example |
|------------|----------|---------|
| Time Series | Trends over time | Request rate |
| Gauge | Current value with thresholds | Error percentage |
| Stat | Single important number | Total requests |
| Bar Gauge | Comparing values | Latency by endpoint |
| Table | Detailed data | Top endpoints |

---

### 8. Auto-Provisioning Datasources

**Problem:** Had to manually add Prometheus datasource every time Grafana container was recreated.

**Solution:** Provisioning files!

**File:** `grafana/provisioning/datasources/prometheus.yml`
```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy        # Grafana backend proxies requests
    url: http://prometheus:9090
    isDefault: true
    editable: false      # Prevent accidental changes
```

**Docker Compose Mount:**
```yaml
grafana:
  volumes:
    - ./grafana/provisioning/datasources:/etc/grafana/provisioning/datasources
```

**What I Learned:**
- Provisioning = Infrastructure as Code for Grafana
- Changes to provisioning files require container restart
- Can also provision dashboards, alert rules, and more
- `access: proxy` means Grafana server makes requests (more secure)

---

## 🔧 Go Application Development

### 9. Prometheus Metrics Implementation

**Three Types of Metrics:**

#### Counter - Things that only go up
```go
var requestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests",
    },
    []string{"method", "path", "status"},  // Labels
)

// Usage
requestsTotal.WithLabelValues("GET", "/shorten", "200").Inc()
```

#### Histogram - Distribution of values
```go
var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request latency in seconds",
        Buckets: prometheus.DefBuckets,  // Default: .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10
    },
    []string{"method", "path"},
)

// Usage - measure duration
start := time.Now()
// ... handle request ...
duration := time.Since(start).Seconds()
requestDuration.WithLabelValues("POST", "/shorten").Observe(duration)
```

#### Gauge - Values that go up and down
```go
var activeConnections = prometheus.NewGauge(
    prometheus.GaugeOpts{
        Name: "active_connections",
        Help: "Number of active connections",
    },
)

// Usage
activeConnections.Inc()  // Connection opened
activeConnections.Dec()  // Connection closed
activeConnections.Set(42)  // Set absolute value
```

**Registration:**
```go
func init() {
    prometheus.MustRegister(requestsTotal)
    prometheus.MustRegister(requestDuration)
    prometheus.MustRegister(activeConnections)
}
```

---

### 10. Middleware Pattern for Metrics

**Why Middleware?**
- Keeps handler code clean
- Applies consistently to all endpoints
- Single place to modify metric collection

**Implementation:**
```go
func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Call the next handler
        next.ServeHTTP(w, r)
        
        // Record metrics after request completes
        duration := time.Since(start).Seconds()
        HttpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
    })
}
```

**Usage:**
```go
handler := MetricsMiddleware(myRouter)
http.ListenAndServe(":8080", handler)
```

---

### 11. Graceful Shutdown Pattern

**Why It Matters:**
- Kubernetes sends SIGTERM before killing pods
- In-flight requests should complete
- Database connections should close cleanly
- Metrics should be flushed

**Implementation:**
```go
func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: router,
    }

    // Start server in goroutine
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("Shutting down gracefully...")

    // Give outstanding requests 5 seconds to complete
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Forced shutdown: %v", err)
    }

    log.Println("Server stopped")
}
```

**What Happens:**
1. Signal received (Ctrl+C or SIGTERM)
2. Server stops accepting new connections
3. Wait up to 5 seconds for active requests to complete
4. Force close if timeout exceeded

---

## 🌐 CORS & Web Security

### 12. CORS Error with Local File

**Error in Browser Console:**
```
Access to fetch at 'http://localhost:8080/shorten' from origin 'null' 
has been blocked by CORS policy
```

**What Happened:** Opened `ui/index.html` as a local file (`file://`), and the browser blocked API requests.

**Why CORS Exists:**
- Browsers enforce Same-Origin Policy for security
- `file://` has origin `null` - doesn't match `http://localhost:8080`
- Without CORS, malicious websites could steal data from other sites

**Solution - Add CORS Middleware:**
```go
func CORSMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Allow requests from any origin
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

        // Handle preflight requests
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Important Headers:**
| Header | Purpose |
|--------|---------|
| `Access-Control-Allow-Origin` | Which origins can access (use specific origin in production!) |
| `Access-Control-Allow-Methods` | Allowed HTTP methods |
| `Access-Control-Allow-Headers` | Allowed request headers |
| `Access-Control-Allow-Credentials` | Allow cookies/auth headers |

**Preflight Requests:**
- Browser sends OPTIONS request first for "non-simple" requests
- Server must respond with CORS headers
- Only then does browser send actual request

**Production Warning:** Never use `*` for Allow-Origin in production with credentials!

---

## 🐋 Docker Compose Deep Dive

### 13. Service Dependencies with Health Checks

**Problem:** App started before Redis was ready, causing connection errors.

**Naive Solution (doesn't work well):**
```yaml
services:
  app:
    depends_on:
      - redis  # Only waits for container to START, not be READY
```

**Better Solution - Health Checks:**
```yaml
services:
  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
    
  app:
    depends_on:
      redis:
        condition: service_healthy  # Wait for health check to pass
```

**Health Check Options:**
- `test`: Command to run (exit code 0 = healthy)
- `interval`: How often to check
- `timeout`: How long to wait for response
- `retries`: Failures before marking unhealthy
- `start_period`: Grace period at startup

---

### 14. Docker Networking

**Key Insight:** Containers on the same network use container names as hostnames.

```yaml
services:
  app:
    environment:
      - REDIS_ADDR=redis:6379    # NOT localhost!
      
  redis:
    # Container name is "redis" by default (service name)
    
networks:
  default:
    driver: bridge
```

**Common Mistake:**
```go
// ❌ Won't work in Docker
redis.NewClient(&redis.Options{Addr: "localhost:6379"})

// ✅ Correct for Docker
redis.NewClient(&redis.Options{Addr: "redis:6379"})
```

**DNS Resolution:**
- Docker's internal DNS resolves container names
- Only works within the same Docker network
- Use `docker network ls` and `docker network inspect` to debug

---

## 🔄 CI/CD with GitHub Actions

### 15. Workflow Structure

**File:** `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: ["main"]
  pull_request:
    branches: ["main"]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
      
      - name: Install dependencies
        run: go mod download
      
      - name: Run tests
        run: go test ./...
      
      - name: Build
        run: go build -o url-shortener ./cmd/server

  docker:
    runs-on: ubuntu-latest
    needs: build  # Run after build succeeds
    if: github.event_name == 'push'  # Only on push, not PR
    steps:
      # ... build and push Docker image
```

**Key Concepts:**

| Concept | Description |
|---------|-------------|
| `on` triggers | When to run (push, PR, schedule, etc.) |
| `jobs` | Independent units of work |
| `steps` | Sequential tasks within a job |
| `needs` | Job dependencies |
| `if` | Conditional execution |

---

### 16. Docker Image Publishing to GHCR

**GitHub Container Registry (GHCR):**
- Free container registry from GitHub
- Integrated with GitHub permissions
- Images at `ghcr.io/username/repo`

**Workflow Steps:**
```yaml
- name: Log in to GHCR
  uses: docker/login-action@v3
  with:
    registry: ghcr.io
    username: ${{ github.actor }}
    password: ${{ secrets.GITHUB_TOKEN }}

- name: Build and push
  uses: docker/build-push-action@v5
  with:
    context: .
    push: true
    tags: ghcr.io/${{ github.repository }}:latest
    cache-from: type=gha
    cache-to: type=gha,mode=max
```

**What I Learned:**
- `GITHUB_TOKEN` is automatically provided - no secrets needed
- Docker layer caching (`type=gha`) speeds up builds
- Use semantic versioning tags in production

---

## ☸️ Kubernetes Concepts

### 17. Deployment vs Service vs Ingress

| Resource | Purpose | Example |
|----------|---------|---------|
| **Deployment** | Manages pods (replicas, updates) | Run 3 copies of my app |
| **Service** | Internal load balancer + DNS | `app-service:80` routes to pods |
| **Ingress** | External HTTP routing | `myapp.com/api` → service |
| **ConfigMap** | Configuration data | Prometheus config |

**How They Connect:**
```
Internet → Ingress → Service → Deployment → Pods
                        ↑           ↑
                      selector    matchLabels
```

---

### 18. ConfigMaps for Configuration

**Problem:** Don't want to rebuild image for config changes.

**Solution - ConfigMap:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
      - job_name: 'url-shortener'
        static_configs:
          - targets: ['url-shortener:80']
```

**Mount in Deployment:**
```yaml
volumes:
  - name: config
    configMap:
      name: prometheus-config
containers:
  - volumeMounts:
    - name: config
      mountPath: /etc/prometheus
```

---

## 📈 Key Takeaways

### Technical Learnings

1. **Always use `rate()` for counters in Grafana** - Raw counters show cumulative values, not what you want

2. **Docker networking uses container names** - Not localhost, not IP addresses

3. **Health checks > depends_on** - Ensure dependencies are ready, not just started

4. **CORS is browser-enforced** - The server just sets headers, browser decides

5. **Graceful shutdown is essential** - Handle SIGTERM for clean Kubernetes deployments

6. **Labels make metrics powerful** - Add dimensions like method, path, status for filtering

7. **Provisioning > manual config** - Infrastructure as Code for Grafana, Prometheus

8. **CI/CD catches issues early** - Tests run on every push, build failures are visible

### Debugging Process

1. **Read the error message carefully** - It usually tells you exactly what's wrong
2. **Check running processes** - `docker ps`, `lsof -i :PORT`
3. **Check logs** - `docker compose logs -f service_name`
4. **Isolate the problem** - Test one thing at a time
5. **Google the exact error** - Someone else has hit this before
6. **Document the solution** - Future you will thank present you

### DevOps Mindset

1. **Observability is not optional** - You can't fix what you can't see
2. **Automate repetitive tasks** - CI/CD, provisioning, health checks
3. **Fail fast, recover faster** - Health checks, graceful shutdown
4. **Cattle, not pets** - Containers are replaceable, don't manually configure them
5. **Everything as Code** - Dockerfiles, docker-compose, K8s manifests, CI workflows

---

## 🔗 Resources

### Official Documentation
- [Prometheus Querying Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Provisioning](https://grafana.com/docs/grafana/latest/administration/provisioning/)
- [Docker Compose Specification](https://docs.docker.com/compose/compose-file/)
- [Go Prometheus Client](https://github.com/prometheus/client_golang)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)

### Tutorials That Helped
- [Prometheus + Grafana Tutorial](https://prometheus.io/docs/visualization/grafana/)
- [Docker Health Checks](https://docs.docker.com/engine/reference/builder/#healthcheck)
- [CORS Explained](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)

### Tools I Used
- **VS Code** with Go extension
- **Docker Desktop** (or Docker CLI on Linux)
- **Postman/curl** for API testing
- **Chrome DevTools** for debugging CORS

---

## 📸 Screenshots

### URL Shortener UI
![UI Screenshot](docs/ui-screenshot.png)

### Prometheus Metrics
![Prometheus Graph](docs/prometheus-graph.png)

### Grafana Dashboard
![Grafana Dashboard](docs/grafana-full-dashboard.png)

---

*Last updated: January 18, 2026*

*This document will be updated as I continue learning and building!*
