# File Session API - Deployment Guide

## Overview

This guide provides comprehensive instructions for deploying the File Session & Metadata Management API in production environments, including configuration examples, security best practices, and operational procedures.

## Configuration Examples

### Environment Configuration

Create a `.env` file or set environment variables:

```bash
# Server Configuration
PORT=8080
HOST=0.0.0.0
ENV=production

# Session Configuration
SESSION_TIMEOUT=24h
SESSION_CLEANUP_INTERVAL=1h
MAX_SESSIONS=10000

# File Storage Configuration
STORAGE_ROOT=/var/lib/familyvault/sessions
MAX_FILE_SIZE=104857600  # 100MB in bytes
ALLOWED_FILE_TYPES=jpg,png,pdf,docx,csv
TEMP_DIR=/tmp/familyvault

# Security Configuration
ENABLE_CORS=true
CORS_ORIGINS=https://yourdomain.com,https://app.yourdomain.com
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=60s

# Logging Configuration
LOG_LEVEL=info
LOG_FORMAT=json
LOG_FILE=/var/log/familyvault/api.log

# Monitoring Configuration
METRICS_ENABLED=true
METRICS_PORT=9090
HEALTH_CHECK_INTERVAL=30s
```

### Application Configuration (config.yaml)

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  max_header_bytes: 1048576

session:
  timeout: 24h
  cleanup_interval: 1h
  max_sessions: 10000
  id_length: 32

storage:
  root: "/var/lib/familyvault/sessions"
  temp_dir: "/tmp/familyvault"
  max_file_size: 104857600
  allowed_types:
    - "jpg"
    - "jpeg"
    - "png"
    - "pdf"
    - "docx"
    - "csv"

security:
  cors:
    enabled: true
    origins:
      - "https://yourdomain.com"
      - "https://app.yourdomain.com"
    methods: ["GET", "POST", "PATCH", "DELETE", "OPTIONS"]
    headers: ["Content-Type", "X-Session-ID", "Authorization"]
  
  rate_limiting:
    enabled: true
    requests_per_minute: 100
    burst: 20
  
  sanitization:
    max_metadata_fields: 50
    max_metadata_value_length: 255
    strip_html: true

logging:
  level: "info"
  format: "json"
  file: "/var/log/familyvault/api.log"
  max_size: 100  # MB
  max_backups: 5
  max_age: 30    # days

monitoring:
  metrics:
    enabled: true
    port: 9090
    path: "/metrics"
  
  health_check:
    enabled: true
    interval: 30s
    timeout: 5s
```

### Docker Configuration

#### Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Create necessary directories
RUN mkdir -p /var/lib/familyvault/sessions
RUN mkdir -p /var/log/familyvault
RUN mkdir -p /tmp/familyvault

COPY --from=builder /app/main .
COPY --from=builder /app/config.yaml .

# Create non-root user
RUN addgroup -g 1001 familyvault && \
    adduser -D -s /bin/sh -u 1001 -G familyvault familyvault

# Set ownership
RUN chown -R familyvault:familyvault /var/lib/familyvault
RUN chown -R familyvault:familyvault /var/log/familyvault
RUN chown -R familyvault:familyvault /tmp/familyvault

USER familyvault

EXPOSE 8080 9090

CMD ["./main"]
```

#### docker-compose.yml
```yaml
version: '3.8'

services:
  familyvault-api:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      - ENV=production
      - LOG_LEVEL=info
    volumes:
      - ./data:/var/lib/familyvault/sessions
      - ./logs:/var/log/familyvault
      - ./config.yaml:/root/config.yaml:ro
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-storage:/var/lib/grafana
      - ./monitoring/grafana/dashboards:/etc/grafana/provisioning/dashboards:ro
      - ./monitoring/grafana/datasources:/etc/grafana/provisioning/datasources:ro
    restart: unless-stopped

volumes:
  grafana-storage:
```

### Kubernetes Deployment

#### deployment.yaml
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: familyvault-api
  labels:
    app: familyvault-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: familyvault-api
  template:
    metadata:
      labels:
        app: familyvault-api
    spec:
      containers:
      - name: familyvault-api
        image: familyvault/api:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: ENV
          value: "production"
        - name: STORAGE_ROOT
          value: "/data/sessions"
        volumeMounts:
        - name: storage
          mountPath: /data
        - name: config
          mountPath: /root/config.yaml
          subPath: config.yaml
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: storage
        persistentVolumeClaim:
          claimName: familyvault-storage
      - name: config
        configMap:
          name: familyvault-config

---
apiVersion: v1
kind: Service
metadata:
  name: familyvault-api-service
spec:
  selector:
    app: familyvault-api
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
  type: LoadBalancer

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: familyvault-storage
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: familyvault-config
data:
  config.yaml: |
    server:
      port: 8080
      host: "0.0.0.0"
    storage:
      root: "/data/sessions"
      max_file_size: 104857600
    # ... rest of configuration
```

## Deployment Steps

### 1. Pre-deployment Checklist

- [ ] Verify Go version compatibility (1.21+)
- [ ] Ensure sufficient disk space for file storage
- [ ] Configure firewall rules for ports 8080 and 9090
- [ ] Set up SSL/TLS certificates
- [ ] Create necessary system users and directories
- [ ] Configure log rotation
- [ ] Set up monitoring infrastructure

### 2. System Preparation

```bash
# Create system user
sudo useradd -r -s /bin/false familyvault

# Create directories
sudo mkdir -p /var/lib/familyvault/sessions
sudo mkdir -p /var/log/familyvault
sudo mkdir -p /etc/familyvault

# Set permissions
sudo chown -R familyvault:familyvault /var/lib/familyvault
sudo chown -R familyvault:familyvault /var/log/familyvault
sudo chmod 755 /var/lib/familyvault/sessions
```

### 3. Application Deployment

```bash
# Build the application
go build -o familyvault-api ./cmd/server

# Copy binary and configuration
sudo cp familyvault-api /usr/local/bin/
sudo cp config.yaml /etc/familyvault/
sudo chmod +x /usr/local/bin/familyvault-api
```

### 4. Service Configuration (systemd)

Create `/etc/systemd/system/familyvault-api.service`:

```ini
[Unit]
Description=FamilyVault File Session API
After=network.target

[Service]
Type=simple
User=familyvault
Group=familyvault
ExecStart=/usr/local/bin/familyvault-api -config /etc/familyvault/config.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=familyvault-api

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/familyvault /var/log/familyvault /tmp/familyvault

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable familyvault-api
sudo systemctl start familyvault-api
```

### 5. Reverse Proxy Configuration (Nginx)

```nginx
upstream familyvault_api {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    ssl_certificate /path/to/ssl/cert.pem;
    ssl_certificate_key /path/to/ssl/key.pem;

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";

    # File upload size limit
    client_max_body_size 100M;

    location / {
        proxy_pass http://familyvault_api;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Metrics endpoint (restrict access)
    location /metrics {
        proxy_pass http://127.0.0.1:9090/metrics;
        allow 10.0.0.0/8;
        allow 172.16.0.0/12;
        allow 192.168.0.0/16;
        deny all;
    }
}
```

## Post-Deployment Verification

### 1. Health Check
```bash
curl -f http://localhost:8080/health
```

### 2. API Functionality Test
```bash
# Test file upload
curl -X POST http://localhost:8080/upload \
  -H "X-Session-ID: test-session-123" \
  -F "file=@test.pdf"

# Test metadata update
curl -X PATCH http://localhost:8080/update-metadata \
  -H "X-Session-ID: test-session-123" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "file_001", "metadata": {"category": "test"}}'
```

### 3. Performance Test
```bash
# Install Apache Bench
sudo apt-get install apache2-utils

# Run load test
ab -n 1000 -c 10 -H "X-Session-ID: test-session" http://localhost:8080/files
```

## Rollback Procedures

### 1. Service Rollback
```bash
# Stop current service
sudo systemctl stop familyvault-api

# Restore previous binary
sudo cp /backup/familyvault-api.backup /usr/local/bin/familyvault-api

# Restore previous configuration
sudo cp /backup/config.yaml.backup /etc/familyvault/config.yaml

# Start service
sudo systemctl start familyvault-api
```

### 2. Database/Storage Rollback
```bash
# Stop service
sudo systemctl stop familyvault-api

# Restore storage from backup
sudo rsync -av /backup/sessions/ /var/lib/familyvault/sessions/

# Start service
sudo systemctl start familyvault-api
```

## Troubleshooting

### Common Issues

1. **Service won't start**
   - Check logs: `sudo journalctl -u familyvault-api -f`
   - Verify configuration: `familyvault-api -config /etc/familyvault/config.yaml -validate`
   - Check permissions: `ls -la /var/lib/familyvault`

2. **File upload failures**
   - Check disk space: `df -h /var/lib/familyvault`
   - Verify file permissions: `ls -la /var/lib/familyvault/sessions`
   - Check file size limits in configuration

3. **High memory usage**
   - Monitor with: `top -p $(pgrep familyvault-api)`
   - Check for memory leaks in logs
   - Restart service if necessary: `sudo systemctl restart familyvault-api`

### Log Analysis
```bash
# View recent logs
sudo journalctl -u familyvault-api -n 100

# Follow logs in real-time
sudo journalctl -u familyvault-api -f

# Filter error logs
sudo journalctl -u familyvault-api -p err

# View logs by date
sudo journalctl -u familyvault-api --since "2025-01-15 10:00:00"
```