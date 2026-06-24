# Cross-Browser Data Stores: Comparison & Normalization Guide

## Scope

This document covers three things in one place:

1. **What data is common** across Chrome, Safari, and Firefox — the intersection that a
   browser-agnostic tool can rely on.
2. **How to normalize visit data** from all three into a single unified model, with
   per-browser SQL extraction queries.
3. **What data is unique** to each browser — the stores and fields with no equivalent
   elsewhere.

The per-browser deep dives are `chrome-internal-browser-data-stores.md`,
`safari-internal-browser-data-stores.md`, and `firefox-internal-browser-data-stores.md`.

---

## 1. Shared architectural pattern

All three browsers distribute state across multiple artifact types (SQLite, JSON, plist,
proprietary binary) rather than centralizing it in one database. But the history **core is
identical** in concept: one row per distinct URL in a catalog table, linked to a separate
visit-events table by foreign key.

| Concept | Chrome (`History`) | Safari (`History.db`) | Firefox (`places.sqlite`) |
|---|---|---|---|
| URL catalog table | `urls` | `history_items` | `moz_places` |
| Visit-events table | `visits` | `history_visits` | `moz_historyvisits` |
| Catalog → visit FK | `visits.url` → `urls.id` | `history_visits.history_item` → `history_items.id` | `moz_historyvisits.place_id` → `moz_places.id` |

---

## 2. Common data (present in all three browsers)

### Browsing history

| Field | Chrome | Safari | Firefox |
|---|---|---|---|
| URL string | `urls.url` | `history_items.url` | `moz_places.url` |
| Page title | `urls.title` | `history_visits.title` ¹ | `moz_places.title` |
| Total visit count | `urls.visit_count` | `history_items.visit_count` | `moz_places.visit_count` |
| Per-visit timestamp | `visits.visit_time` | `history_visits.visit_time` | `moz_historyvisits.visit_date` |
| Last visit timestamp | `urls.last_visit_time` | *derive: `MAX(visit_time)`* | `moz_places.last_visit_date` |

> ¹ Safari stores the title on the **visit** row, not the URL row — use the title from
> the most recent visit as the canonical page title.

### Navigation chains (referrers)

All three record the referring visit with a self-join on the visits table:

| | Chrome | Safari | Firefox |
|---|---|---|---|
| Referring-visit link | `visits.from_visit` → `visits.id` | `history_visits.redirect_source` / `redirect_destination` (self FK) | `moz_historyvisits.from_visit` → self |

### Visit type (coarse)

A **link / typed / reload / redirect** distinction is recoverable everywhere; finer
categories are browser-specific.

| | Chrome | Safari | Firefox |
|---|---|---|---|
| Type encoding | `visits.transition` bitmask (0 LINK, 1 TYPED, 5 RELOAD…) | boolean flags: `http_non_get`, `synthesized` (no enum) | `moz_historyvisits.visit_type` enum (1 LINK, 2 TYPED, 5/6 REDIRECT, 9 RELOAD…) |
| "User typed it" | `urls.typed_count` | `autocomplete_triggers` (BLOB) | `moz_places.typed` |

### Bookmarks

All three persist a folder/URL tree with timestamps — in different serialization formats.

| | Chrome | Safari | Firefox |
|---|---|---|---|
| Store | `Bookmarks` (JSON) | `Bookmarks.plist` (binary plist) | `moz_bookmarks` (SQLite) |
| Tree shape | `roots` → folders → nodes with `children` | `WebBookmarkTypeList` / `WebBookmarkTypeLeaf` | self-referential `parent` FK |
| Per-node fields | `type`, `name`, `url`, `guid`, `date_added` | `URLString`, `URIDictionary.title` | `type`, `title`, `fk`→`moz_places`, `dateAdded` |

### Open tabs & session restore

All three survive a restart, but none use a plain SQL table:

| | Chrome | Safari | Firefox |
|---|---|---|---|
| Store | `Sessions/Session_*` + `Tabs_*` (SNSS binary) | session plists + `RecentlyClosedTabs.plist` | `sessionstore.jsonlz4` + `sessionstore-backups/` (mozLz4) |
| Recoverable | window/tab structure + back/forward URLs | open & recently closed tabs | windows → tabs → `entries[]` nav list |

### Downloads

| | Chrome | Safari | Firefox |
|---|---|---|---|
| Store | `downloads` table in `History` | `Downloads.plist` | visit with `visit_type` = 7 (DOWNLOAD) |
| Common fields | source URL, target path, byte counters, timestamps | source URL, destination, byte counters | page URL, visit time |

---

## 3. The timestamp problem (critical for normalization)

Every browser uses a different epoch. Any unified model must convert to a single
reference — Unix microseconds since 1970-01-01 UTC is the natural choice:

| Browser | Stored unit | Conversion to Unix µs |
|---|---|---|
| Chrome | µs since **1601-01-01** | `value - 11644473600000000` |
| Safari | seconds since **2001-01-01** | `(value + 978307200) * 1000000` |
| Firefox | µs since **1970-01-01** | `value` (no conversion) |

---

## 4. Normalized visit model

### Target schema

```sql
CREATE TABLE norm_visits (
    id           TEXT    PRIMARY KEY,  -- '{browser}:{original_id}'
    browser      TEXT    NOT NULL,     -- 'chrome' | 'safari' | 'firefox'
    url          TEXT    NOT NULL,
    title        TEXT,
    visited_at   INTEGER NOT NULL,     -- Unix µs since 1970-01-01 UTC
    visit_type   TEXT,                 -- 'link'|'typed'|'bookmark'|'reload'|'redirect'|'download'|'other'
    referrer_url TEXT,                 -- resolved URL of the referring page (NULL if unknown)
    duration_us  INTEGER,              -- dwell time in µs (Chrome only; NULL for Safari/Firefox)
    source       TEXT                  -- 'local' | 'synced' | NULL
);
```

### Chrome extraction query

Run against `History`. Resolves the referrer by self-joining `visits` and decodes the
`transition` bitmask to a coarse visit type.

```sql
SELECT
    'chrome:' || v.id                          AS id,
    'chrome'                                   AS browser,
    u.url,
    u.title,
    -- µs since 1601 → µs since 1970
    (v.visit_time - 11644473600000000)         AS visited_at,
    CASE v.transition & 0xff
        WHEN 0  THEN 'link'
        WHEN 1  THEN 'typed'
        WHEN 2  THEN 'bookmark'
        WHEN 5  THEN 'reload'
        WHEN 6  THEN 'redirect'
        WHEN 7  THEN 'download'
        ELSE         'other'
    END                                        AS visit_type,
    ref_u.url                                  AS referrer_url,
    v.visit_duration                           AS duration_us,
    CASE WHEN vs.source = 0 THEN 'synced'
         ELSE 'local' END                      AS source
FROM visits v
JOIN urls u               ON u.id    = v.url
LEFT JOIN visits      pv  ON pv.id   = v.from_visit
LEFT JOIN urls     ref_u  ON ref_u.id = pv.url
LEFT JOIN visit_source vs ON vs.id   = v.id
WHERE u.hidden = 0;   -- exclude subframe-only URLs
```

### Safari extraction query

Run against `History.db`. Safari has no visit-type enum, so `redirect` is inferred from
the self-referential `redirect_source`/`redirect_destination` columns.

```sql
SELECT
    'safari:' || v.id                          AS id,
    'safari'                                   AS browser,
    i.url,
    -- title lives on the visit row in Safari
    v.title,
    -- seconds since 2001 → µs since 1970
    CAST((v.visit_time + 978307200) * 1000000 AS INTEGER) AS visited_at,
    CASE
        WHEN v.redirect_source IS NOT NULL
          OR v.redirect_destination IS NOT NULL  THEN 'redirect'
        WHEN v.synthesized = 1                   THEN 'other'
        WHEN v.http_non_get = 1                  THEN 'other'
        ELSE                                          'link'
    END                                        AS visit_type,
    ref_i.url                                  AS referrer_url,
    NULL                                       AS duration_us,
    CASE WHEN v.origin = 1 THEN 'synced'
         ELSE 'local' END                      AS source
FROM history_visits v
JOIN history_items  i     ON i.id   = v.history_item
LEFT JOIN history_visits rv    ON rv.id  = v.redirect_source
LEFT JOIN history_items  ref_i ON ref_i.id = rv.history_item
WHERE v.load_successful = 1;   -- exclude failed loads
```

### Firefox extraction query

Run against `places.sqlite`. `visit_type` maps cleanly to the normalized enum. Both
permanent and temporary redirects collapse to `'redirect'`.

```sql
SELECT
    'firefox:' || hv.id                        AS id,
    'firefox'                                  AS browser,
    p.url,
    p.title,
    -- µs since 1970 — no conversion needed
    hv.visit_date                              AS visited_at,
    CASE hv.visit_type
        WHEN 1  THEN 'link'
        WHEN 2  THEN 'typed'
        WHEN 3  THEN 'bookmark'
        WHEN 5  THEN 'redirect'
        WHEN 6  THEN 'redirect'
        WHEN 7  THEN 'download'
        WHEN 9  THEN 'reload'
        ELSE         'other'
    END                                        AS visit_type,
    ref_p.url                                  AS referrer_url,
    NULL                                       AS duration_us,
    CASE WHEN hv.source = 1 THEN 'synced'
         ELSE 'local' END                      AS source
FROM moz_historyvisits hv
JOIN moz_places p          ON p.id    = hv.place_id
LEFT JOIN moz_historyvisits pv    ON pv.id    = hv.from_visit
LEFT JOIN moz_places       ref_p  ON ref_p.id = pv.place_id
WHERE p.hidden = 0;   -- exclude subframe-only URLs
```

### Caveats & known limits

| Caveat | Detail |
|---|---|
| Safari `visit_type` | Only `redirect` and `other` can be inferred; `typed` vs. `link` vs. `bookmark` is not distinguishable without the address-bar history (not stored in `History.db`). |
| Safari title | `v.title` can be NULL for redirected or failed visits; fall back to a prior visit's title for the same URL if needed. |
| `duration_us` | Only Chrome records dwell time (`visit_duration`). Safari and Firefox return NULL. |
| `referrer_url` | For Chrome the referrer is always the URL of the *previous visit* (`from_visit` → `urls`). Safari's `redirect_source` only covers HTTP redirects, not click-navigation. Firefox's `from_visit` behaves like Chrome. |
| Hidden/framed URLs | All three have a mechanism to exclude sub-frame-only or hidden URLs (`urls.hidden`, `moz_places.hidden`). The queries above filter them out. Safari has no equivalent flag, so all visits are included. |

---

## 5. Unique browser data

### 5.1 Chrome — exclusive stores & fields

**`History` database extras**

| Table | What it adds |
|---|---|
| `segments` / `segment_usage` | Representative-URL grouping with per-time-slot visit counts that drive most-visited ranking. |
| `clusters` / `clusters_and_visits` / `cluster_keywords` | **"Journeys"** — ML-grouped visit clusters with labels, scores, and keywords. |
| `content_annotations` | Per-visit page intelligence: `visibility_score`, `categories`, `page_language`, `entities`, `related_searches`. |
| `context_annotations` | Per-visit UI context: `window_id`, `tab_id`, `task_id`, `browser_type`, `page_end_reason`, foreground duration. |
| `visited_links` | Partitioned `:visited` link-styling graph (`link_url_id`, `top_level_url`, `frame_url`). |
| `keyword_search_terms` | Dedicated search-terms table mapping each query to the resulting URL. |
| Rich `visits` provenance | `originator_*` columns (sync device of origin), `opener_visit`, `app_id` (originating PWA). |

**Separate single-purpose databases**

| Store | Holds |
|---|---|
| `Top Sites` | New-tab most-visited tiles (`url`, `url_rank`, `title`). |
| `Shortcuts` | Omnibox shortcut learning (`omni_box_shortcuts`). |
| `Network Action Predictor` | Typed-text → URL prediction for preconnect/prerender (hits/misses). |
| `Web Data` | Configured search engines, form autofill, addresses, credit cards. |

**Sync & tab-group state (`Sync Data/LevelDB`)**

| Sync entity | Holds |
|---|---|
| `Sessions` (foreign sessions) | Open tabs of other signed-in devices. |
| `SavedTabGroup` / `SavedTabGroupTab` | Saved & synced tab groups and their member tabs. |
| `ReadingList` | Reading-list entries (URL, title, read/unread). |

---

### 5.2 Safari — exclusive stores & fields

**`History.db` extras**

| Field / table | What it adds |
|---|---|
| `history_items.domain_expansion` | "Important" host fragment for autocomplete (e.g. `apple` for `apple.com`). |
| `history_items.daily_visit_counts` / `weekly_visit_counts` | Packed binary visit histograms (per-day / per-week buckets, not SQL-queryable). |
| `history_visits.load_successful` / `http_non_get` / `synthesized` | Per-visit load result, non-GET flag, and browser-generated marker. |
| `history_tombstones` | Records of deletions over a time range for sync reconciliation. |
| `history_tags` / `history_items_to_tags` | Tagging system for history items, with `item_count` maintained by SQL triggers. |
| SQL triggers | Safari is the only one of the three with triggers defined in the history schema. |

**Property-list & auxiliary stores**

| Store | Holds |
|---|---|
| `Downloads.plist` | Download records (Safari keeps these out of `History.db`). |
| `RecentlyClosedTabs.plist` | Recently closed tabs/windows for "Reopen". |
| `TopSites.plist` | Pinned / most-visited start-page tiles. |
| `PerSitePreferences.db` | Per-site settings: zoom, autoplay, camera/mic, content-blocker overrides. |
| `ContentBlockerStatistics.db` | Counters of trackers/resources blocked. |
| `IgnoredSiriSuggestedSites.db` | Sites excluded from Siri suggestions. |

**Reading List, Tab Groups & iCloud tabs**

| Item | Where |
|---|---|
| **Reading List** | `Bookmarks.plist` — list titled `com.apple.ReadingList`; leaves carry `DateAdded`, `DateLastViewed`, `PreviewText`. |
| Reading-list offline archives | `ReadingListArchives/<UUID>/` — saved webarchive per item. |
| **Tab Groups** | `Bookmarks.plist` — `WebBookmarkTypeList` folders (stored as bookmark folders). |
| **iCloud Tabs** | `CloudTabs.db` (SQLite) — open tabs from other Apple devices (only when iCloud sync is active). |

> Safari stores **passwords in the macOS Keychain** (outside the profile). No in-profile cookies or favicons store is documented.

---

### 5.3 Firefox — exclusive stores & fields

**`places.sqlite` extras**

| Table / field | What it adds |
|---|---|
| `moz_origins` | One row per scheme+host, aggregating frecency across a whole site. |
| `moz_places.frecency` / `alt_frecency` | Composite **"frequency + recency"** ranking score — Firefox's signature metric. |
| `moz_places.rev_host` | Reversed host (`moc.elpmaxe.`) for fast domain grouping. |
| `moz_places.description` / `preview_image_url` / `site_name` | Open-Graph-style page metadata stored inline. |
| `moz_annos` / `moz_items_annos` | Generic key/value annotations on pages and bookmarks. |
| `moz_inputhistory` | Address-bar learning: which typed string led to which page, weighted by `use_count`. |
| `moz_keywords` | Keyword shortcuts (e.g. `wiki` → a URL/POST body). |
| `moz_historyvisits.triggeringPlaceId` | The page that triggered a navigation (e.g. a search-results page). |
| `moz_places_metadata` | Per-page **engagement metrics**: `total_view_time`, `typing_time`, `key_presses`, `scrolling_time`, `scrolling_distance`. |
| `moz_places_metadata_search_queries` | Deduplicated search-query strings referenced by the metadata table. |
| `moz_newtab_story_click` / `moz_newtab_story_impression` | New-tab story telemetry (clicks, impressions, positions). |

**Sibling navigation databases**

| Store | Holds |
|---|---|
| `cookies.sqlite` (`moz_cookies`) | HTTP cookies — **plaintext** (Firefox does not encrypt cookie values at rest). |
| `formhistory.sqlite` (`moz_formhistory`) | Saved form-field autocomplete values. |
| `permissions.sqlite` (`moz_perms`) | Per-site permissions: popups, camera, notifications. |
| `content-prefs.sqlite` | Per-site content preferences (zoom level, etc.). |
| `protections.sqlite` | Tracking-protection event counters for the privacy dashboard. |
| `logins.db` + `key4.db` | Saved logins (encrypted; key material in `key4.db`). |

**Containers, native tab groups & synced tabs**

| Item | Where |
|---|---|
| **Containers (Contextual Identities)** | `containers.json` — `userContextId`, name, icon, color; tabs reference a container by ID. |
| **Native Tab Groups** | Inside `sessionstore` JSON (`groups[]` / `groupId` on each tab). |
| **Synced tabs from other devices** | Fetched live from the Sync server — **no on-disk database**. |
| Reading list | *(none)* — Firefox integrates Pocket instead. |

---

## 6. Cross-browser "almost-common" concepts

Concepts present in more than one browser but different enough to stay out of the unified
model:

| Concept | Chrome | Safari | Firefox |
|---|---|---|---|
| Favicons store in profile | `Favicons` DB | *(outside profile)* | `favicons.sqlite` |
| Cookies in profile | `Cookies` DB (values encrypted) | *(outside profile)* | `cookies.sqlite` (values plaintext) |
| Search-engine config | `Web Data.keywords` | `SearchDescriptions.plist` | *(stored in prefs, not a history store)* |
| Search-terms history | `keyword_search_terms` | *(none)* | `moz_inputhistory` + metadata search queries |
| Saved passwords | `Login Data` (encrypted) | macOS Keychain | `logins.db` + `key4.db` |
| Tab groups | Sync LevelDB `SavedTabGroup` | `Bookmarks.plist` folders | `sessionstore` `groups[]` |
| Reading list | Sync LevelDB `ReadingList` | `Bookmarks.plist` + `ReadingListArchives/` | *(none — uses Pocket)* |
| Synced tabs (other devices) | Sync LevelDB `Sessions` | `CloudTabs.db` | live from server (no disk) |
| Per-visit/URL ranking score | `segments` / journeys | `visit_count_score` | `frecency` |

---

*Derived from the verified per-browser store documentation. Created 2026-06-24.*
