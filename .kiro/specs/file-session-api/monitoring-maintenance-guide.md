# File Session API - Monitoring and Maintenance Guide

## Overview

This guide provides comprehensive procedures for monitoring, maintaining, and troubleshooting the File Session & Metadata Management API in production environments. It covers monitoring setup, maintenance schedules, performance optimization, and operational procedures.

## Monitoring Infrastructure

### 1. Metrics Collection

#### Prometheus Configuration

Create `/etc/prometheus/prometheus.yml`:
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "familyvault_rules.yml"

scrape_configs:
  - job_name: 'familyvault-api'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 10s
    metrics_path: /metrics
    
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['localhost:9100']
      
  - job_name: 'nginx'
    static_configs:
      - targets: ['localhost:9113']

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

#### Application Metrics

```go
// Key metrics to expose
var (
    // Request metrics
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
    
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )
    
    // File operation metrics
    fileUploadsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "file_uploads_total",
            Help: "Total number of file uploads",
        },
        []string{"status"},
    )
    
    fileUploadSize = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "file_upload_size_bytes",
            Help: "Size of uploaded files in bytes",
            Buckets: []float64{1024, 10240, 102400, 1048576, 10485760, 104857600},
        },
        []string{"file_type"},
    )
    
    // Session metrics
    activeSessions = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "active_sessions_total",
            Help: "Number of active sessions",
        },
    )
    
    sessionDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name: "session_duration_seconds",
            Help: "Session duration in seconds",
            Buckets: []float64{300, 900, 1800, 3600, 7200, 14400, 28800, 86400},
        },
    )
    
    // Storage metrics
    storageUsed = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "storage_used_bytes",
            Help: "Storage space used in bytes",
        },
        []string{"type"},
    )
    
    // Error metrics
    errorsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "errors_total",
            Help: "Total number of errors",
        },
        []string{"type", "code"},
    )
)
```

### 2. Alerting Rules

Create `/etc/prometheus/familyvault_rules.yml`:
```yaml
groups:
  - name: familyvault.rules
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: rate(errors_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors per second"
          
      # High response time
      - alert: HighResponseTime
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High response time detected"
          description: "95th percentile response time is {{ $value }} seconds"
          
      # Storage space warning
      - alert: StorageSpaceWarning
        expr: (storage_used_bytes / (storage_used_bytes + node_filesystem_free_bytes)) > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Storage space running low"
          description: "Storage usage is at {{ $value | humanizePercentage }}"
          
      # Storage space critical
      - alert: StorageSpaceCritical
        expr: (storage_used_bytes / (storage_used_bytes + node_filesystem_free_bytes)) > 0.9
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Storage space critically low"
          description: "Storage usage is at {{ $value | humanizePercentage }}"
          
      # Service down
      - alert: ServiceDown
        expr: up{job="familyvault-api"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "FamilyVault API service is down"
          description: "The FamilyVault API service has been down for more than 1 minute"
          
      # High memory usage
      - alert: HighMemoryUsage
        expr: process_resident_memory_bytes / 1024 / 1024 > 512
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is {{ $value }}MB"
          
      # Too many active sessions
      - alert: TooManyActiveSessions
        expr: active_sessions_total > 5000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High number of active sessions"
          description: "Active sessions: {{ $value }}"
```

### 3. Grafana Dashboards

#### Main Dashboard Configuration
```json
{
  "dashboard": {
    "title": "FamilyVault API Dashboard",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{method}} {{endpoint}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          },
          {
            "expr": "histogram_quantile(0.50, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "50th percentile"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(errors_total[5m])",
            "legendFormat": "{{type}} - {{code}}"
          }
        ]
      },
      {
        "title": "Active Sessions",
        "type": "singlestat",
        "targets": [
          {
            "expr": "active_sessions_total"
          }
        ]
      },
      {
        "title": "Storage Usage",
        "type": "graph",
        "targets": [
          {
            "expr": "storage_used_bytes",
            "legendFormat": "{{type}}"
          }
        ]
      },
      {
        "title": "File Upload Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(file_uploads_total[5m])",
            "legendFormat": "{{status}}"
          }
        ]
      }
    ]
  }
}
```

### 4. Log Management

#### Structured Logging Configuration
```go
type Logger struct {
    *logrus.Logger
    requestID string
}

func NewLogger() *Logger {
    log := logrus.New()
    log.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: time.RFC3339,
        FieldMap: logrus.FieldMap{
            logrus.FieldKeyTime:  "timestamp",
            logrus.FieldKeyLevel: "level",
            logrus.FieldKeyMsg:   "message",
        },
    })
    
    return &Logger{Logger: log}
}

func (l *Logger) WithRequest(r *http.Request) *Logger {
    return &Logger{
        Logger: l.Logger.WithFields(logrus.Fields{
            "request_id":   getRequestID(r),
            "method":       r.Method,
            "url":          r.URL.String(),
            "remote_addr":  r.RemoteAddr,
            "user_agent":   r.UserAgent(),
        }).Logger,
        requestID: getRequestID(r),
    }
}
```

#### Log Aggregation with ELK Stack

**Filebeat Configuration** (`/etc/filebeat/filebeat.yml`):
```yaml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - /var/log/familyvault/*.log
  json.keys_under_root: true
  json.add_error_key: true
  fields:
    service: familyvault-api
    environment: production

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "familyvault-api-%{+yyyy.MM.dd}"

processors:
  - add_host_metadata:
      when.not.contains.tags: forwarded
```

**Logstash Configuration** (`/etc/logstash/conf.d/familyvault.conf`):
```ruby
input {
  beats {
    port => 5044
  }
}

filter {
  if [service] == "familyvault-api" {
    # Parse timestamp
    date {
      match => [ "timestamp", "ISO8601" ]
    }
    
    # Extract session ID
    if [message] =~ /session_id/ {
      grok {
        match => { "message" => "session_id\":\"(?<session_id>[^\"]+)" }
      }
    }
    
    # Classify log levels
    if [level] == "error" {
      mutate {
        add_tag => [ "error" ]
      }
    }
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "familyvault-api-%{+YYYY.MM.dd}"
  }
}
```

## Health Checks

### 1. Application Health Endpoints

```go
type HealthChecker struct {
    storage     FileStorage
    manifest    ManifestManager
    sessionMgr  SessionManager
}

func (h *HealthChecker) HealthCheck(w http.ResponseWriter, r *http.Request) {
    health := HealthStatus{
        Status:    "healthy",
        Timestamp: time.Now(),
        Checks:    make(map[string]CheckResult),
    }
    
    // Check storage
    if err := h.checkStorage(); err != nil {
        health.Checks["storage"] = CheckResult{
            Status: "unhealthy",
            Error:  err.Error(),
        }
        health.Status = "unhealthy"
    } else {
        health.Checks["storage"] = CheckResult{Status: "healthy"}
    }
    
    // Check manifest system
    if err := h.checkManifest(); err != nil {
        health.Checks["manifest"] = CheckResult{
            Status: "unhealthy",
            Error:  err.Error(),
        }
        health.Status = "unhealthy"
    } else {
        health.Checks["manifest"] = CheckResult{Status: "healthy"}
    }
    
    // Check session management
    if err := h.checkSessions(); err != nil {
        health.Checks["sessions"] = CheckResult{
            Status: "unhealthy",
            Error:  err.Error(),
        }
        health.Status = "unhealthy"
    } else {
        health.Checks["sessions"] = CheckResult{Status: "healthy"}
    }
    
    // Set HTTP status
    if health.Status == "unhealthy" {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    json.NewEncoder(w).Encode(health)
}

type HealthStatus struct {
    Status    string                 `json:"status"`
    Timestamp time.Time              `json:"timestamp"`
    Checks    map[string]CheckResult `json:"checks"`
}

type CheckResult struct {
    Status string `json:"status"`
    Error  string `json:"error,omitempty"`
}
```

### 2. External Health Monitoring

#### Kubernetes Liveness and Readiness Probes
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

#### External Monitoring Script
```bash
#!/bin/bash
# external_health_check.sh

API_URL="https://api.yourdomain.com"
HEALTH_ENDPOINT="$API_URL/health"
SLACK_WEBHOOK="https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"

check_health() {
    local response=$(curl -s -w "%{http_code}" -o /tmp/health_response "$HEALTH_ENDPOINT")
    local http_code="${response: -3}"
    
    if [ "$http_code" -eq 200 ]; then
        local status=$(jq -r '.status' /tmp/health_response)
        if [ "$status" = "healthy" ]; then
            return 0
        fi
    fi
    
    return 1
}

send_alert() {
    local message="$1"
    curl -X POST -H 'Content-type: application/json' \
        --data "{\"text\":\"🚨 FamilyVault API Alert: $message\"}" \
        "$SLACK_WEBHOOK"
}

# Main health check
if ! check_health; then
    send_alert "API health check failed"
    exit 1
fi

echo "Health check passed"
```

## Performance Monitoring

### 1. Performance Metrics

#### Key Performance Indicators (KPIs)
```go
// Performance tracking
type PerformanceTracker struct {
    requestLatency    *prometheus.HistogramVec
    throughput        *prometheus.CounterVec
    concurrentUsers   *prometheus.Gauge
    resourceUsage     *prometheus.GaugeVec
}

func (pt *PerformanceTracker) TrackRequest(endpoint string, duration time.Duration) {
    pt.requestLatency.WithLabelValues(endpoint).Observe(duration.Seconds())
    pt.throughput.WithLabelValues(endpoint).Inc()
}

func (pt *PerformanceTracker) UpdateResourceUsage() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    pt.resourceUsage.WithLabelValues("memory_heap").Set(float64(m.HeapInuse))
    pt.resourceUsage.WithLabelValues("memory_stack").Set(float64(m.StackInuse))
    pt.resourceUsage.WithLabelValues("goroutines").Set(float64(runtime.NumGoroutine()))
}
```

### 2. Performance Benchmarking

#### Load Testing Script
```bash
#!/bin/bash
# load_test.sh

API_URL="http://localhost:8080"
SESSION_ID="load-test-session-$(date +%s)"
CONCURRENT_USERS=50
TEST_DURATION=300  # 5 minutes

echo "Starting load test with $CONCURRENT_USERS concurrent users for ${TEST_DURATION}s"

# Create test files
mkdir -p /tmp/load_test_files
for i in {1..10}; do
    dd if=/dev/urandom of="/tmp/load_test_files/test_file_$i.bin" bs=1M count=1 2>/dev/null
done

# Upload test
echo "Testing file uploads..."
ab -n 1000 -c $CONCURRENT_USERS -T "multipart/form-data" \
   -H "X-Session-ID: $SESSION_ID" \
   -p /tmp/upload_data.txt \
   "$API_URL/upload"

# Metadata update test
echo "Testing metadata updates..."
ab -n 2000 -c $CONCURRENT_USERS -T "application/json" \
   -H "X-Session-ID: $SESSION_ID" \
   -p /tmp/metadata_update.json \
   "$API_URL/update-metadata"

# File listing test
echo "Testing file listing..."
ab -n 5000 -c $CONCURRENT_USERS \
   -H "X-Session-ID: $SESSION_ID" \
   "$API_URL/files"

# Cleanup
rm -rf /tmp/load_test_files
echo "Load test completed"
```

#### Performance Profiling
```go
// Enable pprof in production (with authentication)
import _ "net/http/pprof"

func enableProfiling() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}

// Custom profiling endpoints
func (s *Server) profileHandler(w http.ResponseWriter, r *http.Request) {
    // Authenticate request
    if !s.authenticateAdmin(r) {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    switch r.URL.Path {
    case "/debug/pprof/heap":
        pprof.Handler("heap").ServeHTTP(w, r)
    case "/debug/pprof/goroutine":
        pprof.Handler("goroutine").ServeHTTP(w, r)
    case "/debug/pprof/profile":
        pprof.Profile(w, r)
    default:
        pprof.Index(w, r)
    }
}
```

## Maintenance Procedures

### 1. Routine Maintenance Schedule

#### Daily Tasks (Automated)
```bash
#!/bin/bash
# daily_maintenance.sh

LOG_FILE="/var/log/familyvault/maintenance.log"
DATE=$(date '+%Y-%m-%d %H:%M:%S')

echo "[$DATE] Starting daily maintenance" >> "$LOG_FILE"

# Check disk space
DISK_USAGE=$(df /var/lib/familyvault | awk 'NR==2 {print $5}' | sed 's/%//')
if [ "$DISK_USAGE" -gt 80 ]; then
    echo "[$DATE] WARNING: Disk usage is ${DISK_USAGE}%" >> "$LOG_FILE"
    # Send alert
    curl -X POST -H 'Content-type: application/json' \
        --data "{\"text\":\"⚠️ FamilyVault: Disk usage is ${DISK_USAGE}%\"}" \
        "$SLACK_WEBHOOK"
fi

# Clean up expired sessions
/usr/local/bin/familyvault-api -cleanup-expired-sessions >> "$LOG_FILE" 2>&1

# Rotate logs
logrotate /etc/logrotate.d/familyvault

# Check service health
if ! systemctl is-active --quiet familyvault-api; then
    echo "[$DATE] ERROR: Service is not running" >> "$LOG_FILE"
    systemctl restart familyvault-api
fi

# Backup critical data
/usr/local/bin/backup_familyvault.sh >> "$LOG_FILE" 2>&1

echo "[$DATE] Daily maintenance completed" >> "$LOG_FILE"
```

#### Weekly Tasks
```bash
#!/bin/bash
# weekly_maintenance.sh

LOG_FILE="/var/log/familyvault/maintenance.log"
DATE=$(date '+%Y-%m-%d %H:%M:%S')

echo "[$DATE] Starting weekly maintenance" >> "$LOG_FILE"

# Update system packages (security only)
apt list --upgradable | grep -i security
if [ $? -eq 0 ]; then
    apt update && apt upgrade -y --only-upgrade $(apt list --upgradable 2>/dev/null | grep -i security | cut -d'/' -f1)
fi

# Analyze performance metrics
/usr/local/bin/analyze_performance.sh >> "$LOG_FILE" 2>&1

# Check SSL certificate expiration
SSL_EXPIRY=$(openssl x509 -in /etc/ssl/certs/familyvault.crt -noout -dates | grep notAfter | cut -d= -f2)
SSL_EXPIRY_EPOCH=$(date -d "$SSL_EXPIRY" +%s)
CURRENT_EPOCH=$(date +%s)
DAYS_UNTIL_EXPIRY=$(( (SSL_EXPIRY_EPOCH - CURRENT_EPOCH) / 86400 ))

if [ "$DAYS_UNTIL_EXPIRY" -lt 30 ]; then
    echo "[$DATE] WARNING: SSL certificate expires in $DAYS_UNTIL_EXPIRY days" >> "$LOG_FILE"
fi

# Database integrity check (if applicable)
/usr/local/bin/check_data_integrity.sh >> "$LOG_FILE" 2>&1

echo "[$DATE] Weekly maintenance completed" >> "$LOG_FILE"
```

#### Monthly Tasks
```bash
#!/bin/bash
# monthly_maintenance.sh

LOG_FILE="/var/log/familyvault/maintenance.log"
DATE=$(date '+%Y-%m-%d %H:%M:%S')

echo "[$DATE] Starting monthly maintenance" >> "$LOG_FILE"

# Full system backup
/usr/local/bin/full_backup.sh >> "$LOG_FILE" 2>&1

# Security audit
/usr/local/bin/security_audit.sh >> "$LOG_FILE" 2>&1

# Performance optimization
/usr/local/bin/optimize_performance.sh >> "$LOG_FILE" 2>&1

# Update documentation
/usr/local/bin/update_runbooks.sh >> "$LOG_FILE" 2>&1

echo "[$DATE] Monthly maintenance completed" >> "$LOG_FILE"
```

### 2. Backup and Recovery

#### Automated Backup Script
```bash
#!/bin/bash
# backup_familyvault.sh

BACKUP_DIR="/backup/familyvault"
SOURCE_DIR="/var/lib/familyvault"
DATE=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=30

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Create incremental backup
rsync -av --link-dest="$BACKUP_DIR/latest" \
      "$SOURCE_DIR/" \
      "$BACKUP_DIR/backup_$DATE/"

# Update latest symlink
rm -f "$BACKUP_DIR/latest"
ln -s "backup_$DATE" "$BACKUP_DIR/latest"

# Compress old backups
find "$BACKUP_DIR" -name "backup_*" -type d -mtime +1 -exec tar -czf {}.tar.gz {} \; -exec rm -rf {} \;

# Clean old backups
find "$BACKUP_DIR" -name "backup_*.tar.gz" -mtime +$RETENTION_DAYS -delete

# Verify backup
if [ -d "$BACKUP_DIR/backup_$DATE" ]; then
    echo "Backup completed successfully: backup_$DATE"
    
    # Test restore (dry run)
    rsync -av --dry-run "$BACKUP_DIR/backup_$DATE/" /tmp/restore_test/
    if [ $? -eq 0 ]; then
        echo "Backup verification passed"
    else
        echo "Backup verification failed"
        exit 1
    fi
else
    echo "Backup failed"
    exit 1
fi
```

#### Recovery Procedures
```bash
#!/bin/bash
# restore_familyvault.sh

BACKUP_DIR="/backup/familyvault"
TARGET_DIR="/var/lib/familyvault"
BACKUP_DATE="$1"

if [ -z "$BACKUP_DATE" ]; then
    echo "Usage: $0 <backup_date>"
    echo "Available backups:"
    ls -la "$BACKUP_DIR" | grep backup_
    exit 1
fi

# Stop service
systemctl stop familyvault-api

# Backup current state
mv "$TARGET_DIR" "${TARGET_DIR}.pre_restore.$(date +%s)"

# Restore from backup
if [ -f "$BACKUP_DIR/backup_${BACKUP_DATE}.tar.gz" ]; then
    tar -xzf "$BACKUP_DIR/backup_${BACKUP_DATE}.tar.gz" -C "$(dirname $TARGET_DIR)"
    mv "$(dirname $TARGET_DIR)/backup_${BACKUP_DATE}" "$TARGET_DIR"
elif [ -d "$BACKUP_DIR/backup_${BACKUP_DATE}" ]; then
    cp -r "$BACKUP_DIR/backup_${BACKUP_DATE}" "$TARGET_DIR"
else
    echo "Backup not found: backup_${BACKUP_DATE}"
    exit 1
fi

# Set permissions
chown -R familyvault:familyvault "$TARGET_DIR"
chmod -R 755 "$TARGET_DIR"

# Start service
systemctl start familyvault-api

# Verify restoration
sleep 10
if systemctl is-active --quiet familyvault-api; then
    echo "Restoration completed successfully"
else
    echo "Service failed to start after restoration"
    exit 1
fi
```

### 3. Database Maintenance (if applicable)

#### Data Integrity Checks
```bash
#!/bin/bash
# check_data_integrity.sh

SESSIONS_DIR="/var/lib/familyvault/sessions"
ERRORS=0

echo "Checking data integrity..."

# Check for orphaned files
find "$SESSIONS_DIR" -name "*.json" -type f | while read manifest; do
    session_dir=$(dirname "$manifest")
    session_id=$(basename "$session_dir")
    
    # Check if manifest is valid JSON
    if ! jq empty "$manifest" 2>/dev/null; then
        echo "ERROR: Invalid JSON in $manifest"
        ((ERRORS++))
    fi
    
    # Check if referenced files exist
    jq -r '.files | keys[]' "$manifest" 2>/dev/null | while read file_id; do
        if [ ! -f "$session_dir/files/$file_id" ]; then
            echo "ERROR: Missing file $file_id in session $session_id"
            ((ERRORS++))
        fi
    done
done

# Check for files without manifest entries
find "$SESSIONS_DIR" -path "*/files/*" -type f | while read file_path; do
    session_dir=$(dirname "$(dirname "$file_path")")
    file_id=$(basename "$file_path")
    manifest="$session_dir/manifest.json"
    
    if [ -f "$manifest" ]; then
        if ! jq -e ".files[\"$file_id\"]" "$manifest" >/dev/null 2>&1; then
            echo "WARNING: Orphaned file $file_path"
        fi
    fi
done

if [ $ERRORS -eq 0 ]; then
    echo "Data integrity check passed"
else
    echo "Data integrity check found $ERRORS errors"
    exit 1
fi
```

## Troubleshooting Guide

### 1. Common Issues and Solutions

#### High Memory Usage
```bash
# Check memory usage
ps aux | grep familyvault-api
cat /proc/$(pgrep familyvault-api)/status | grep -E "(VmRSS|VmSize)"

# Generate heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Check for memory leaks
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof
```

#### High CPU Usage
```bash
# Generate CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Check system load
uptime
iostat 1 5
```

#### Storage Issues
```bash
# Check disk space
df -h /var/lib/familyvault

# Check inode usage
df -i /var/lib/familyvault

# Find large files
find /var/lib/familyvault -type f -size +100M -exec ls -lh {} \;

# Check for corrupted files
find /var/lib/familyvault -name "*.json" -exec jq empty {} \; 2>&1 | grep -v "parse error"
```

### 2. Emergency Procedures

#### Service Recovery
```bash
#!/bin/bash
# emergency_recovery.sh

echo "Starting emergency recovery..."

# Stop service
systemctl stop familyvault-api

# Check for corrupted files
/usr/local/bin/check_data_integrity.sh
if [ $? -ne 0 ]; then
    echo "Data corruption detected, restoring from backup..."
    /usr/local/bin/restore_familyvault.sh latest
fi

# Clear temporary files
rm -rf /tmp/familyvault/*

# Reset permissions
chown -R familyvault:familyvault /var/lib/familyvault
chmod -R 755 /var/lib/familyvault

# Start service
systemctl start familyvault-api

# Wait and check
sleep 10
if systemctl is-active --quiet familyvault-api; then
    echo "Emergency recovery completed successfully"
else
    echo "Emergency recovery failed, manual intervention required"
    exit 1
fi
```

#### Disaster Recovery
```bash
#!/bin/bash
# disaster_recovery.sh

BACKUP_LOCATION="$1"
NEW_SERVER_IP="$2"

if [ -z "$BACKUP_LOCATION" ] || [ -z "$NEW_SERVER_IP" ]; then
    echo "Usage: $0 <backup_location> <new_server_ip>"
    exit 1
fi

echo "Starting disaster recovery..."

# Install application
/usr/local/bin/install_familyvault.sh

# Restore data
rsync -av "$BACKUP_LOCATION/" /var/lib/familyvault/

# Update configuration
sed -i "s/old_server_ip/$NEW_SERVER_IP/g" /etc/familyvault/config.yaml

# Update DNS/load balancer
# (This would be specific to your infrastructure)

# Start services
systemctl enable familyvault-api
systemctl start familyvault-api

# Verify recovery
/usr/local/bin/verify_recovery.sh

echo "Disaster recovery completed"
```

## Performance Optimization

### 1. Application Optimization

#### Memory Optimization
```go
// Implement object pooling for frequently used objects
var fileRecordPool = sync.Pool{
    New: func() interface{} {
        return &FileRecord{}
    },
}

func getFileRecord() *FileRecord {
    return fileRecordPool.Get().(*FileRecord)
}

func putFileRecord(fr *FileRecord) {
    // Reset fields
    *fr = FileRecord{}
    fileRecordPool.Put(fr)
}

// Use streaming for large file operations
func (s *FileStorage) StreamFile(sessionID, fileID string, w io.Writer) error {
    file, err := os.Open(s.getFilePath(sessionID, fileID))
    if err != nil {
        return err
    }
    defer file.Close()
    
    _, err = io.Copy(w, file)
    return err
}
```

#### Database Optimization
```go
// Implement caching for frequently accessed data
type ManifestCache struct {
    cache map[string]*CachedManifest
    mutex sync.RWMutex
    ttl   time.Duration
}

type CachedManifest struct {
    manifest  *Manifest
    timestamp time.Time
}

func (mc *ManifestCache) Get(sessionID string) (*Manifest, bool) {
    mc.mutex.RLock()
    defer mc.mutex.RUnlock()
    
    cached, exists := mc.cache[sessionID]
    if !exists || time.Since(cached.timestamp) > mc.ttl {
        return nil, false
    }
    
    return cached.manifest, true
}
```

### 2. System Optimization

#### File System Optimization
```bash
# Optimize file system for many small files
mount -o remount,noatime,nodiratime /var/lib/familyvault

# Use appropriate file system
# For many small files: ext4 with dir_index
# For large files: xfs

# Optimize I/O scheduler
echo mq-deadline > /sys/block/sda/queue/scheduler
```

#### Network Optimization
```bash
# Optimize TCP settings
echo 'net.core.rmem_max = 16777216' >> /etc/sysctl.conf
echo 'net.core.wmem_max = 16777216' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_rmem = 4096 87380 16777216' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_wmem = 4096 65536 16777216' >> /etc/sysctl.conf
sysctl -p
```

## Conclusion

This monitoring and maintenance guide provides comprehensive procedures for operating the File Session API in production. Regular monitoring, proactive maintenance, and quick incident response are essential for maintaining service reliability and performance.

Key takeaways:
- Implement comprehensive monitoring with alerts
- Automate routine maintenance tasks
- Maintain regular backup and recovery procedures
- Have emergency procedures ready
- Continuously optimize performance based on metrics
- Document all procedures and keep them updated

Regular review and updates of these procedures ensure continued operational excellence as the system evolves and scales.