import fs from "node:fs/promises";
import path from "node:path";

const STEAM_UA =
  "Mozilla/5.0 (compatible; SteamSuggestions/1.0; +https://localhost)";
const FRESH_MS = 24 * 60 * 60 * 1000;

export function reviewsToPopularity(reviews) {
  const n = Math.max(0, Number(reviews) || 0);
  if (n <= 0) return 25;
  const log = Math.log10(n + 1);
  const min = Math.log10(80);
  const max = Math.log10(1_500_000);
  return Math.round(Math.min(100, Math.max(0, ((log - min) / (max - min)) * 100)));
}

export function createReviewStore(filePath) {
  let cache = {};

  async function load() {
    try {
      cache = JSON.parse(await fs.readFile(filePath, "utf8"));
    } catch {
      cache = {};
    }
  }

  async function save() {
    await fs.mkdir(path.dirname(filePath), { recursive: true });
    await fs.writeFile(filePath, JSON.stringify(cache), "utf8");
  }

  async function fetchReviews(appid) {
    const url = new URL(`https://store.steampowered.com/appreviews/${appid}`);
    url.searchParams.set("json", "1");
    url.searchParams.set("language", "all");
    url.searchParams.set("purchase_type", "all");
    url.searchParams.set("num_per_page", "0");
    url.searchParams.set("filter", "all");
    const res = await fetch(url, { headers: { "User-Agent": STEAM_UA } });
    if (res.status === 429) {
      const err = new Error("rate");
      err.retry = true;
      throw err;
    }
    if (!res.ok) return null;
    const json = await res.json();
    const total = Number(json?.query_summary?.total_reviews);
    if (!Number.isFinite(total)) return null;
    return {
      appid,
      t: Date.now(),
      total,
      popularity: reviewsToPopularity(total),
    };
  }

  async function mapWithLimit(items, limit, fn) {
    const results = new Array(items.length);
    let index = 0;
    async function worker() {
      while (index < items.length) {
        const current = index++;
        results[current] = await fn(items[current]);
      }
    }
    await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
    return results;
  }

  async function getCounts(appids) {
    const unique = [...new Set(appids.map(Number).filter(Boolean))];
    const now = Date.now();
    const missing = [];
    const counts = {};

    for (const appid of unique) {
      const entry = cache[String(appid)];
      if (entry?.total >= 0 && now - entry.t < FRESH_MS) counts[appid] = entry;
      else missing.push(appid);
    }

    await mapWithLimit(missing, 4, async (appid) => {
      for (let attempt = 0; attempt < 3; attempt += 1) {
        try {
          await new Promise((r) => setTimeout(r, 80));
          const snapshot = await fetchReviews(appid);
          if (snapshot) {
            cache[String(appid)] = snapshot;
            counts[appid] = snapshot;
          }
          return;
        } catch (err) {
          if (err.retry && attempt < 2) {
            await new Promise((r) => setTimeout(r, 900 * (attempt + 1)));
            continue;
          }
        }
      }
    });

    if (missing.length) save().catch(() => {});
    return counts;
  }

  return { load, getCounts };
}
