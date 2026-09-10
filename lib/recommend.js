const TASTE_HOUR_FLOOR = 1;
const BOUNCE_HOUR_CAP = 3;

const GENERIC_TAGS = new Set([
  "Singleplayer",
  "Multiplayer",
  "Co-op",
  "Online Co-Op",
  "Local Co-Op",
  "Co-op Campaign",
  "PvP",
  "Online PvP",
  "Shared/Split Screen",
  "Steam Achievements",
  "Steam Cloud",
  "Steam Trading Cards",
  "Steam Workshop",
  "Controller",
  "Full controller support",
  "Partial Controller Support",
  "Family Sharing",
  "Remote Play Together",
  "Remote Play on TV",
  "Indie",
  "Action",
  "Adventure",
  "Casual",
  "Early Access",
  "Free to Play",
  "Great Soundtrack",
  "Replay Value",
  "Atmospheric",
  "2D",
  "3D",
  "Pixel Graphics",
  "Cute",
  "Funny",
  "Comedy",
  "Colorful",
  "Sci-fi",
  "Futuristic",
  "Story Rich",
  "First-Person",
  "Third Person",
  "Third-Person Shooter",
  "Fast-Paced",
  "Physics",
  "Physics Based",
  "Sandbox",
  "Building",
  "Open World",
  "Fantasy",
  "Dark Fantasy",
  "Horror",
  "Difficult",
  "Exploration",
  "Combat",
  "Character Customization",
  "Retro",
  "Arcade",
]);

const BROAD_TAGS = new Set(["RPG", "Strategy", "Simulation", "Sports", "Racing", "Shooter", "FPS"]);

export function normalizeName(name) {
  return String(name || "")
    .toLowerCase()
    .replace(/[™®©]/g, "")
    .replace(/[_:]+/g, " ")
    .replace(/\b(demo|playtest|legacy|goty)\b/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

export function tagSpecificity(name) {
  if (GENERIC_TAGS.has(name)) return 0.1;
  if (BROAD_TAGS.has(name)) return 0.32;
  return 1;
}

export function primaryTags(tags, limit = 3) {
  return [...(tags || [])]
    .filter((tag) => tagSpecificity(tag.name) >= 0.5)
    .sort((a, b) => (b.weight || 0) - (a.weight || 0))
    .slice(0, limit)
    .map((tag) => tag.name);
}

export function asCatalogItem(appid, name, extra = {}) {
  const tags = extra.tags || [];
  return {
    appid: Number(appid),
    name,
    tags,
    clusters: extra.clusters?.length ? extra.clusters : primaryTags(tags),
    related: extra.related || [],
    popularity: extra.popularity ?? 55,
    header: extra.header || "",
    reviews: extra.reviews || 0,
  };
}

function tagVector(tags) {
  const vec = {};
  let maxWeight = 0;
  for (const tag of tags || []) maxWeight = Math.max(maxWeight, Number(tag.weight) || 0);
  if (maxWeight <= 0) return vec;
  for (const tag of tags || []) {
    const scale = tagSpecificity(tag.name);
    if (scale <= 0) continue;
    const value = ((Number(tag.weight) || 0) / maxWeight) * scale;
    if (value <= 0) continue;
    vec[tag.name] = (vec[tag.name] || 0) + value;
  }
  return vec;
}

function addVector(target, vec, scale) {
  for (const [key, value] of Object.entries(vec)) {
    target[key] = (target[key] || 0) + value * scale;
  }
}

function dot(a, b) {
  let sum = 0;
  const [left, right] = Object.keys(a).length <= Object.keys(b).length ? [a, b] : [b, a];
  for (const key of Object.keys(left)) {
    if (right[key]) sum += left[key] * right[key];
  }
  return sum;
}

function norm(vec) {
  let sum = 0;
  for (const value of Object.values(vec)) sum += value * value;
  return Math.sqrt(sum);
}

function cosine(a, b) {
  const n = norm(a) * norm(b);
  return n ? dot(a, b) / n : 0;
}

function gameHasTag(game, tagName) {
  return (game.tags || []).some((tag) => tag.name === tagName) || (game.clusters || []).includes(tagName);
}

function addWeight(map, key, amount) {
  if (!key || tagSpecificity(key) < 0.5) return;
  map[key] = (map[key] || 0) + amount;
}

export function buildTaste(games) {
  const ownedIds = new Set(games.map((game) => Number(game.appid)));
  const ownedNames = new Set(games.map((game) => normalizeName(game.name)));
  const tagHours = {};
  const vec = {};
  const bounced = [];
  const played = [];

  for (const game of games) {
    const tags = game.tags || [];
    const hours = Number(game.hours) || 0;
    const clusters = primaryTags(tags);
    if (hours > 0 && hours < BOUNCE_HOUR_CAP) {
      bounced.push({ ...game, tags, clusters });
    }
    if (hours < TASTE_HOUR_FLOOR) continue;
    const weight = hours * (game.recentlyPlayed ? 2.4 : 1);
    const gameVec = tagVector(tags);
    addVector(vec, gameVec, weight);
    let maxWeight = 0;
    for (const tag of tags) maxWeight = Math.max(maxWeight, Number(tag.weight) || 0);
    for (const tag of tags) {
      const share = maxWeight ? (Number(tag.weight) || 0) / maxWeight : 0;
      addWeight(tagHours, tag.name, weight * share);
    }
    played.push({
      appid: game.appid,
      name: game.name,
      hours,
      recentlyPlayed: Boolean(game.recentlyPlayed),
      tags,
      clusters,
      vec: gameVec,
    });
  }

  played.sort((a, b) => b.hours - a.hours);
  const rankedClusters = Object.entries(tagHours).sort((a, b) => b[1] - a[1]);

  return {
    ownedIds,
    ownedNames,
    tagHours,
    vec,
    rankedClusters: rankedClusters.map(([name, hours]) => ({ name, hours })),
    topPlayed: played.slice(0, 20),
    recent: played.filter((game) => game.recentlyPlayed),
    bounced,
  };
}

function bestComparable(item, taste) {
  const related = new Set((item.related || []).map(normalizeName));
  const itemVec = item.vec || tagVector(item.tags);
  let best = null;
  for (const game of taste.topPlayed) {
    const nameHit = related.has(normalizeName(game.name));
    const similarity = cosine(itemVec, game.vec || tagVector(game.tags));
    if (!nameHit && similarity < 0.32) continue;
    const bonus = nameHit ? 2.6 : 1 + similarity;
    const value = game.hours * bonus * similarity * (game.recentlyPlayed ? 1.4 : 1);
    if (!best || value > best.value) {
      best = { game, value, nameHit, similarity, shared: sharedTagNames(item.tags, game.tags, 3) };
    }
  }
  return best;
}

function sharedTagNames(left, right, limit = 3) {
  const rightSet = new Set((right || []).map((tag) => tag.name));
  return [...(left || [])]
    .filter((tag) => rightSet.has(tag.name) && tagSpecificity(tag.name) >= 1)
    .sort((a, b) => (b.weight || 0) - (a.weight || 0))
    .slice(0, limit)
    .map((tag) => tag.name);
}

function bouncePenalty(item, taste) {
  const tags = primaryTags(item.tags, 6);
  if (!tags.length) return 0;
  let penalty = 0;
  for (const tag of tags) {
    const loved = taste.tagHours[tag] || 0;
    const bouncedInTag = taste.bounced.filter((game) => gameHasTag(game, tag));
    if (loved < 8 && bouncedInTag.length >= 2) penalty += 40;
    if (loved < 3 && bouncedInTag.length >= 1) penalty += 15;
  }
  return penalty;
}

function formatHours(hours) {
  if (hours >= 10) return `${Math.round(hours)}h`;
  return `${hours.toFixed(1)}h`;
}

function formatTagList(names) {
  if (!names.length) return "";
  if (names.length === 1) return names[0];
  if (names.length === 2) return `${names[0]} and ${names[1]}`;
  return `${names.slice(0, -1).join(", ")}, and ${names.at(-1)}`;
}

function reasonFor(item, comparable, taste) {
  if (comparable?.shared?.length) {
    const tags = formatTagList(comparable.shared);
    if (comparable.game.recentlyPlayed) {
      return `Like ${comparable.game.name} (${tags}) — in your recent play.`;
    }
    return `Like ${comparable.game.name} · ${tags} · ${formatHours(comparable.game.hours)}`;
  }
  if (comparable?.nameHit) {
    return `Related to ${comparable.game.name} (${formatHours(comparable.game.hours)})`;
  }
  const recent = taste.recent[0];
  if (recent) {
    const shared = sharedTagNames(item.tags, recent.tags, 2);
    if (shared.length) return `Close to your recent play · ${recent.name}`;
  }
  const top = taste.rankedClusters[0];
  if (top && gameHasTag(item, top.name)) {
    return `Hits your ${top.name} streak (${formatHours(top.hours)})`;
  }
  return "Fits your library tags";
}

function scoreItem(item, taste) {
  const itemVec = tagVector(item.tags);
  item.vec = itemVec;
  const overlap = dot(taste.vec, itemVec);
  const similarity = cosine(taste.vec, itemVec);
  const comparable = bestComparable(item, taste);
  const distinctiveHits = sharedTagNames(item.tags, taste.topPlayed.flatMap((game) => game.tags), 8);
  const bounce = bouncePenalty(item, taste);
  const familiarRaw = Math.max(
    0,
    similarity * 0.72 + (comparable ? Math.min(1, comparable.value / 260) * 0.28 : 0) - bounce / 500
  );

  return {
    appid: item.appid,
    name: item.name,
    clusters: primaryTags(item.tags),
    tags: (item.tags || []).slice(0, 12).map((tag) => tag.name || tag),
    popularity: Number(item.popularity ?? 50),
    reviews: Number(item.reviews) || 0,
    steamUrl: `https://store.steampowered.com/app/${item.appid}/`,
    header: item.header || `https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/${item.appid}/header.jpg`,
    familiarRaw,
    similarity,
    overlap,
    distinctiveHits,
    fit: familiarRaw,
    reason: reasonFor(item, comparable, taste),
    because: comparable?.game?.name || null,
  };
}

function clamp01(value) {
  const n = Number(value);
  if (!Number.isFinite(n)) return 0;
  return Math.min(1, Math.max(0, n));
}

function inTasteRange(item) {
  return item.similarity >= 0.16 || (item.distinctiveHits || []).length >= 1 || item.because;
}

function selectNeighbors(items, min = 24) {
  const close = items.filter(inTasteRange);
  if (close.length >= min) return close;
  const keep = new Set(close.map((item) => item.appid));
  const byFit = [...items].sort((a, b) => b.familiarRaw - a.familiarRaw || a.name.localeCompare(b.name));
  for (const item of byFit) {
    if (keep.size >= min) break;
    keep.add(item.appid);
  }
  return items.filter((item) => keep.has(item.appid));
}

function axisScore(items, valueFn) {
  const sorted = [...items].sort((a, b) => {
    const delta = valueFn(b) - valueFn(a);
    if (delta) return delta;
    return a.name.localeCompare(b.name);
  });
  const rank = new Map(sorted.map((item, index) => [item.appid, index]));
  const last = Math.max(1, items.length - 1);
  return (item) => 1 - rank.get(item.appid) / last;
}

export function stampPrices(result, prices) {
  if (!result?.groups) return result;
  for (const group of Object.values(result.groups)) {
    attachPrices(group, prices);
  }
  return result;
}

function attachPrices(items, prices) {
  for (const item of items) {
    const price = prices?.[item.appid] || prices?.[String(item.appid)];
    if (!price) continue;
    item.price = price.formatted || "";
    item.discount = price.discount || 0;
    item.onSale = Boolean(price.onSale);
    item.lowPrice = price.low || null;
    item.atLow = Boolean(price.atLow);
  }
  return items;
}

function publicCard(item) {
  const card = { ...item };
  delete card.familiarRaw;
  delete card.similarity;
  delete card.overlap;
  delete card.distinctiveHits;
  return card;
}

export function recommend(games, catalog, options = {}) {
  const taste = buildTaste(games);
  const popularityDial = Number.isFinite(Number(options.popularity)) ? clamp01(options.popularity) : 0.5;
  const weirdness = Number.isFinite(Number(options.weirdness)) ? clamp01(options.weirdness) : 0.25;
  const wMainstream = popularityDial;
  const wFamiliar = 1 - weirdness;

  const seen = new Set();
  const cards = [];
  for (const item of [...catalog, ...(options.extraCatalog || [])]) {
    const appid = Number(item.appid);
    if (!appid || seen.has(appid)) continue;
    seen.add(appid);
    if (taste.ownedIds.has(appid)) continue;
    if (taste.ownedNames.has(normalizeName(item.name))) continue;
    cards.push(scoreItem(item, taste));
  }

  attachPrices(cards, options.prices);
  let neighbors = options.onSale ? cards.filter((item) => item.onSale) : cards;
  neighbors = selectNeighbors(neighbors);

  const mainstreamOf = axisScore(neighbors, (item) => item.popularity);
  const familiarOf = axisScore(neighbors, (item) => item.familiarRaw);
  for (const item of neighbors) {
    const mainstream = mainstreamOf(item);
    const familiar = familiarOf(item);
    item.score = Math.round(
      500 * (wMainstream * mainstream + (1 - wMainstream) * (1 - mainstream)) +
        500 * (wFamiliar * familiar + (1 - wFamiliar) * (1 - familiar))
    );
  }
  neighbors.sort((a, b) => b.score - a.score || a.name.localeCompare(b.name));

  const more = neighbors.slice(0, 7).map(publicCard);
  const adjacent = neighbors.slice(7, 11).map(publicCard);
  const used = new Set([...more, ...adjacent].map((item) => item.appid));
  const leftover = neighbors.filter((item) => !used.has(item.appid));
  leftover.sort((a, b) =>
    weirdness >= 0.5 ? b.familiarRaw - a.familiarRaw : a.familiarRaw - b.familiarRaw
  );
  const wildcard = leftover[0] ? [publicCard(leftover[0])] : [];

  return {
    taste: {
      heaviest: taste.rankedClusters[0]?.name || null,
      clusters: taste.rankedClusters.slice(0, 6),
      recent: taste.recent.map((game) => game.name),
      topPlayed: taste.topPlayed.slice(0, 8).map((game) => ({ name: game.name, hours: game.hours })),
    },
    dials: {
      popularity: popularityDial,
      weirdness,
      onSale: Boolean(options.onSale),
    },
    groups: {
      more_like: more,
      adjacent,
      wildcard,
    },
  };
}

