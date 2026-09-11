diff --git a/web/.gitkeep b/web/.gitkeep
deleted file mode 100644
index e69de29..0000000
diff --git a/web/admin.html b/web/admin.html
new file mode 100644
index 0000000..99837b1
--- /dev/null
+++ b/web/admin.html
@@ -0,0 +1,95 @@
+<!DOCTYPE html>
+<html lang="en">
+<head>
+<meta charset="utf-8">
+<meta name="viewport" content="width=device-width, initial-scale=1">
+<title>Atlas ΓÇö Admin</title>
+<link rel="manifest" href="manifest.json">
+<style>
+body{font-family:system-ui,sans-serif;max-width:720px;margin:1rem auto;padding:0 1rem}
+input,button{font-size:1rem;padding:.5rem;margin:.25rem 0}button{cursor:pointer}
+.msg{margin:.5rem 0}.err{color:#b00}.ok{color:#060}
+table{border-collapse:collapse;width:100%;font-size:.9rem}td,th{border:1px solid #999;padding:.3rem .5rem;text-align:left}
+.top{display:flex;justify-content:space-between;align-items:baseline}
+label{display:block;margin:.3rem 0}
+</style>
+</head>
+<body>
+<div class="top"><h1>Admin</h1><span><span id="who"></span> <button id="out">Logout</button></span></div>
+
+<h2>College rate</h2>
+<div id="college"></div>
+
+<h2>Create class</h2>
+<form id="cf">
+<label>Name <input id="name" required></label>
+<label>Homeroom teacher ID <input id="tid" type="number" placeholder="(optional)"></label>
+<label>Geofence lat <input id="lat" type="number" step="any" value="3.1390" required></label>
+<label>Geofence lng <input id="lng" type="number" step="any" value="101.6869" required></label>
+<label>Radius (m) <input id="rad" type="number" value="30" required></label>
+<button type="submit">Create</button>
+</form>
+<p id="m" class="msg"></p>
+
+<h2>Classes</h2>
+<ul id="cls"></ul>
+
+<h2>Audit log</h2>
+<button id="audit">Refresh audit</button>
+<table><thead><tr><th>id</th><th>actor</th><th>action</th><th>entity</th><th>entity_id</th><th>at</th></tr></thead>
+<tbody id="logs"></tbody></table>
+
+<script src="app.js"></script>
+<script>
+(function () {
+  Atlas.el('out').onclick = Atlas.logout;
+  Atlas.requireRole(['admin']).then(function (me) {
+    if (!me) return;
+    Atlas.el('who').textContent = me.email;
+    loadCollege(); loadClasses(); loadAudit();
+  });
+
+  async function loadCollege() {
+    var r = await Atlas.api('GET', '/v1/analytics/college');
+    Atlas.el('college').textContent = r.status === 200
+      ? 'present ' + r.data.present + ' / ' + r.data.total + ' (' + r.data.pct + '%)'
+      : 'College analytics failed (' + r.status + ')';
+  }
+
+  document.getElementById('cf').addEventListener('submit', async function (e) {
+    e.preventDefault();
+    var body = {
+      class_name: Atlas.el('name').value.trim(),
+      geofence_lat: +Atlas.el('lat').value,
+      geofence_lng: +Atlas.el('lng').value,
+      geofence_radius_m: +Atlas.el('rad').value
+    };
+    if (Atlas.el('tid').value) body.homeroom_teacher_id = +Atlas.el('tid').value;
+    var r = await Atlas.api('POST', '/v1/admin/classes', body);
+    if (r.status !== 200) { Atlas.msg('m', 'Create class failed (' + r.status + '): ' + (r.data.error || ''), true); return; }
+    Atlas.msg('m', 'Class #' + r.data.id + ' created', false);
+    loadClasses();
+  });
+
+  async function loadClasses() {
+    var r = await Atlas.api('GET', '/v1/classes');
+    if (r.status !== 200) return;
+    Atlas.el('cls').innerHTML = r.data.classes.map(function (c) {
+      return '<li>#' + c.id + ' ' + c.class_name + ' @ ' + c.geofence_lat + ',' + c.geofence_lng + ' r=' + c.geofence_radius_m + 'm</li>';
+    }).join('');
+  }
+
+  Atlas.el('audit').onclick = loadAudit;
+  async function loadAudit() {
+    var r = await Atlas.api('GET', '/v1/audit-logs?limit=50');
+    if (r.status !== 200) { Atlas.msg('m', 'Audit load failed (' + r.status + ')', true); return; }
+    Atlas.el('logs').innerHTML = r.data.logs.map(function (l) {
+      return '<tr><td>' + l.id + '</td><td>' + (l.actor_id == null ? 'ΓÇö' : l.actor_id) +
+        '</td><td>' + l.action + '</td><td>' + l.entity + '</td><td>' + l.entity_id +
+        '</td><td>' + l.created_at + '</td></tr>';
+    }).join('');
+  }
+})();
+</script>
+</body>
+</html>
diff --git a/web/app.js b/web/app.js
new file mode 100644
index 0000000..fbfcf55
--- /dev/null
+++ b/web/app.js
@@ -0,0 +1,121 @@
+/* Atlas shared frontend helper ΓÇö vanilla JS, no build step.
+ * Same-origin API (nginx proxies /v1/ to the Go API).
+ * Token: localStorage 'atlas_token'. 401 with a stored token -> back to login.
+ */
+(function () {
+  'use strict';
+
+  var TOKEN_KEY = 'atlas_token';
+
+  function getToken() {
+    try { return localStorage.getItem(TOKEN_KEY) || ''; } catch (e) { return ''; }
+  }
+  function setToken(t) {
+    try {
+      if (t) localStorage.setItem(TOKEN_KEY, t);
+      else localStorage.removeItem(TOKEN_KEY);
+    } catch (e) { /* storage unavailable */ }
+  }
+
+  // Low-level call. Returns {status, data}. Never throws on HTTP errors.
+  async function api(method, path, body, opts) {
+    opts = opts || {};
+    var headers = {};
+    var tok = getToken();
+    if (tok) headers['Authorization'] = 'Bearer ' + tok;
+    var payload;
+    if (body instanceof FormData) {
+      payload = body; // browser sets multipart boundary
+    } else if (body !== undefined) {
+      headers['Content-Type'] = 'application/json';
+      payload = JSON.stringify(body);
+    }
+    var res;
+    try {
+      res = await fetch(path, { method: method, headers: headers, body: payload });
+    } catch (e) {
+      return { status: 0, data: { error: 'NETWORK_ERROR' } };
+    }
+    var data = null;
+    try { data = await res.json(); } catch (e) { data = null; }
+    return { status: res.status, data: data || {} };
+  }
+
+  async function login(email, password) {
+    var r = await api('POST', '/v1/auth/login', { email: email, password: password });
+    if (r.status === 200 && r.data.token) {
+      setToken(r.data.token);
+      return { ok: true };
+    }
+    return { ok: false, status: r.status, error: r.data.error || 'LOGIN_FAILED' };
+  }
+
+  function logout() {
+    setToken('');
+    location.href = 'index.html';
+  }
+
+  function roleHome(role) {
+    if (role === 'admin') return 'admin.html';
+    if (role === 'teacher') return 'teacher.html';
+    if (role === 'class_rep' || role === 'rep') return 'rep.html';
+    if (role === 'student') return 'student.html';
+    return 'index.html';
+  }
+
+  // Page guard: require a stored token + one of roles. Resolves with /v1/me.
+  // Redirects to index.html on missing/invalid token or wrong role.
+  async function requireRole(allowed) {
+    if (!getToken()) {
+      location.href = 'index.html';
+      return null;
+    }
+    var r = await api('GET', '/v1/me');
+    if (r.status === 401 || r.status === 403 || r.status === 0) {
+      setToken('');
+      location.href = 'index.html';
+      return null;
+    }
+    if (r.status !== 200 || !r.data || allowed.indexOf(r.data.role) === -1) {
+      if (r.data && r.data.role) location.href = roleHome(r.data.role);
+      else location.href = 'index.html';
+      return null;
+    }
+    return r.data;
+  }
+
+  function el(id) { return document.getElementById(id); }
+  function msg(id, text, isErr) {
+    var n = el(id);
+    if (!n) return;
+    n.textContent = text || '';
+    n.className = text ? (isErr ? 'msg err' : 'msg ok') : 'msg';
+  }
+
+  // Distinct scan-error wording per API contract.
+  function scanError(status, data) {
+    data = data || {};
+    if (status === 401 || data.error === 'INVALID_CODE') return 'Invalid code (401) ΓÇö check the code with your class rep.';
+    if (status === 403 || data.error === 'OUT_OF_GEOFENCE') {
+      return 'Out of geofence (403)' + (data.distance_m != null ? ' ΓÇö ' + data.distance_m + ' m away.' : '.');
+    }
+    if (status === 429 || data.error === 'RATE_LIMITED') return 'Rate limited (429) ΓÇö wait a moment and retry.';
+    if (status === 410 || data.error === 'SESSION_CLOSED') return 'Session closed (410).';
+    return 'Scan failed (' + status + '): ' + (data.error || 'request failed');
+  }
+
+  function uploadError(status, data) {
+    data = data || {};
+    if (status === 413 || data.error === 'FILE_TOO_LARGE') return 'File too large (413) ΓÇö max 5 MB.';
+    if (data.error === 'INVALID_FILE_TYPE') return 'Invalid file type (400) ΓÇö pdf/jpg/png only.';
+    if (data.error === 'BAD_REASON') return 'Bad reason (400) ΓÇö pick sick, outreach, or personal.';
+    return 'Upload failed (' + status + '): ' + (data.error || 'request failed');
+  }
+
+  window.Atlas = {
+    api: api, login: login, logout: logout,
+    getToken: getToken, setToken: setToken,
+    roleHome: roleHome, requireRole: requireRole,
+    el: el, msg: msg, scanError: scanError, uploadError: uploadError
+  };
+})();
diff --git a/web/index.html b/web/index.html
new file mode 100644
index 0000000..0f4a29d
--- /dev/null
+++ b/web/index.html
@@ -0,0 +1,41 @@
+<!DOCTYPE html>
+<html lang="en">
+<head>
+<meta charset="utf-8">
+<meta name="viewport" content="width=device-width, initial-scale=1">
+<title>Atlas ΓÇö Login</title>
+<link rel="manifest" href="manifest.json">
+<style>
+body{font-family:system-ui,sans-serif;max-width:480px;margin:2rem auto;padding:0 1rem}
+label{display:block;margin:.6rem 0 .2rem}input,button{font-size:1rem;padding:.5rem;width:100%;box-sizing:border-box}
+button{margin-top:1rem}.msg{margin-top:1rem}.err{color:#b00}.ok{color:#060}
+</style>
+</head>
+<body>
+<h1>Atlas Attendance</h1>
+<form id="f">
+<label>Email <input id="email" type="email" required autocomplete="username"></label>
+<label>Password <input id="password" type="password" required autocomplete="current-password"></label>
+<button type="submit">Log in</button>
+</form>
+<p id="m" class="msg"></p>
+<script src="app.js"></script>
+<script>
+(function () {
+  // If already authed, route straight to the role page.
+  Atlas.api('GET', '/v1/me').then(function (r) {
+    if (r.status === 200 && r.data.role && Atlas.getToken()) location.href = Atlas.roleHome(r.data.role);
+  });
+  if ('serviceWorker' in navigator) navigator.serviceWorker.register('sw.js').catch(function () {});
+  document.getElementById('f').addEventListener('submit', async function (e) {
+    e.preventDefault();
+    var r = await Atlas.login(Atlas.el('email').value.trim(), Atlas.el('password').value);
+    if (!r.ok) { Atlas.msg('m', 'Login failed (' + r.status + '): ' + r.error, true); return; }
+    var me = await Atlas.api('GET', '/v1/me');
+    if (me.status !== 200) { Atlas.msg('m', 'Logged in but /v1/me failed (' + me.status + ')', true); return; }
+    location.href = Atlas.roleHome(me.data.role);
+  });
+})();
+</script>
+</body>
+</html>
diff --git a/web/manifest.json b/web/manifest.json
new file mode 100644
index 0000000..174d763
--- /dev/null
+++ b/web/manifest.json
@@ -0,0 +1,9 @@
+{
+  "name": "Atlas Attendance",
+  "short_name": "Atlas",
+  "start_url": "index.html",
+  "display": "standalone",
+  "background_color": "#ffffff",
+  "theme_color": "#0b5fff",
+  "description": "Minimal attendance PWA: rep QR, student scan, teacher review, admin classes."
+}
diff --git a/web/rep.html b/web/rep.html
new file mode 100644
index 0000000..4c5bd07
--- /dev/null
+++ b/web/rep.html
@@ -0,0 +1,114 @@
+<!DOCTYPE html>
+<html lang="en">
+<head>
+<meta charset="utf-8">
+<meta name="viewport" content="width=device-width, initial-scale=1">
+<title>Atlas ΓÇö Class Rep</title>
+<link rel="manifest" href="manifest.json">
+<style>
+body{font-family:system-ui,sans-serif;max-width:640px;margin:1rem auto;padding:0 1rem}
+select,input,button{font-size:1rem;padding:.5rem;margin:.25rem 0}button{cursor:pointer}
+#code{font-size:2.5rem;letter-spacing:.5rem;font-family:monospace}
+.msg{margin:.5rem 0}.err{color:#b00}.ok{color:#060}
+ul{padding-left:1.2rem}li{margin:.2rem 0}.top{display:flex;justify-content:space-between;align-items:baseline}
+#qrFallback{color:#b00}
+</style>
+</head>
+<body>
+<div class="top"><h1>Class Rep</h1><span><span id="who"></span> <button id="out">Logout</button></span></div>
+<label>Class <select id="cls"></select></label>
+<button id="open">Open session</button>
+<button id="close" disabled>Close session</button>
+<p>Session: <b id="sess">none</b></p>
+<div id="qr"></div>
+<p id="qrFallback"></p>
+<p>Code: <span id="code">ΓÇö</span> <span id="cd"></span></p>
+<p id="m" class="msg"></p>
+<h2>Live check-ins</h2>
+<ul id="feed"></ul>
+<script src="https://cdnjs.cloudflare.com/ajax/libs/qrcodejs/1.0.0/qrcode.min.js"></script>
+<script src="app.js"></script>
+<script>
+(function () {
+  var sessionId = 0, pollT = null, cdT = null, expAt = 0, qrObj = null;
+  Atlas.el('out').onclick = Atlas.logout;
+  Atlas.requireRole(['class_rep', 'rep', 'admin']).then(function (me) {
+    if (!me) return;
+    Atlas.el('who').textContent = me.email + ' (' + me.role + ')';
+    loadClasses();
+  });
+
+  async function loadClasses() {
+    var r = await Atlas.api('GET', '/v1/classes');
+    if (r.status !== 200) { Atlas.msg('m', 'Load classes failed (' + r.status + ')', true); return; }
+    Atlas.el('cls').innerHTML = r.data.classes.map(function (c) {
+      return '<option value="' + c.id + '">' + c.class_name + ' (#' + c.id + ')</option>';
+    }).join('');
+  }
+
+  Atlas.el('open').onclick = async function () {
+    var r = await Atlas.api('POST', '/v1/qr/session', { class_id: +Atlas.el('cls').value });
+    if (r.status !== 200) { Atlas.msg('m', 'Open session failed (' + r.status + '): ' + (r.data.error || ''), true); return; }
+    sessionId = r.data.session_id || r.data.id;
+    Atlas.el('sess').textContent = '#' + sessionId;
+    Atlas.el('close').disabled = false;
+    Atlas.msg('m', 'Session #' + sessionId + ' open', false);
+    tick(); pollFeed();
+    clearInterval(pollT); pollT = setInterval(function () { tick(); pollFeed(); }, 5000);
+  };
+
+  Atlas.el('close').onclick = async function () {
+    if (!sessionId) return;
+    var r = await Atlas.api('POST', '/v1/qr/session/' + sessionId + '/close', {});
+    clearInterval(pollT); Atlas.el('close').disabled = true;
+    Atlas.msg('m', r.status === 200 ? 'Session closed, marked_present=' + r.data.marked : 'Close failed (' + r.status + ')', r.status !== 200);
+  };
+
+  async function tick() {
+    if (!sessionId) return;
+    var r = await Atlas.api('GET', '/v1/qr/session/' + sessionId + '/code');
+    if (r.status !== 200) {
+      Atlas.msg('m', 'Code fetch failed (' + r.status + '): ' + (r.data.error || ''), true);
+      if (r.status === 410) clearInterval(pollT);
+      return;
+    }
+    var code = r.data.code, exp = r.data.exp != null ? r.data.exp : r.data.expires_at;
+    Atlas.el('code').textContent = code;
+    // API returns exp as unix seconds; tolerate RFC3339 strings too.
+    expAt = (typeof exp === 'number') ? exp * 1000 : Date.parse(exp);
+    if (!expAt) expAt = Date.now() + 30000;
+    drawQR(code);
+    clearInterval(cdT); countdown();
+    cdT = setInterval(countdown, 1000);
+  }
+
+  function countdown() {
+    var s = Math.max(0, Math.round((expAt - Date.now()) / 1000));
+    Atlas.el('cd').textContent = '(expires in ' + s + 's)';
+    if (s <= 0) clearInterval(cdT);
+  }
+
+  function drawQR(code) {
+    var box = Atlas.el('qr');
+    if (!window.QRCode) {
+      box.innerHTML = '';
+      Atlas.el('qrFallback').textContent = 'QR library offline (CDN blocked) ΓÇö type the code above.';
+      return;
+    }
+    Atlas.el('qrFallback').textContent = '';
+    if (!qrObj) { box.innerHTML = ''; qrObj = new QRCode(box, { width: 180, height: 180 }); }
+    qrObj.clear(); qrObj.makeCode(code);
+  }
+
+  async function pollFeed() {
+    if (!sessionId) return;
+    var r = await Atlas.api('GET', '/v1/attendance?session_id=' + sessionId);
+    if (r.status !== 200) return;
+    Atlas.el('feed').innerHTML = r.data.records.map(function (x) {
+      return '<li>student #' + x.student_id + ' ΓÇö ' + x.status + ' @ ' + x.recorded_at + '</li>';
+    }).join('') || '<li>(no check-ins yet)</li>';
+  }
+})();
+</script>
+</body>
+</html>
diff --git a/web/student.html b/web/student.html
new file mode 100644
index 0000000..f8f5df5
--- /dev/null
+++ b/web/student.html
@@ -0,0 +1,110 @@
+<!DOCTYPE html>
+<html lang="en">
+<head>
+<meta charset="utf-8">
+<meta name="viewport" content="width=device-width, initial-scale=1">
+<title>Atlas ΓÇö Student</title>
+<link rel="manifest" href="manifest.json">
+<style>
+body{font-family:system-ui,sans-serif;max-width:640px;margin:1rem auto;padding:0 1rem}
+input,select,button{font-size:1rem;padding:.5rem;margin:.25rem 0}button{cursor:pointer}
+.msg{margin:.5rem 0}.err{color:#b00}.ok{color:#060}
+.badge{display:inline-block;padding:.2rem .6rem;border-radius:1rem;background:#eee}
+.badge.lock{background:#cfc}.top{display:flex;justify-content:space-between;align-items:baseline}
+</style>
+</head>
+<body>
+<div class="top"><h1>Student</h1><span><span id="who"></span> <button id="out">Logout</button></span></div>
+
+<h2>Scan attendance</h2>
+<form id="scan">
+<label>Session ID <input id="sess" type="number" required></label><br>
+<label>Code <input id="code" required autocomplete="off"></label><br>
+<label>Lat <input id="lat" type="number" step="any" required></label>
+<label>Lng <input id="lng" type="number" step="any" required></label><br>
+<button type="button" id="gps">Use my location</button>
+<span id="gpsBadge" class="badge">no GPS lock</span><br>
+<button type="submit">Check in</button>
+</form>
+<p id="m" class="msg"></p>
+<h3>My status</h3>
+<ul id="mine"></ul>
+
+<h2>Absence request</h2>
+<form id="abs">
+<label>Session ID <input id="asess" type="number" required></label><br>
+<label>Reason <select id="reason"><option value="sick">sick</option><option value="outreach">outreach</option><option value="personal">personal</option></select></label><br>
+<label>Proof (pdf/jpg/png, max 5MB) <input id="file" type="file" accept=".pdf,.jpg,.jpeg,.png" required></label><br>
+<button type="submit">Upload</button>
+</form>
+<p id="m2" class="msg"></p>
+<h3>My requests</h3>
+<ul id="reqs"></ul>
+
+<script src="app.js"></script>
+<script>
+(function () {
+  Atlas.el('out').onclick = Atlas.logout;
+  Atlas.requireRole(['student']).then(function (me) {
+    if (!me) return;
+    Atlas.el('who').textContent = me.email;
+    loadMine();
+  });
+
+  Atlas.el('gps').onclick = function () {
+    if (!navigator.geolocation) { Atlas.msg('m', 'Geolocation not supported', true); return; }
+    navigator.geolocation.getCurrentPosition(function (p) {
+      Atlas.el('lat').value = p.coords.latitude.toFixed(6);
+      Atlas.el('lng').value = p.coords.longitude.toFixed(6);
+      var b = Atlas.el('gpsBadge');
+      b.textContent = 'GPS lock ┬▒' + Math.round(p.coords.accuracy) + ' m';
+      b.className = 'badge lock';
+    }, function (e) { Atlas.msg('m', 'Geolocation denied: ' + e.message, true); });
+  };
+
+  document.getElementById('scan').addEventListener('submit', async function (e) {
+    e.preventDefault();
+    var r = await Atlas.api('POST', '/v1/attendance/scan', {
+      session_id: +Atlas.el('sess').value, code: Atlas.el('code').value.trim(),
+      lat: +Atlas.el('lat').value, lng: +Atlas.el('lng').value
+    });
+    if (r.status !== 200) { Atlas.msg('m', Atlas.scanError(r.status, r.data), true); return; }
+    Atlas.msg('m', 'Checked in: ' + r.data.status + ' (' + (r.data.distance_m || 0) + ' m)', false);
+    loadMine();
+  });
+
+  async function loadMine() {
+    var sid = +Atlas.el('sess').value;
+    if (!sid) { Atlas.el('mine').innerHTML = '<li>enter a session ID above, then check in</li>'; return; }
+    var r = await Atlas.api('GET', '/v1/attendance?session_id=' + sid);
+    if (r.status !== 200) return;
+    Atlas.el('mine').innerHTML = r.data.records.map(function (x) {
+      return '<li>#' + x.student_id + ' ΓÇö ' + x.status + ' @ ' + x.recorded_at + '</li>';
+    }).join('') || '<li>(no record)</li>';
+  }
+  Atlas.el('sess').addEventListener('change', loadMine);
+
+  document.getElementById('abs').addEventListener('submit', async function (e) {
+    e.preventDefault();
+    var fd = new FormData();
+    fd.append('session_id', Atlas.el('asess').value);
+    fd.append('reason_type', Atlas.el('reason').value);
+    fd.append('file', Atlas.el('file').files[0]);
+    var r = await Atlas.api('POST', '/v1/absences', fd);
+    if (r.status !== 200) { Atlas.msg('m2', Atlas.uploadError(r.status, r.data), true); return; }
+    Atlas.msg('m2', 'Absence #' + r.data.id + ' submitted', false);
+    loadReqs();
+  });
+
+  async function loadReqs() {
+    var r = await Atlas.api('GET', '/v1/absences');
+    if (r.status !== 200) return;
+    Atlas.el('reqs').innerHTML = r.data.requests.map(function (x) {
+      return '<li>#' + x.id + ' session #' + x.session_id + ' ' + x.reason_type + ' ΓÇö ' + x.status + '</li>';
+    }).join('') || '<li>(none)</li>';
+  }
+  loadReqs();
+})();
+</script>
+</body>
+</html>
diff --git a/web/sw.js b/web/sw.js
new file mode 100644
index 0000000..206fb38
--- /dev/null
+++ b/web/sw.js
@@ -0,0 +1,34 @@
+/* Atlas service worker ΓÇö cache the app shell offline. */
+var CACHE = 'atlas-v1';
+var SHELL = [
+  'index.html', 'rep.html', 'student.html', 'teacher.html', 'admin.html',
+  'app.js', 'manifest.json'
+];
+
+self.addEventListener('install', function (e) {
+  e.waitUntil(caches.open(CACHE).then(function (c) { return c.addAll(SHELL); }));
+});
+
+self.addEventListener('activate', function (e) {
+  e.waitUntil(
+    caches.keys().then(function (keys) {
+      return Promise.all(keys.filter(function (k) { return k !== CACHE; }).map(function (k) { return caches.delete(k); }));
+    })
+  );
+});
+
+self.addEventListener('fetch', function (e) {
+  if (e.request.method !== 'GET') return;
+  var url = new URL(e.request.url);
+  if (url.origin !== location.origin) return; // leave CDN QR lib to network
+  if (url.pathname.indexOf('/v1/') === 0 || url.pathname.indexOf('/uploads/') === 0) return; // API + docs stay live
+  e.respondWith(
+    caches.match(e.request).then(function (hit) {
+      return hit || fetch(e.request).then(function (res) {
+        var copy = res.clone();
+        caches.open(CACHE).then(function (c) { c.put(e.request, copy); });
+        return res;
+      });
+    })
+  );
+});
diff --git a/web/teacher.html b/web/teacher.html
new file mode 100644
index 0000000..c2ae557
--- /dev/null
+++ b/web/teacher.html
@@ -0,0 +1,88 @@
+<!DOCTYPE html>
+<html lang="en">
+<head>
+<meta charset="utf-8">
+<meta name="viewport" content="width=device-width, initial-scale=1">
+<title>Atlas ΓÇö Teacher</title>
+<link rel="manifest" href="manifest.json">
+<style>
+body{font-family:system-ui,sans-serif;max-width:720px;margin:1rem auto;padding:0 1rem}
+input,button{font-size:1rem;padding:.5rem;margin:.25rem}button{cursor:pointer}
+.msg{margin:.5rem 0}.err{color:#b00}.ok{color:#060}
+.card{border:1px solid #ccc;border-radius:.5rem;padding:.75rem;margin:.5rem 0}
+iframe{width:100%;height:280px;border:1px solid #999}
+.top{display:flex;justify-content:space-between;align-items:baseline}
+.cards{display:flex;gap:.75rem;flex-wrap:wrap}.rate{border:1px solid #999;padding:.5rem 1rem;border-radius:.5rem}
+</style>
+</head>
+<body>
+<div class="top"><h1>Teacher</h1><span><span id="who"></span> <button id="out">Logout</button></span></div>
+
+<h2>Attendance rate</h2>
+<label>Session ID <input id="sess" type="number"></label>
+<button id="rate">Load rate</button>
+<div class="cards" id="rates"></div>
+<p id="m" class="msg"></p>
+
+<h2>Absence queue (pending)</h2>
+<button id="queue">Refresh queue</button>
+<div id="q"></div>
+
+<script src="app.js"></script>
+<script>
+(function () {
+  Atlas.el('out').onclick = Atlas.logout;
+  Atlas.requireRole(['teacher', 'admin']).then(function (me) {
+    if (!me) return;
+    Atlas.el('who').textContent = me.email + ' (' + me.role + ')';
+    loadQueue();
+  });
+
+  Atlas.el('rate').onclick = async function () {
+    var sid = +Atlas.el('sess').value;
+    if (!sid) { Atlas.msg('m', 'Enter a session ID', true); return; }
+    var r = await Atlas.api('GET', '/v1/analytics/class?session_id=' + sid);
+    if (r.status !== 200) { Atlas.msg('m', 'Rate load failed (' + r.status + '): ' + (r.data.error || ''), true); return; }
+    var cats = (r.data.categories || []).map(function (c) { return c.reason + ': ' + c.count; }).join(', ');
+    Atlas.el('rates').innerHTML =
+      '<div class="rate">present <b>' + r.data.rate.present + '</b> / ' + r.data.rate.total +
+      ' (' + r.data.rate.pct + '%)</div>' +
+      '<div class="rate">absence reasons: ' + (cats || 'none') + '</div>';
+  };
+
+  Atlas.el('queue').onclick = loadQueue;
+
+  async function loadQueue() {
+    var r = await Atlas.api('GET', '/v1/absences?status=pending');
+    if (r.status !== 200) { Atlas.msg('m', 'Queue load failed (' + r.status + ')', true); return; }
+    var box = Atlas.el('q');
+    if (!r.data.requests.length) { box.innerHTML = '<p>(queue empty)</p>'; return; }
+    box.innerHTML = '';
+    r.data.requests.forEach(function (x) {
+      var d = document.createElement('div');
+      d.className = 'card';
+      d.innerHTML = '<p><b>#' + x.id + '</b> student #' + x.student_id + ' session #' + x.session_id +
+        ' ΓÇö ' + x.reason_type + ' (' + x.status + ')</p>';
+      var f = document.createElement('iframe');
+      f.src = x.proof_file_url;
+      f.title = 'proof doc #' + x.id;
+      d.appendChild(f);
+      [['approved', 'Approve'], ['rejected', 'Reject']].forEach(function (pair) {
+        var b = document.createElement('button');
+        b.textContent = pair[1];
+        b.onclick = function () { review(x.id, pair[0]); };
+        d.appendChild(b);
+      });
+      box.appendChild(d);
+    });
+  }
+
+  async function review(id, decision) {
+    var r = await Atlas.api('PATCH', '/v1/absences/' + id + '/review', { decision: decision });
+    Atlas.msg('m', r.status === 200 ? 'Absence #' + id + ' ' + r.data.status : 'Review failed (' + r.status + '): ' + (r.data.error || ''), r.status !== 200);
+    if (r.status === 200) loadQueue();
+  }
+})();
+</script>
+</body>
+</html>
 web/.gitkeep      |   0
 web/admin.html    |  95 ++++++++++++++++++++++++++++++++++++++++++
 web/app.js        | 121 ++++++++++++++++++++++++++++++++++++++++++++++++++++++
 web/index.html    |  41 ++++++++++++++++++
 web/manifest.json |   9 ++++
 web/rep.html      | 114 ++++++++++++++++++++++++++++++++++++++++++++++++++
 web/student.html  | 110 +++++++++++++++++++++++++++++++++++++++++++++++++
 web/sw.js         |  34 +++++++++++++++
 web/teacher.html  |  88 +++++++++++++++++++++++++++++++++++++++
 9 files changed, 612 insertions(+)
