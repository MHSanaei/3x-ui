var self = this;

var filesToCache = [
    '/'
];

self.addEventListener('install', function (e) {
    e.waitUntil(
        caches.open('3xPanel').then(function (cache) {
            return cache.addAll(filesToCache);
        })
    );
});
self.addEventListener('activate', function (event) {
    event.waitUntil(
        caches.keys().then(function (cacheNames) {
            return Promise.all(
                cacheNames.filter(function (cacheName) {
                }).map(function (cacheName) {
                    return caches.delete(cacheName);
                })
            );
        })
    );
});
self.addEventListener('fetch', function (event) {
    event.respondWith(
        caches.open('mysite-dynamic').then(function (cache) {
            return cache.match(event.request).then(function (response) {
                return response || fetch(event.request).then(function (response) {
                    cache.put(event.request, response.clone());
                    return response;
                });
            });
        })
    );
});

console.clear();