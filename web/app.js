/* Atlas shared frontend helper — vanilla JS, no build step.
 * Same-origin API (nginx proxies /v1/ to the Go API).
 * Token: localStorage 'atlas_token'. 401 with a stored token -> back to login.
 */
(function () {
  'use strict';

  var TOKEN_KEY = 'atlas_token';

  function getToken() {
    try { return localStorage.getItem(TOKEN_KEY) || ''; } catch (e) { return ''; }
  }
  function setToken(t) {
    try {
      if (t) localStorage.setItem(TOKEN_KEY, t);
      else localStorage.removeItem(TOKEN_KEY);
    } catch (e) { /* storage unavailable */ }
  }

  // Low-level call. Returns {status, data}. Never throws on HTTP errors.
  async function api(method, path, body, opts) {
    opts = opts || {};
    var headers = {};
    var tok = getToken();
    if (tok) headers['Authorization'] = 'Bearer ' + tok;
    var payload;
    if (body instanceof FormData) {
      payload = body; // browser sets multipart boundary
    } else if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
      payload = JSON.stringify(body);
    }
    var res;
    try {
      res = await fetch(path, { method: method, headers: headers, body: payload });
    } catch (e) {
      return { status: 0, data: { error: 'NETWORK_ERROR' } };
    }
    var data = null;
    try { data = await res.json(); } catch (e) { data = null; }
    return { status: res.status, data: data || {} };
  }

  async function login(email, password) {
    var r = await api('POST', '/v1/auth/login', { email: email, password: password });
    if (r.status === 200 && r.data.token) {
      setToken(r.data.token);
      return { ok: true };
    }
    return { ok: false, status: r.status, error: r.data.error || 'LOGIN_FAILED' };
  }

  function logout() {
    setToken('');
    location.href = 'index.html';
  }

  function roleHome(role) {
    if (role === 'admin') return 'admin.html';
    if (role === 'teacher') return 'teacher.html';
    if (role === 'class_rep' || role === 'rep') return 'rep.html';
    if (role === 'student') return 'student.html';
    return 'index.html';
  }

  // Page guard: require a stored token + one of roles. Resolves with /v1/me.
  // Redirects to index.html on missing/invalid token or wrong role.
  async function requireRole(allowed) {
    if (!getToken()) {
      location.href = 'index.html';
      return null;
    }
    var r = await api('GET', '/v1/me');
    if (r.status === 401 || r.status === 403 || r.status === 0) {
      setToken('');
      location.href = 'index.html';
      return null;
    }
    if (r.status !== 200 || !r.data || allowed.indexOf(r.data.role) === -1) {
      if (r.data && r.data.role) location.href = roleHome(r.data.role);
      else location.href = 'index.html';
      return null;
    }
    return r.data;
  }

  function el(id) { return document.getElementById(id); }
  function msg(id, text, isErr) {
    var n = el(id);
    if (!n) return;
    n.textContent = text || '';
    n.className = text ? (isErr ? 'msg err' : 'msg ok') : 'msg';
  }

  // Distinct scan-error wording per API contract.
  function scanError(status, data) {
    data = data || {};
    if (status === 401 || data.error === 'INVALID_CODE') return 'Invalid code (401) — check the code with your class rep.';
    if (status === 403 || data.error === 'OUT_OF_GEOFENCE') {
      return 'Out of geofence (403)' + (data.distance_m != null ? ' — ' + data.distance_m + ' m away.' : '.');
    }
    if (status === 429 || data.error === 'RATE_LIMITED') return 'Rate limited (429) — wait a moment and retry.';
    if (status === 410 || data.error === 'SESSION_CLOSED') return 'Session closed (410).';
    return 'Scan failed (' + status + '): ' + (data.error || 'request failed');
  }

  function uploadError(status, data) {
    data = data || {};
    if (status === 413 || data.error === 'FILE_TOO_LARGE') return 'File too large (413) — max 5 MB.';
    if (data.error === 'INVALID_FILE_TYPE') return 'Invalid file type (400) — pdf/jpg/png only.';
    if (data.error === 'BAD_REASON') return 'Bad reason (400) — pick sick, outreach, or personal.';
    return 'Upload failed (' + status + '): ' + (data.error || 'request failed');
  }

  window.Atlas = {
    api: api, login: login, logout: logout,
    getToken: getToken, setToken: setToken,
    roleHome: roleHome, requireRole: requireRole,
    el: el, msg: msg, scanError: scanError, uploadError: uploadError
  };
})();
