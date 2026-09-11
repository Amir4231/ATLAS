diff --git a/api/internal/storage/storage.go b/api/internal/storage/storage.go
index e55fdf0..b386b09 100644
--- a/api/internal/storage/storage.go
+++ b/api/internal/storage/storage.go
@@ -54,15 +54,15 @@ func (l LocalStorage) Save(filename string, data []byte) (string, error) {
 		return "", errors.New("invalid filename")
 	}
 	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
 		return "", err
 	}
 	var prefix [8]byte
 	if _, err := rand.Read(prefix[:]); err != nil {
 		return "", err
 	}
 	name := hex.EncodeToString(prefix[:]) + "-" + filename
-	if err := os.WriteFile(filepath.Join(l.Dir, name), data, 0o600); err != nil {
+	if err := os.WriteFile(filepath.Join(l.Dir, name), data, 0o644); err != nil {
 		return "", err
 	}
 	return strings.TrimSuffix(l.BaseURL, "/") + "/" + name, nil
 }
diff --git a/nginx/nginx.conf b/nginx/nginx.conf
index 5d0ef8f..2747060 100644
--- a/nginx/nginx.conf
+++ b/nginx/nginx.conf
@@ -20,14 +20,15 @@ http {
             proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
             proxy_set_header X-Forwarded-Proto $scheme;
         }
 
         location /uploads/ {
             alias /usr/share/nginx/uploads/;
         }
 
         location / {
             root /usr/share/nginx/html;
-            try_files $uri =404;
+            index index.html;
+            try_files $uri $uri/ /index.html;
         }
     }
 }
