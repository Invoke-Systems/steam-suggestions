import express from "express";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { buildTaste, primaryTags, recommend, stampPrices } from "./lib/recommend.js";
import { createPriceStore } from "./lib/prices.js";
import { createReviewStore } from "./lib/reviews.js";
import { openSteamDb } from "./lib/steam-db.js";
import { createTagStore } from "./lib/steam-tags.js";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const CACHE_PATH = path.join(__dirname, "data", "genre-cache.json");
const REVIEW_PATH = path.join(__dirname, "data", "review-cache.json");
const STEAM_UA =
  "Mozilla/5.0 (compatible; SteamSuggestions/1.0; +https://localhost)";

async function loadEnv() {
  try {
    const raw = await fs.readFile(path.join(__dirname, ".env"), "utf8");
    for (const line of raw.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith("#")) continue;
      const eq = trimmed.indexOf("=");
      if (eq === -1) continue;
      const key = trimmed.slice(0, eq).trim();
      const value = trimmed.slice(eq + 1).trim().replace(/^['"]|['"]$/g, "");
      if (!(key in process.env)) process.env[key] = value;
    }
  } catch {
    // No .env file is fine; the UI can still collect a key.
  }
}

const app = express();
app.use(express.json({ limit: "1mb" }));
app.use(express.static(path.join(__dirname, "public")));

const STEAM_DB_PATH = path.join(__dirname, "data", "steam.sqlite");
let genreCache = {};
let catalog = [];
const steamDb = openSteamDb(STEAM_DB_PATH);
const priceStore = createPriceStore(steamDb);
const reviewStore = createReviewStore(REVIEW_PATH);
const tagStore = createTagStore(steamDb, { getApiKey: envKey });
let fallbackPopularity = {};
const libraryCache = new Map();
const rateBuckets = new Map();
const LIBRARY_CACHE_MS = 10 * 60 * 1000;

async function loadCache() {
  try {
    const raw = await fs.readFile(CACHE_PATH, "utf8");
    genreCache = JSON.parse(raw);
  } catch {
    genreCache = {};
  }
}

async function saveCache() {
  await fs.mkdir(path.dirname(CACHE_PATH), { recursive: true });
  await fs.writeFile(CACHE_PATH, JSON.stringify(genreCache), "utf8");
}

function envKey() {
  return process.env.STEAM_API_KEY?.trim() || "";
}

function acceptClientKey() {
  if (envKey()) return false;
  const flag = process.env.ALLOW_CLIENT_API_KEY?.trim().toLowerCase();
  return flag !== "0" && flag !== "false";
}

function resolveKey(bodyKey) {
  const server = envKey();
  if (server) return server;
  const client = String(bodyKey || "").trim();
  if (acceptClientKey() && client) return client;
  const err = new Error(
    acceptClientKey()
      ? "Add STEAM_API_KEY to .env, or paste a Steam Web API key for local use."
      : "This instance is not configured with a Steam API key."
  );
  err.status = acceptClientKey() ? 400 : 503;
  throw err;
}

function clientIp(req) {
  return req.socket?.remoteAddress || "unknown";
}

function rateLimit(req, max, windowMs) {
  const key = `${req.path}:${clientIp(req)}`;
  const now = Date.now();
  const recent = (rateBuckets.get(key) || []).filter((time) => now - time < windowMs);
  if (recent.length >= max) {
    const err = new Error("Too many requests. Try again in a few minutes.");
    err.status = 429;
    throw err;
  }
  recent.push(now);
  rateBuckets.set(key, recent);
}

function parseSteamInput(raw) {
  const input = String(raw || "").trim();
  if (!input) {
    const err = new Error("Enter a Steam profile URL, vanity name, or SteamID64.");
    err.status = 400;
    throw err;
  }

  const profile = input.match(/steamcommunity\.com\/profiles\/(\d{17})/i);
  if (profile) return { steamid: profile[1] };

  const vanityMatch = input.match(/steamcommunity\.com\/id\/([^/?#]+)/i);
  if (vanityMatch) return { vanity: decodeURIComponent(vanityMatch[1]) };

  if (/^\d{17}$/.test(input)) return { steamid: input };

  return { vanity: input.replace(/^@/, "") };
}

async function steamGet(url) {
  const res = await fetch(url, { headers: { "User-Agent": STEAM_UA } });
  if (!res.ok) {
    const err = new Error(`Steam API returned ${res.status}.`);
    err.status = res.status === 403 ? 403 : 502;
    throw err;
  }
  return res.json();
}

async function resolveSteamId(key, parsed) {
  if (parsed.steamid) return parsed.steamid;
  const url = new URL("https://api.steampowered.com/ISteamUser/ResolveVanityURL/v1/");
  url.searchParams.set("key", key);
  url.searchParams.set("vanityurl", parsed.vanity);
  const data = await steamGet(url);
  const steamid = data?.response?.steamid;
  if (data?.response?.success !== 1 || !steamid) {
    const err = new Error(`Could not resolve Steam vanity URL "${parsed.vanity}".`);
    err.status = 404;
    throw err;
  }
  return steamid;
}

function minutesToHours(minutes) {
  return Math.round(((Number(minutes) || 0) / 60) * 10) / 10;
}

function mapGame(game, recentIds) {
  const minutes = Number(game.playtime_forever) || 0;
  const recentMinutes = Number(game.playtime_2weeks) || 0;
  const lastPlayed = Number(game.rtime_last_played) || 0;
  return {
    appid: game.appid,
    name: game.name || `App ${game.appid}`,
    icon: game.img_icon_url
      ? `https://media.steampowered.com/steamcommunity/public/images/apps/${game.appid}/${game.img_icon_url}.jpg`
      : "",
    header: `https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/${game.appid}/header.jpg`,
    minutes,
    hours: minutesToHours(minutes),
    minutes2Weeks: recentMinutes,
    hours2Weeks: minutesToHours(recentMinutes),
    lastPlayed,
    recentlyPlayed: recentIds.has(game.appid) || recentMinutes > 0,
    steamUrl: `https://store.steampowered.com/app/${game.appid}/`,
  };
}

app.get("/api/health", (_req, res) => {
  res.json({ ok: true, steam: tagStore.stats() });
});

app.get("/api/config", (_req, res) => {
  res.json({
    hasServerKey: Boolean(envKey()),
    acceptClientKey: acceptClientKey(),
    recommend: true,
  });
});

app.post("/api/library", async (req, res, next) => {
  try {
    rateLimit(req, 20, 60 * 60 * 1000);
    const key = resolveKey(req.body?.apiKey);
    const steamid = await resolveSteamId(key, parseSteamInput(req.body?.identifier));
    const cached = libraryCache.get(steamid);
    if (cached && Date.now() - cached.at < LIBRARY_CACHE_MS) {
      res.json(cached.payload);
      return;
    }

    const ownedUrl = new URL("https://api.steampowered.com/IPlayerService/GetOwnedGames/v1/");
    ownedUrl.searchParams.set("key", key);
    ownedUrl.searchParams.set("steamid", steamid);
    ownedUrl.searchParams.set("include_appinfo", "true");
    ownedUrl.searchParams.set("include_played_free_games", "true");
    ownedUrl.searchParams.set("format", "json");

    const recentUrl = new URL("https://api.steampowered.com/IPlayerService/GetRecentlyPlayedGames/v1/");
    recentUrl.searchParams.set("key", key);
    recentUrl.searchParams.set("steamid", steamid);
    recentUrl.searchParams.set("count", "0");

    const playerUrl = new URL("https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/");
    playerUrl.searchParams.set("key", key);
    playerUrl.searchParams.set("steamids", steamid);

    const [owned, recent, summaries] = await Promise.all([
      steamGet(ownedUrl),
      steamGet(recentUrl),
      steamGet(playerUrl),
    ]);

    const ownedGames = owned?.response?.games || [];
    if (!ownedGames.length && !owned?.response?.game_count) {
      const err = new Error(
        "No games returned. The profile must have Game details set to Public."
      );
      err.status = 404;
      throw err;
    }

    const recentGames = recent?.response?.games || [];
    const recentIds = new Set(recentGames.map((game) => game.appid));
    const player = summaries?.response?.players?.[0] || null;
    const games = ownedGames
      .map((game) => mapGame(game, recentIds))
      .sort((a, b) => b.minutes - a.minutes || a.name.localeCompare(b.name));

    const payload = {
      player: player
        ? {
            steamid: player.steamid,
            name: player.personaname,
            avatar: player.avatarfull || player.avatarmedium,
            profileUrl: player.profileurl,
          }
        : { steamid, name: "Steam player", avatar: "", profileUrl: "" },
      games,
      recentlyPlayed: recentGames.map((game) => mapGame(game, recentIds)),
    };
    const played = games.filter((game) => game.hours >= 1);
    const tagMap = steamDb.getTags(played.map((game) => game.appid));
    const taste = buildTaste(
      played.map((game) => ({
        ...game,
        tags: tagMap.get(Number(game.appid)) || [],
      }))
    );
    payload.taste = { clusters: taste.rankedClusters.slice(0, 6) };
    for (const game of games) {
      const tags = tagMap.get(Number(game.appid));
      if (tags?.length) game.tags = primaryTags(tags, 3);
    }
    libraryCache.set(steamid, { at: Date.now(), payload });
    if (played.length) {
      const names = Object.fromEntries(played.slice(0, 60).map((game) => [game.appid, game.name]));
      tagStore.ensureTags(played.slice(0, 60).map((game) => game.appid), { names }).catch(() => {});
    }
    res.json(payload);
  } catch (err) {
    next(err);
  }
});

async function fetchStoreDetails(appid) {
  const url = new URL("https://store.steampowered.com/api/appdetails");
  url.searchParams.set("appids", String(appid));
  url.searchParams.set("filters", "basic,genres,developers,publishers,release_date,metacritic,categories");
  url.searchParams.set("cc", "us");
  url.searchParams.set("l", "english");

  const res = await fetch(url, {
    headers: {
      "User-Agent": STEAM_UA,
      "Accept-Language": "en-US,en;q=0.9",
    },
  });

  if (res.status === 429) {
    const err = new Error("rate");
    err.retry = true;
    throw err;
  }
  if (!res.ok) {
    return { appid, missing: true, genres: [] };
  }

  const json = await res.json();
  const entry = json?.[appid];
  if (!entry?.success || !entry.data) {
    return { appid, missing: true, genres: [] };
  }

  const data = entry.data;
  return {
    appid,
    type: data.type || "",
    genres: (data.genres || []).map((g) => g.description).filter(Boolean),
    categories: (data.categories || []).map((c) => c.description).filter(Boolean),
    developers: data.developers || [],
    publishers: data.publishers || [],
    releaseDate: data.release_date?.date || "",
    metacritic: data.metacritic?.score ?? null,
    shortDescription: data.short_description || "",
  };
}

async function mapWithLimit(items, limit, fn) {
  const results = new Array(items.length);
  let index = 0;

  async function worker() {
    while (index < items.length) {
      const current = index++;
      results[current] = await fn(items[current], current);
    }
  }

  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
  return results;
}

app.post("/api/recommend", async (req, res, next) => {
  try {
    rateLimit(req, 40, 60 * 60 * 1000);
    const games = req.body?.games;
    if (!Array.isArray(games) || games.length === 0) {
      const err = new Error("Load a library before requesting recommendations.");
      err.status = 400;
      throw err;
    }
    if (games.length > 8000) {
      const err = new Error("Library is too large to score.");
      err.status = 400;
      throw err;
    }
    const slim = games.map((game) => ({
      appid: Number(game.appid),
      name: String(game.name || ""),
      hours: Number(game.hours) || 0,
      recentlyPlayed: Boolean(game.recentlyPlayed),
    }));
    const onSale = Boolean(req.body?.onSale);
    const specials = onSale ? await priceStore.featuredSales() : [];
    const pool = [...catalog];
    const names = {};
    for (const game of slim) names[game.appid] = game.name;
    for (const item of pool) names[item.appid] = item.name;
    const tagIds = [
      ...slim.filter((game) => game.hours >= 1).map((game) => game.appid),
      ...pool.map((item) => item.appid),
    ];
    const tagMap = await tagStore.ensureTags(tagIds, { names });
    const taggedGames = slim.map((game) => ({
      ...game,
      tags: fallbackTags(tagMap.get(game.appid), []),
    }));
    const reviewCounts = await reviewStore.getCounts(pool.map((item) => item.appid));
    const scoredCatalog = catalog.map((item) =>
      withReviewPopularity(withSteamTags(item, tagMap), reviewCounts)
    );
    let prices = {};
    if (onSale) {
      prices = await priceStore.getPrices(catalog.map((item) => item.appid));
      const catalogIds = new Set(catalog.map((item) => Number(item.appid)));
      for (const sale of specials) {
        if (!catalogIds.has(Number(sale.appid))) continue;
        if (sale.discount > 0 && !prices[sale.appid]?.onSale) {
          prices[sale.appid] = {
            appid: sale.appid,
            t: Date.now(),
            discount: sale.discount,
            formatted: sale.formatted,
            final: sale.final,
            onSale: true,
            low: prices[sale.appid]?.low || null,
            atLow: false,
          };
        }
      }
    }
    const result = recommend(taggedGames, scoredCatalog, {
      popularity: Number(req.body?.popularity),
      weirdness: Number(req.body?.weirdness),
      onSale,
      prices,
    });
    const shown = Object.values(result.groups).flat().map((item) => item.appid);
    const missing = shown.filter((appid) => !prices[appid]);
    if (missing.length) {
      const extraPrices = await priceStore.getPrices(missing);
      stampPrices(result, extraPrices);
    }
    res.json(result);
  } catch (err) {
    next(err);
  }
});

app.post("/api/genres", async (req, res, next) => {
  try {
    const appids = [...new Set((req.body?.appids || []).map(Number).filter(Boolean))];
    const details = {};
    const missing = [];

    for (const appid of appids) {
      if (genreCache[appid]) details[appid] = genreCache[appid];
      else missing.push(appid);
    }

    await mapWithLimit(missing, 3, async (appid) => {
      await new Promise((r) => setTimeout(r, 120));
      for (let attempt = 0; attempt < 4; attempt += 1) {
        try {
          const info = await fetchStoreDetails(appid);
          genreCache[appid] = info;
          details[appid] = info;
          return;
        } catch (err) {
          if (err.retry && attempt < 3) {
            await new Promise((r) => setTimeout(r, 1200 * (attempt + 1)));
            continue;
          }
          details[appid] = { appid, missing: true, genres: [] };
          return;
        }
      }
    });

    if (missing.length) {
      saveCache().catch(() => {});
    }

    res.json({ details });
  } catch (err) {
    next(err);
  }
});

app.use((err, _req, res, _next) => {
  const status = err.status || 500;
  res.status(status).json({ error: err.message || "Unexpected server error." });
});

function fallbackTags(steamTags, catalogTags) {
  if (steamTags?.length) return steamTags;
  if (Array.isArray(catalogTags) && catalogTags[0]?.name) return catalogTags;
  return (catalogTags || []).map((name, index) => ({
    name,
    weight: Math.max(1, 20 - index),
    tagid: 0,
  }));
}

function withSteamTags(item, tagMap) {
  const tags = fallbackTags(tagMap.get(Number(item.appid)), item.tags);
  return { ...item, tags, clusters: primaryTags(tags) };
}

function withReviewPopularity(item, reviewCounts) {
  const stats = reviewCounts[item.appid] || reviewCounts[String(item.appid)];
  if (stats?.total >= 0) {
    return { ...item, reviews: stats.total, popularity: stats.popularity };
  }
  return {
    ...item,
    reviews: 0,
    popularity: fallbackPopularity[String(item.appid)] ?? fallbackPopularity[item.appid] ?? 50,
  };
}

await loadEnv();
await loadCache();
await priceStore.load();
await reviewStore.load();
catalog = JSON.parse(await fs.readFile(path.join(__dirname, "data", "catalog.json"), "utf8"));
try {
  fallbackPopularity = JSON.parse(await fs.readFile(path.join(__dirname, "data", "popularity.json"), "utf8"));
} catch {
  fallbackPopularity = {};
}
const PORT = Number(process.env.PORT) || 3847;
app.listen(PORT, () => {
  console.log(`Steam Suggestions running at http://localhost:${PORT}`);
  reviewStore.getCounts(catalog.map((item) => item.appid)).catch(() => {});
  const catalogNames = Object.fromEntries(catalog.map((item) => [item.appid, item.name]));
  tagStore
    .refreshTagDict()
    .then(() => tagStore.ensureTags(catalog.map((item) => item.appid), { names: catalogNames }))
    .then(() => tagStore.ingestAppList())
    .then((stats) => {
      if (stats) {
        console.log(
          `Steam SQLite: ${stats.games} apps, ${stats.tagged} tagged, ${stats.priced} priced`
        );
      }
    })
    .catch((err) => console.warn("Steam catalog ingest skipped:", err.message));
});
