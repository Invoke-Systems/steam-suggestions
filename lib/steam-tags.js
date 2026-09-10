const STEAM_UA = "Mozilla/5.0 (compatible; SteamSuggestions/1.0; +https://localhost)";
const TAG_LIST_URL = "https://api.steampowered.com/IStoreService/GetTagList/v1/?language=english";
const ITEMS_URL = "https://api.steampowered.com/IStoreBrowseService/GetItems/v1/";
const APPLIST_URL = "https://api.steampowered.com/IStoreService/GetAppList/v1/";
const BATCH = 40;
const TAG_TTL_MS = 14 * 24 * 60 * 60 * 1000;
const TAG_DICT_TTL_MS = 7 * 24 * 60 * 60 * 1000;
const APPLIST_TTL_MS = 7 * 24 * 60 * 60 * 1000;

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function createTagStore(db, { getApiKey } = {}) {
  let ingesting = false;

  async function refreshTagDict(force = false) {
    const at = Number(db.getMeta("tag_list_at") || 0);
    const count = db.stats().tagNames;
    if (!force && count > 0 && Date.now() - at < TAG_DICT_TTL_MS) return;
    const res = await fetch(TAG_LIST_URL, { headers: { "User-Agent": STEAM_UA } });
    if (!res.ok) throw new Error(`Steam tag list returned ${res.status}.`);
    const json = await res.json();
    const tags = json.response?.tags || [];
    if (!tags.length) return;
    db.replaceTagDict(tags, json.response?.version_hash || "");
  }

  async function fetchItems(appids) {
    const input = {
      ids: appids.map((appid) => ({ appid: Number(appid) })),
      context: { language: "english", country_code: "US" },
      data_request: { include_tag_count: 20 },
    };
    const url = `${ITEMS_URL}?input_json=${encodeURIComponent(JSON.stringify(input))}`;
    const res = await fetch(url, { headers: { "User-Agent": STEAM_UA } });
    if (!res.ok) {
      const err = new Error(`Steam GetItems returned ${res.status}.`);
      err.retry = res.status === 429 || res.status >= 500;
      throw err;
    }
    const json = await res.json();
    return json.response?.store_items || [];
  }

  async function ensureTags(appids, { names = {} } = {}) {
    const ids = [...new Set(appids.map(Number).filter(Boolean))];
    if (!ids.length) return new Map();

    await refreshTagDict();

    for (const id of ids) {
      const name = names[id] || names[String(id)] || "";
      if (name) db.upsertGame(id, name);
    }

    const missing = db.staleAppids(ids, TAG_TTL_MS);
    for (let i = 0; i < missing.length; i += BATCH) {
      const chunk = missing.slice(i, i + BATCH);
      try {
        const items = await fetchItems(chunk);
        const seen = new Set();
        for (const item of items) {
          const appid = Number(item.appid);
          seen.add(appid);
          const tags = (item.tags || []).map((tag) => ({
            tagid: Number(tag.tagid),
            weight: Number(tag.weight) || 0,
          }));
          db.setGameTags(appid, item.name || names[appid] || names[String(appid)] || "", tags, true);
        }
        for (const appid of chunk) {
          if (!seen.has(Number(appid))) {
            db.setGameTags(appid, names[appid] || names[String(appid)] || "", [], false);
          }
        }
      } catch (err) {
        if (err.retry) await sleep(1200);
      }
      if (i + BATCH < missing.length) await sleep(120);
    }

    return db.getTags(ids);
  }

  async function ingestAppList() {
    if (ingesting) return db.stats();
    const key = getApiKey?.() || process.env.STEAM_API_KEY?.trim() || "";
    if (!key) return db.stats();
    const at = Number(db.getMeta("applist_at") || 0);
    const existing = db.stats().games;
    if (existing > 1000 && Date.now() - at < APPLIST_TTL_MS) return db.stats();

    ingesting = true;
    try {
      let lastAppid = 0;
      let haveMore = true;
      let total = 0;
      while (haveMore) {
        const url = new URL(APPLIST_URL);
        url.searchParams.set("key", key);
        url.searchParams.set("include_games", "true");
        url.searchParams.set("max_results", "50000");
        if (lastAppid) url.searchParams.set("last_appid", String(lastAppid));
        const res = await fetch(url, { headers: { "User-Agent": STEAM_UA } });
        if (!res.ok) throw new Error(`Steam app list returned ${res.status}.`);
        const json = await res.json();
        const apps = (json.response?.apps || []).map((app) => ({
          appid: app.appid,
          name: app.name || "",
          price_change_number: app.price_change_number ?? null,
        }));
        if (apps.length) db.upsertGames(apps);
        total += apps.length;
        haveMore = Boolean(json.response?.have_more_results);
        lastAppid = Number(json.response?.last_appid) || (apps.at(-1)?.appid ?? lastAppid);
        await new Promise((resolve) => setImmediate(resolve));
      }
      db.setMeta("applist_at", String(Date.now()));
      db.setMeta("applist_count", String(total));
      return db.stats();
    } finally {
      ingesting = false;
    }
  }

  return { refreshTagDict, ensureTags, ingestAppList, stats: () => db.stats() };
}
