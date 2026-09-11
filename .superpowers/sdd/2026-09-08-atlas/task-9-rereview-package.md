diff --git a/docker-compose.yml b/docker-compose.yml
index be31756..8c22fd8 100644
--- a/docker-compose.yml
+++ b/docker-compose.yml
@@ -1,10 +1,13 @@
+# Fresh-boot order: up -> apply migrations/001_init.sql once -> restart api
+# (001_init.sql is single-apply; api has no auto-migrate by design)
+# Run: docker compose up -d --build, apply 001_init.sql once, then restart api.
 services:
   postgres:
     image: postgres:16
     mem_limit: 1024m
     environment:
       POSTGRES_USER: atlas
       POSTGRES_PASSWORD: atlas
       POSTGRES_DB: atlas
     command: ["postgres", "-c", "shared_buffers=256MB", "-c", "max_connections=100"]
     volumes:
@@ -25,38 +28,40 @@ services:
       test: ["CMD", "redis-cli", "ping"]
       interval: 5s
       timeout: 5s
       retries: 10
 
   api:
     build:
       context: .
       dockerfile: api/Dockerfile
     mem_limit: 100m
+    restart: unless-stopped
     environment:
       PORT: "8080"
       DATABASE_URL: "postgres://atlas:atlas@postgres:5432/atlas?sslmode=disable"
       REDIS_ADDR: "redis:6379"
       JWT_SECRET: ${JWT_SECRET}
       UPLOAD_DIR: /data/uploads
       SEED: "true"
     volumes:
       - uploads:/data/uploads
     depends_on:
       postgres:
         condition: service_healthy
       redis:
         condition: service_healthy
 
   nginx:
     image: nginx:alpine
     mem_limit: 32m
+    restart: unless-stopped
     ports:
       - "8080:80"
     volumes:
       - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
       - ./web:/usr/share/nginx/html:ro
       - uploads:/usr/share/nginx/uploads:ro
     depends_on:
       - api
 
 volumes:
