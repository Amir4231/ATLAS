/* Atlas service worker — cache the app shell offline. */
var CACHE = 'atlas-v1';
var SHELL = [
  'index.html', 'rep.html', 'student.html', 'teacher.html', 'admin.html',
  'app.js', 'manifest.json'
];

self.addEventListener('install', function (e) {
  e.waitUntil(caches.open(CACHE).then(function (c) { return c.addAll(SHELL); }));
});

self.addEventListener('activate', function (e) {
  e.waitUntil(
    caches.keys().then(function (keys) {
      return Promise.all(keys.filter(function (k) { return k !== CACHE; }).map(function (k) { return caches.delete(k); }));
    })
  );
});

self.addEventListener('fetch', function (e) {
  if (e.request.method !== 'GET') return;
  var url = new URL(e.request.url);
  if (url.origin !== location.origin) return; // leave CDN QR lib to network
  if (url.pathname.indexOf('/v1/') === 0 || url.pathname.indexOf('/uploads/') === 0) return; // API + docs stay live
  e.respondWith(
    caches.match(e.request).then(function (hit) {
      return hit || fetch(e.request).then(function (res) {
        var copy = res.clone();
        caches.open(CACHE).then(function (c) { c.put(e.request, copy); });
        return res;
      });
    })
  );
});
