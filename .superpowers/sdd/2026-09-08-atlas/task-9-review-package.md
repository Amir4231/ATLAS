diff --git a/api/Dockerfile b/api/Dockerfile
new file mode 100644
index 0000000..eab12a2
--- /dev/null
+++ b/api/Dockerfile
@@ -0,0 +1,12 @@
+FROM golang:1.26-alpine AS build
+WORKDIR /src
+COPY api/go.mod api/go.sum ./
+RUN go mod download
+COPY api/ ./
+RUN CGO_ENABLED=0 go build -o /atlas .
+
+FROM alpine:3.21
+RUN apk add --no-cache ca-certificates
+COPY --from=build /atlas /atlas
+EXPOSE 8080
+CMD ["/atlas"]
diff --git a/docker-compose.yml b/docker-compose.yml
new file mode 100644
index 0000000..be31756
--- /dev/null
+++ b/docker-compose.yml
@@ -0,0 +1,65 @@
+services:
+  postgres:
+    image: postgres:16
+    mem_limit: 1024m
+    environment:
+      POSTGRES_USER: atlas
+      POSTGRES_PASSWORD: atlas
+      POSTGRES_DB: atlas
+    command: ["postgres", "-c", "shared_buffers=256MB", "-c", "max_connections=100"]
+    volumes:
+      - pgdata:/var/lib/postgresql/data
+    healthcheck:
+      test: ["CMD-SHELL", "pg_isready -U atlas -d atlas"]
+      interval: 5s
+      timeout: 5s
+      retries: 10
+
+  redis:
+    image: redis:7
+    mem_limit: 128m
+    command: ["redis-server", "--maxmemory", "96mb", "--maxmemory-policy", "allkeys-lru"]
+    volumes:
+      - redisdata:/data
+    healthcheck:
+      test: ["CMD", "redis-cli", "ping"]
+      interval: 5s
+      timeout: 5s
+      retries: 10
+
+  api:
+    build:
+      context: .
+      dockerfile: api/Dockerfile
+    mem_limit: 100m
+    environment:
+      PORT: "8080"
+      DATABASE_URL: "postgres://atlas:atlas@postgres:5432/atlas?sslmode=disable"
+      REDIS_ADDR: "redis:6379"
+      JWT_SECRET: ${JWT_SECRET}
+      UPLOAD_DIR: /data/uploads
+      SEED: "true"
+    volumes:
+      - uploads:/data/uploads
+    depends_on:
+      postgres:
+        condition: service_healthy
+      redis:
+        condition: service_healthy
+
+  nginx:
+    image: nginx:alpine
+    mem_limit: 32m
+    ports:
+      - "8080:80"
+    volumes:
+      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
+      - ./web:/usr/share/nginx/html:ro
+      - uploads:/usr/share/nginx/uploads:ro
+    depends_on:
+      - api
+
+volumes:
+  pgdata:
+  redisdata:
+  uploads:
diff --git a/nginx/nginx.conf b/nginx/nginx.conf
new file mode 100644
index 0000000..5d0ef8f
--- /dev/null
+++ b/nginx/nginx.conf
@@ -0,0 +1,33 @@
+events {}
+
+http {
+    include /etc/nginx/mime.types;
+    default_type application/octet-stream;
+    access_log /var/log/nginx/access.log;
+    error_log /var/log/nginx/error.log warn;
+
+    upstream api_upstream {
+        server api:8080;
+    }
+
+    server {
+        listen 80;
+
+        location /v1/ {
+            proxy_pass http://api_upstream;
+            proxy_set_header Host $host;
+            proxy_set_header X-Real-IP $remote_addr;
+            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
+            proxy_set_header X-Forwarded-Proto $scheme;
+        }
+
+        location /uploads/ {
+            alias /usr/share/nginx/uploads/;
+        }
+
+        location / {
+            root /usr/share/nginx/html;
+            try_files $uri =404;
+        }
+    }
+}
diff --git a/web/.gitkeep b/web/.gitkeep
new file mode 100644
index 0000000..e69de29
