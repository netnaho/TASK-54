/**
 * DeviceCache — client-side LRU cache for exercise content and media.
 *
 * Storage strategy:
 *   metadata (LRU index, per-item size estimates): localStorage key "careops_cache_meta"
 *   exercise JSON: localStorage keys "careops_ex_<id>"
 *   media blobs:   IndexedDB database "careops_media_cache", store "blobs"
 *
 * Limits:
 *   MAX_ITEMS: 200 (item count cap across all cached content)
 *   MAX_BYTES: 2 * 1024 * 1024 * 1024 (2 GB estimated size cap)
 *
 * Size estimation:
 *   exercise JSON items: JSON.stringify(data).length bytes (JS string char count; each JS char
 *   is up to 2 UTF-16 bytes, so this is a conservative upper bound for ASCII-heavy content).
 *   media blobs: blob.size bytes (exact, as reported by the Blob API).
 *
 * LRU eviction:
 *   On every write, if total count > MAX_ITEMS or total estimated bytes > MAX_BYTES,
 *   the least-recently-accessed item(s) are evicted until limits are satisfied.
 *   Access order is tracked by "lastAccessed" timestamp in the metadata store.
 *
 * Shared workstation guidance:
 *   clearAll() removes all metadata and all IndexedDB entries.
 *   The cache is tied to the browser profile, not the CareOps session.
 *   Signing out of CareOps does NOT clear the cache.
 *   Staff MUST click "Clear Device Cache" before leaving a shared workstation
 *   to prevent the next user from reading their cached exercise data.
 *
 * Honest limitations:
 *   - IndexedDB storage quota varies by browser and OS (typically 20–60% of free disk).
 *     Browser may reject writes before this module's MAX_BYTES limit is reached.
 *     When a write is rejected (QuotaExceededError), the module logs a warning and skips
 *     caching that item; the feature degrades gracefully to a no-cache mode.
 *   - Size estimates for JSON items are character counts, not exact byte sizes.
 *   - Private / incognito windows have ephemeral storage that clears on close.
 *   - This module does not encrypt cached content. Do not rely on it for PII protection.
 *     The server applies masking before serving non-finance views; cached data reflects
 *     whatever the server returned.
 */

const DeviceCache = (function () {
  'use strict';

  const MAX_ITEMS = 200;
  const MAX_BYTES = 2 * 1024 * 1024 * 1024; // 2 GB
  const META_KEY  = 'careops_cache_meta';
  const EX_PREFIX = 'careops_ex_';
  const IDB_NAME  = 'careops_media_cache';
  const IDB_STORE = 'blobs';
  const IDB_VER   = 1;

  // ── Metadata helpers ───────────────────────────────────────────────────────

  function loadMeta() {
    try {
      var raw = localStorage.getItem(META_KEY);
      return raw ? JSON.parse(raw) : { items: {}, totalBytes: 0, count: 0 };
    } catch (_) {
      return { items: {}, totalBytes: 0, count: 0 };
    }
  }

  function saveMeta(meta) {
    try { localStorage.setItem(META_KEY, JSON.stringify(meta)); } catch (_) {}
  }

  /** Touch access time and return updated meta. Does NOT save. */
  function touch(meta, key) {
    if (meta.items[key]) {
      meta.items[key].lastAccessed = Date.now();
    }
    return meta;
  }

  /** Register a new or updated item in meta. Does NOT save. */
  function register(meta, key, sizeBytes) {
    var existing = meta.items[key];
    if (existing) {
      meta.totalBytes -= existing.size;
    } else {
      meta.count++;
    }
    meta.items[key] = { size: sizeBytes, lastAccessed: Date.now() };
    meta.totalBytes += sizeBytes;
    return meta;
  }

  /** Remove an item from meta. Does NOT save. */
  function unregister(meta, key) {
    var item = meta.items[key];
    if (!item) return meta;
    meta.totalBytes -= item.size;
    meta.count--;
    delete meta.items[key];
    return meta;
  }

  /**
   * Evict the least-recently-accessed items until count <= MAX_ITEMS
   * and totalBytes <= MAX_BYTES. Returns list of evicted keys.
   */
  function evictIfNeeded(meta) {
    var evicted = [];
    while (meta.count > MAX_ITEMS || meta.totalBytes > MAX_BYTES) {
      // Find LRU key
      var lruKey = null;
      var lruTime = Infinity;
      for (var k in meta.items) {
        if (meta.items[k].lastAccessed < lruTime) {
          lruTime = meta.items[k].lastAccessed;
          lruKey = k;
        }
      }
      if (!lruKey) break; // safety
      evicted.push(lruKey);
      meta = unregister(meta, lruKey);
    }
    return { meta: meta, evicted: evicted };
  }

  // ── IndexedDB helpers ──────────────────────────────────────────────────────

  function openIDB() {
    return new Promise(function (resolve, reject) {
      if (!window.indexedDB) { reject(new Error('IndexedDB not available')); return; }
      var req = indexedDB.open(IDB_NAME, IDB_VER);
      req.onupgradeneeded = function (e) {
        e.target.result.createObjectStore(IDB_STORE);
      };
      req.onsuccess = function (e) { resolve(e.target.result); };
      req.onerror   = function (e) { reject(e.target.error); };
    });
  }

  function idbPut(key, value) {
    return openIDB().then(function (db) {
      return new Promise(function (resolve, reject) {
        var tx  = db.transaction(IDB_STORE, 'readwrite');
        var req = tx.objectStore(IDB_STORE).put(value, key);
        req.onsuccess = function () { resolve(); };
        req.onerror   = function (e) { reject(e.target.error); };
      });
    });
  }

  function idbGet(key) {
    return openIDB().then(function (db) {
      return new Promise(function (resolve, reject) {
        var tx  = db.transaction(IDB_STORE, 'readonly');
        var req = tx.objectStore(IDB_STORE).get(key);
        req.onsuccess = function (e) { resolve(e.target.result || null); };
        req.onerror   = function (e) { reject(e.target.error); };
      });
    });
  }

  function idbDelete(key) {
    return openIDB().then(function (db) {
      return new Promise(function (resolve, reject) {
        var tx  = db.transaction(IDB_STORE, 'readwrite');
        var req = tx.objectStore(IDB_STORE).delete(key);
        req.onsuccess = function () { resolve(); };
        req.onerror   = function (e) { reject(e.target.error); };
      });
    });
  }

  function idbClearAll() {
    return openIDB().then(function (db) {
      return new Promise(function (resolve, reject) {
        var tx  = db.transaction(IDB_STORE, 'readwrite');
        var req = tx.objectStore(IDB_STORE).clear();
        req.onsuccess = function () { resolve(); };
        req.onerror   = function (e) { reject(e.target.error); };
      });
    });
  }

  // ── Public API ─────────────────────────────────────────────────────────────

  /**
   * Cache a serialisable exercise object (JSON). Evicts LRU if limits exceeded.
   * @param {string} id - Exercise ID
   * @param {object} data - Plain object to serialise
   */
  function cacheExercise(id, data) {
    var key     = EX_PREFIX + id;
    var payload = JSON.stringify(data);
    var size    = payload.length; // char count as byte estimate

    var meta = loadMeta();
    meta     = register(meta, key, size);
    var r    = evictIfNeeded(meta);
    meta     = r.meta;

    // Remove evicted localStorage items
    r.evicted.forEach(function (k) {
      if (k.indexOf(EX_PREFIX) === 0) {
        try { localStorage.removeItem(k); } catch (_) {}
      }
    });

    saveMeta(meta);
    try { localStorage.setItem(key, payload); } catch (e) {
      // QuotaExceededError — degrade gracefully
      console.warn('DeviceCache: localStorage quota exceeded; skipping cache for', id, e);
      meta = unregister(meta, key);
      saveMeta(meta);
    }
  }

  /**
   * Retrieve a cached exercise. Returns the parsed object or null.
   * @param {string} id
   * @returns {object|null}
   */
  function getExercise(id) {
    var key = EX_PREFIX + id;
    try {
      var raw = localStorage.getItem(key);
      if (!raw) return null;
      var meta = touch(loadMeta(), key);
      saveMeta(meta);
      return JSON.parse(raw);
    } catch (_) {
      return null;
    }
  }

  /**
   * Cache a media Blob (image or video).
   * @param {string} filename - The bare filename (as in /media/:filename)
   * @param {Blob}   blob
   * @returns {Promise<void>}
   */
  function cacheMedia(filename, blob) {
    var key  = 'media_' + filename;
    var size = blob.size;

    var meta = loadMeta();
    meta = register(meta, key, size);
    var r = evictIfNeeded(meta);
    meta = r.meta;

    // Remove evicted blobs from IndexedDB
    var deletePromises = r.evicted
      .filter(function (k) { return k.indexOf('media_') === 0; })
      .map(function (k) { return idbDelete(k).catch(function () {}); });

    saveMeta(meta);

    return Promise.all(deletePromises).then(function () {
      return idbPut(key, blob).catch(function (e) {
        console.warn('DeviceCache: IndexedDB write failed for', filename, e);
        meta = unregister(meta, key);
        saveMeta(meta);
      });
    });
  }

  /**
   * Retrieve a cached media Blob.
   * @param {string} filename
   * @returns {Promise<Blob|null>}
   */
  function getMedia(filename) {
    var key = 'media_' + filename;
    return idbGet(key).then(function (blob) {
      if (!blob) return null;
      var meta = touch(loadMeta(), key);
      saveMeta(meta);
      return blob;
    }).catch(function () { return null; });
  }

  /**
   * Remove all cached data (exercises + media) and reset metadata.
   * Call before leaving a shared workstation.
   * @returns {Promise<void>}
   */
  function clearAll() {
    // Clear localStorage exercise entries
    var meta = loadMeta();
    Object.keys(meta.items).forEach(function (k) {
      if (k.indexOf(EX_PREFIX) === 0) {
        try { localStorage.removeItem(k); } catch (_) {}
      }
    });
    try { localStorage.removeItem(META_KEY); } catch (_) {}

    // Clear IndexedDB
    return idbClearAll().catch(function () {});
  }

  /**
   * Returns current cache statistics.
   * @returns {Promise<{count: number, totalBytes: number, sizeMB: string}>}
   */
  function getStats() {
    return Promise.resolve().then(function () {
      var meta = loadMeta();
      return {
        count:      meta.count,
        totalBytes: meta.totalBytes,
        sizeMB:     (meta.totalBytes / (1024 * 1024)).toFixed(2)
      };
    });
  }

  return { cacheExercise, getExercise, cacheMedia, getMedia, clearAll, getStats };
}());
