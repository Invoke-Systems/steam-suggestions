import fs from "node:fs";
import path from "node:path";
import { DatabaseSync } from "node:sqlite";

export const STEAM_SCHEMA = `
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT
);
CREATE TABLE IF NOT EXISTS tag_dict (
  tagid INTEGER PRIMARY KEY,
  name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS games (
  appid INTEGER PRIMARY KEY,
  name TEXT NOT NULL DEFAULT '',
  tags_fetched_at INTEGER,
  tags_ok INTEGER NOT NULL DEFAULT 0,
  price_fetched_at INTEGER,
  price_change_number INTEGER
);
CREATE TABLE IF NOT EXISTS game_tags (
  appid INTEGER NOT NULL,
  tagid INTEGER NOT NULL,
  weight INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (appid, tagid)
);
CREATE INDEX IF NOT EXISTS idx_game_tags_tagid ON game_tags(tagid);
CREATE TABLE IF NOT EXISTS price_latest (
  appid INTEGER PRIMARY KEY,
  t INTEGER NOT NULL,
  currency TEXT NOT NULL DEFAULT '',
  initial INTEGER NOT NULL DEFAULT 0,
  final INTEGER NOT NULL DEFAULT 0,
  discount INTEGER NOT NULL DEFAULT 0,
  formatted TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS price_history (
  appid INTEGER NOT NULL,
  day INTEGER NOT NULL,
  t INTEGER NOT NULL,
  currency TEXT NOT NULL DEFAULT '',
  initial INTEGER NOT NULL DEFAULT 0,
  final INTEGER NOT NULL DEFAULT 0,
  discount INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (appid, day)
);
`;

function tryExec(db, sql) {
  try {
    db.exec(sql);
  } catch {
    // column or table already exists
  }
}

export function openSteamDb(dbPath) {
  fs.mkdirSync(path.dirname(dbPath), { recursive: true });
  const db = new DatabaseSync(dbPath);
  db.exec("PRAGMA journal_mode = WAL");
  db.exec("PRAGMA synchronous = NORMAL");
  db.exec("PRAGMA busy_timeout = 5000");
  db.exec(STEAM_SCHEMA);
  tryExec(db, "ALTER TABLE games ADD COLUMN price_fetched_at INTEGER");
  tryExec(db, "ALTER TABLE games ADD COLUMN price_change_number INTEGER");

  const getMetaStmt = db.prepare("SELECT value FROM meta WHERE key = ?");
  const setMetaStmt = db.prepare(
    "INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value"
  );
  const upsertGameStmt = db.prepare(`
    INSERT INTO games (appid, name) VALUES (?, ?)
    ON CONFLICT(appid) DO UPDATE SET name = CASE
      WHEN excluded.name != '' THEN excluded.name
      ELSE games.name
    END
  `);
  const upsertGameWithChangeStmt = db.prepare(`
    INSERT INTO games (appid, name, price_change_number) VALUES (?, ?, ?)
    ON CONFLICT(appid) DO UPDATE SET
      name = CASE WHEN excluded.name != '' THEN excluded.name ELSE games.name END,
      price_fetched_at = CASE
        WHEN games.price_change_number IS NOT NULL
          AND excluded.price_change_number IS NOT NULL
          AND games.price_change_number != excluded.price_change_number
        THEN NULL
        ELSE games.price_fetched_at
      END,
      price_change_number = COALESCE(excluded.price_change_number, games.price_change_number)
  `);
  const replaceDict = db.prepare("INSERT INTO tag_dict (tagid, name) VALUES (?, ?)");
  const deleteTags = db.prepare("DELETE FROM game_tags WHERE appid = ?");
  const insertTag = db.prepare("INSERT INTO game_tags (appid, tagid, weight) VALUES (?, ?, ?)");
  const markFetched = db.prepare(
    "UPDATE games SET tags_fetched_at = ?, tags_ok = ? WHERE appid = ?"
  );
  const markPriced = db.prepare("UPDATE games SET price_fetched_at = ? WHERE appid = ?");
  const selectStale = db.prepare(`
    SELECT appid FROM games
    WHERE appid = ? AND tags_fetched_at IS NOT NULL AND tags_fetched_at > ?
  `);
  const selectTags = db.prepare(`
    SELECT gt.tagid, gt.weight, COALESCE(td.name, 'Tag ' || gt.tagid) AS name
    FROM game_tags gt
    LEFT JOIN tag_dict td ON td.tagid = gt.tagid
    WHERE gt.appid = ?
    ORDER BY gt.weight DESC
  `);
  const upsertLatest = db.prepare(`
    INSERT INTO price_latest (appid, t, currency, initial, final, discount, formatted)
    VALUES (?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(appid) DO UPDATE SET
      t = excluded.t,
      currency = excluded.currency,
      initial = excluded.initial,
      final = excluded.final,
      discount = excluded.discount,
      formatted = excluded.formatted
  `);
  const upsertHistory = db.prepare(`
    INSERT INTO price_history (appid, day, t, currency, initial, final, discount)
    VALUES (?, ?, ?, ?, ?, ?, ?)
    ON CONFLICT(appid, day) DO UPDATE SET
      t = excluded.t,
      currency = excluded.currency,
      initial = excluded.initial,
      final = excluded.final,
      discount = excluded.discount
  `);
  const selectPrice = db.prepare(`
    SELECT
      pl.appid, pl.t, pl.currency, pl.initial, pl.final, pl.discount, pl.formatted,
      (SELECT MIN(ph.final) FROM price_history ph WHERE ph.appid = pl.appid AND ph.final > 0) AS low_final,
      (SELECT MAX(ph.final) FROM price_history ph WHERE ph.appid = pl.appid AND ph.final > 0) AS high_final
    FROM price_latest pl
    WHERE pl.appid = ?
  `);
  const gameCountStmt = db.prepare("SELECT COUNT(*) AS n FROM games");
  const taggedCountStmt = db.prepare("SELECT COUNT(*) AS n FROM games WHERE tags_ok = 1");
  const pricedCountStmt = db.prepare("SELECT COUNT(*) AS n FROM games WHERE price_fetched_at IS NOT NULL");
  const tagDictCountStmt = db.prepare("SELECT COUNT(*) AS n FROM tag_dict");

  function getMeta(key) {
    const row = getMetaStmt.get(key);
    return row?.value ?? null;
  }

  function setMeta(key, value) {
    setMetaStmt.run(key, String(value));
  }

  function upsertGame(appid, name = "") {
    upsertGameStmt.run(Number(appid), String(name || ""));
  }

  function withTransaction(fn) {
    db.exec("BEGIN");
    try {
      const result = fn();
      db.exec("COMMIT");
      return result;
    } catch (err) {
      try {
        db.exec("ROLLBACK");
      } catch {
        // ignore rollback failures
      }
      throw err;
    }
  }

  function upsertGames(apps) {
    withTransaction(() => {
      for (const app of apps) {
        const change = app.price_change_number;
        if (change == null) {
          upsertGameStmt.run(Number(app.appid), String(app.name || ""));
        } else {
          upsertGameWithChangeStmt.run(Number(app.appid), String(app.name || ""), Number(change));
        }
      }
    });
  }

  function replaceTagDict(tags, versionHash) {
    withTransaction(() => {
      db.exec("DELETE FROM tag_dict");
      for (const tag of tags) {
        replaceDict.run(Number(tag.tagid), String(tag.name || ""));
      }
    });
    if (versionHash) setMeta("tag_list_hash", versionHash);
    setMeta("tag_list_at", String(Date.now()));
  }

  function setGameTags(appid, name, tags, ok) {
    const id = Number(appid);
    withTransaction(() => {
      upsertGameStmt.run(id, String(name || ""));
      deleteTags.run(id);
      for (const tag of tags) {
        insertTag.run(id, Number(tag.tagid), Number(tag.weight) || 0);
      }
      markFetched.run(Date.now(), ok ? 1 : 0, id);
    });
  }

  function setPrice(appid, snapshot) {
    const id = Number(appid);
    const t = Number(snapshot.t) || Date.now();
    const day = Math.floor(t / 86400000);
    withTransaction(() => {
      upsertGameStmt.run(id, String(snapshot.name || ""));
      upsertLatest.run(
        id,
        t,
        String(snapshot.currency || ""),
        Number(snapshot.initial) || 0,
        Number(snapshot.final) || 0,
        Number(snapshot.discount) || 0,
        String(snapshot.formatted || "")
      );
      upsertHistory.run(
        id,
        day,
        t,
        String(snapshot.currency || ""),
        Number(snapshot.initial) || 0,
        Number(snapshot.final) || 0,
        Number(snapshot.discount) || 0
      );
      markPriced.run(t, id);
    });
  }

  function staleAppids(appids, maxAgeMs) {
    const cutoff = Date.now() - maxAgeMs;
    const missing = [];
    for (const appid of appids) {
      const id = Number(appid);
      if (!id) continue;
      upsertGameStmt.run(id, "");
      const fresh = selectStale.get(id, cutoff);
      if (!fresh) missing.push(id);
    }
    return [...new Set(missing)];
  }

  function getTags(appids) {
    const map = new Map();
    for (const appid of appids) {
      const id = Number(appid);
      const rows = selectTags.all(id);
      map.set(id, rows.map((row) => ({ tagid: row.tagid, name: row.name, weight: row.weight })));
    }
    return map;
  }

  function decoratePrice(row) {
    if (!row) return null;
    const lowFinal = Number(row.low_final);
    const highFinal = Number(row.high_final);
    const hasLow = Number.isFinite(lowFinal) && lowFinal > 0;
    const seenHigher = Number.isFinite(highFinal) && highFinal > (Number(row.final) || 0);
    return {
      appid: Number(row.appid),
      t: Number(row.t),
      currency: row.currency || "",
      initial: Number(row.initial) || 0,
      final: Number(row.final) || 0,
      discount: Number(row.discount) || 0,
      formatted: row.formatted || "",
      onSale: (Number(row.discount) || 0) > 0,
      low: hasLow && seenHigher ? formatCents(lowFinal, row.currency) : null,
      atLow: hasLow && seenHigher && (Number(row.discount) || 0) > 0 && (Number(row.final) || 0) <= lowFinal,
    };
  }

  function getPrices(appids) {
    const prices = {};
    for (const appid of appids) {
      const id = Number(appid);
      const row = selectPrice.get(id);
      if (row) prices[id] = decoratePrice(row);
    }
    return prices;
  }

  function stats() {
    return {
      games: gameCountStmt.get()?.n || 0,
      tagged: taggedCountStmt.get()?.n || 0,
      priced: pricedCountStmt.get()?.n || 0,
      tagNames: tagDictCountStmt.get()?.n || 0,
    };
  }

  return {
    getMeta,
    setMeta,
    upsertGame,
    upsertGames,
    replaceTagDict,
    setGameTags,
    setPrice,
    staleAppids,
    getTags,
    getPrices,
    stats,
    close: () => db.close(),
  };
}

export function formatCents(cents, currency) {
  const n = Number(cents);
  if (!Number.isFinite(n) || n <= 0) return null;
  const amount = (n / 100).toFixed(2);
  if (!currency || currency === "USD") return `$${amount}`;
  return `${amount} ${currency}`;
}
