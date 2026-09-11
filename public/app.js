let config = { hasServerKey: true, user: null, acceptClientKey: false };
let pendingGuestIdentifier = "";

const form = document.querySelector("#connect-form");
const identifierInput = document.querySelector("#identifier");
const apiKeyInput = document.querySelector("#api-key");
const apiKeyField = document.querySelector("#api-key-field");

(function stripSecretsFromURL() {
  const params = new URLSearchParams(location.search);
  const queryIdentifier = params.get("identifier");
  params.delete("apiKey");
  params.delete("key");
  params.delete("identifier");
  const cleaned = params.toString();
  const next = location.pathname + (cleaned ? `?${cleaned}` : "") + location.hash;
  if (`${location.pathname}${location.search}${location.hash}` !== next) {
    history.replaceState({}, "", next);
  }
  if (queryIdentifier && identifierInput) {
    identifierInput.value = queryIdentifier;
    pendingGuestIdentifier = queryIdentifier;
  }
})();

const loadBtn = document.querySelector("#load-btn");
const demoBtn = document.querySelector("#demo-btn");
const statusEl = document.querySelector("#status");
const workspace = document.querySelector("#workspace");
const playerCard = document.querySelector("#player-card");
const gamesEl = document.querySelector("#games");
const searchInput = document.querySelector("#search");
const sortSelect = document.querySelector("#sort");
const enrichStatus = document.querySelector("#enrich-status");

const recsEl = document.querySelector("#recs");
const recGroupsEl = document.querySelector("#rec-groups");
const tasteLine = document.querySelector("#taste-line");
const tasteProfileEl = document.querySelector("#taste-profile");
const tasteBarsEl = document.querySelector("#taste-bars");
const searchCriteriaEl = document.querySelector("#search-criteria");
const recsSearchSlot = document.querySelector("#recs-search-slot");
const accountSearchSlot = document.querySelector("#account-search-slot");
const popularityDial = document.querySelector("#dial-popularity");
const weirdnessDial = document.querySelector("#dial-weirdness");
const saleDial = document.querySelector("#dial-sale");
const adultDial = document.querySelector("#dial-adult");
const goreDial = document.querySelector("#dial-gore");
const shovelDial = document.querySelector("#dial-shovel");
const minReviewsInput = document.querySelector("#min-reviews");
const maxReviewsInput = document.querySelector("#max-reviews");
const minPositiveInput = document.querySelector("#min-positive");
const includeTagInput = document.querySelector("#include-tag");
const excludeTagInput = document.querySelector("#exclude-tag");
const includeChipsEl = document.querySelector("#include-chips");
const excludeChipsEl = document.querySelector("#exclude-chips");
const includeSuggestEl = document.querySelector("#include-suggest");
const excludeSuggestEl = document.querySelector("#exclude-suggest");
const preferTagInput = document.querySelector("#prefer-tag");
const preferSuggestEl = document.querySelector("#prefer-suggest");
const preferChipsEl = document.querySelector("#prefer-chips");
const preferAddBtn = document.querySelector("#prefer-add");
const avoidTagInput = document.querySelector("#avoid-tag");
const avoidSuggestEl = document.querySelector("#avoid-suggest");
const avoidChipsEl = document.querySelector("#avoid-chips");
const avoidAddBtn = document.querySelector("#avoid-add");
const distanceField = document.querySelector("#distance-field");
const changeProfileBtn = document.querySelector("#change-profile");
const profileBtn = document.querySelector("#profile-btn");
const topSignin = document.querySelector("#top-signin");
const brandHome = document.querySelector("#brand-home");
const accountEl = document.querySelector("#account");
const accountAvatar = document.querySelector("#account-avatar");
const accountInitials = document.querySelector("#account-initials");
const accountName = document.querySelector("#account-name");
const accountNote = document.querySelector("#account-note");
const accountOverview = document.querySelector("#account-overview");
const accountWatchlists = document.querySelector("#account-watchlists");
const accountSteamName = document.querySelector("#account-steam-name");
const accountSteamID = document.querySelector("#account-steamid");
const accountProfileLink = document.querySelector("#account-profile-link");
const accountLastLogin = document.querySelector("#account-last-login");
const accountJoined = document.querySelector("#account-joined");
const accountRecsBtn = document.querySelector("#account-recs");
const accountSales = document.querySelector("#account-sales");
const accountUpcoming = document.querySelector("#account-upcoming");
const watchlistForm = document.querySelector("#watchlist-form");
const searchNameInput = document.querySelector("#search-name");
const savedSearchesEl = document.querySelector("#saved-searches");
const advancedSearchEl = document.querySelector("#advanced-search");
const libraryToggle = document.querySelector("#library-toggle");
const libraryBody = document.querySelector("#library-body");

const HIDDEN_KEY = "steam-hidden-appids";
const FILTERS_KEY = "steam-rec-filters";

function loadHidden() {
  try {
    return JSON.parse(localStorage.getItem(HIDDEN_KEY) || "[]").filter(Number);
  } catch {
    return [];
  }
}

function saveHidden(ids) {
  localStorage.setItem(HIDDEN_KEY, JSON.stringify(ids));
}

function loadFilters() {
  try {
    return JSON.parse(localStorage.getItem(FILTERS_KEY) || "{}") || {};
  } catch {
    return {};
  }
}

function saveFilters() {
  localStorage.setItem(
    FILTERS_KEY,
    JSON.stringify({
      popularity: state.popularity,
      weirdness: state.weirdness,
      onSale: state.onSale,
      hideAdult: state.hideAdult,
      hideGore: state.hideGore,
      skipShovelware: state.skipShovelware,
      minReviews: state.minReviews,
      maxReviews: state.maxReviews,
      minPositive: state.minPositive,
      includeTags: state.includeTags,
      excludeTags: state.excludeTags,
      tagWeights: state.tagWeights,
      advancedOpen: state.advancedOpen,
      libraryOpen: state.libraryOpen,
    }),
  );
  updateAdvancedCount();
}

function parseOptionalInt(value) {
  const text = String(value ?? "").trim();
  if (!text) return null;
  const n = Number(text);
  if (!Number.isFinite(n) || n < 0) return null;
  return Math.round(n);
}

const FILTER_DEFAULTS = {
  popularity: 0.5,
  weirdness: 0.25,
  onSale: false,
  hideAdult: false,
  hideGore: false,
  skipShovelware: true,
  minReviews: null,
  maxReviews: null,
  minPositive: null,
  advancedOpen: false,
  libraryOpen: false,
};

const state = {
  player: null,
  games: [],
  recs: null,
  taste: null,
  wishlist: [],
  hidden: loadHidden(),
  filter: "all",
  view: "cards",
  search: "",
  sort: "hours",
  includeTags: [],
  excludeTags: [],
  tagWeights: [],
  ...FILTER_DEFAULTS,
};

function resetOptions() {
  Object.assign(state, FILTER_DEFAULTS);
  state.includeTags = [];
  state.excludeTags = [];
  state.tagWeights = [];
  state.hidden = [];
  state.recs = null;
  state.taste = null;
  state.wishlist = [];
  state.games = [];
  state.player = null;
  state.search = "";
  state.filter = "all";
  saveHidden([]);
  saveFilters();
  applyFiltersToDom();
  recGroupsEl.replaceChildren();
  tasteBarsEl.replaceChildren();
  gamesEl.replaceChildren();
  tasteLine.textContent = "";
  searchInput.value = "";
  document.querySelectorAll("[data-filter]").forEach((el) => {
    el.classList.toggle("active", el.dataset.filter === "all");
  });
}

{
  const saved = loadFilters();
  if (Number.isFinite(saved.popularity)) state.popularity = saved.popularity;
  if (Number.isFinite(saved.weirdness)) state.weirdness = saved.weirdness;
  if (typeof saved.onSale === "boolean") state.onSale = saved.onSale;
  if (typeof saved.hideAdult === "boolean") state.hideAdult = saved.hideAdult;
  if (typeof saved.hideGore === "boolean") state.hideGore = saved.hideGore;
  if (typeof saved.skipShovelware === "boolean") state.skipShovelware = saved.skipShovelware;
  if (saved.minReviews != null) state.minReviews = parseOptionalInt(saved.minReviews);
  if (saved.maxReviews != null) state.maxReviews = parseOptionalInt(saved.maxReviews);
  if (saved.minPositive != null) state.minPositive = parseOptionalInt(saved.minPositive);
  if (Array.isArray(saved.includeTags)) state.includeTags = saved.includeTags.filter((t) => typeof t === "string" && t.trim());
  if (Array.isArray(saved.excludeTags)) state.excludeTags = saved.excludeTags.filter((t) => typeof t === "string" && t.trim());
  if (Array.isArray(saved.tagWeights)) state.tagWeights = normalizeTagWeights(saved.tagWeights);
  if (typeof saved.advancedOpen === "boolean") state.advancedOpen = saved.advancedOpen;
  if (typeof saved.libraryOpen === "boolean") state.libraryOpen = saved.libraryOpen;
}

function applyFiltersToDom() {
  try {
    if (popularityDial) popularityDial.value = String(Math.round(state.popularity * 100));
    if (weirdnessDial) weirdnessDial.value = String(Math.round(state.weirdness * 100));
    if (saleDial) saleDial.checked = state.onSale;
    if (adultDial) adultDial.checked = state.hideAdult;
    if (goreDial) goreDial.checked = state.hideGore;
    if (shovelDial) shovelDial.checked = state.skipShovelware;
    if (minReviewsInput) {
      minReviewsInput.value = state.minReviews ?? "";
      minReviewsInput.placeholder = state.skipShovelware ? "500" : "none";
    }
    if (maxReviewsInput) maxReviewsInput.value = state.maxReviews ?? "";
    if (minPositiveInput) {
      minPositiveInput.value = state.minPositive ?? "";
      minPositiveInput.placeholder = state.skipShovelware ? "70" : "none";
    }
    renderTagChips();
    if (distanceField) distanceField.hidden = !state.games.length;
    if (advancedSearchEl) advancedSearchEl.hidden = false;
    if (libraryToggle && libraryBody) setDisclosure(libraryToggle, libraryBody, state.libraryOpen);
    updateAdvancedCount();
  } catch {
    /* homepage can run without the workspace controls */
  }
}

function setDisclosure(button, panel, open) {
  panel.hidden = !open;
  button.setAttribute("aria-expanded", String(open));
}

function advancedFilterCount() {
  let n = 0;
  if (!state.skipShovelware) n += 1;
  if (state.hideAdult) n += 1;
  if (state.hideGore) n += 1;
  if (state.minReviews != null) n += 1;
  if (state.maxReviews != null) n += 1;
  if (state.minPositive != null) n += 1;
  n += state.includeTags.length + state.excludeTags.length + state.tagWeights.length;
  return n;
}

function updateAdvancedCount() {}

const DEMO = {
  player: {
    steamid: "76561198000000000",
    name: "Demo Librarian",
    avatar: "",
    profileUrl: "https://steamcommunity.com/",
  },
  games: [
    game(730, "Counter-Strike 2", 812, 6.4, daysAgo(1), true, ["Action", "FPS"]),
    game(220, "Half-Life 2", 41, 0, daysAgo(400), false, ["Action", "Story Rich"]),
    game(413150, "Stardew Valley", 186, 2.1, daysAgo(3), true, ["Simulation", "RPG"]),
    game(1145360, "Hades", 74, 0, daysAgo(40), false, ["Action", "Roguelike"]),
    game(292030, "The Witcher 3: Wild Hunt", 129, 0, daysAgo(220), false, ["RPG", "Open World"]),
    game(1245620, "ELDEN RING", 97, 0, daysAgo(90), false, ["RPG", "Souls-like"]),
    game(1174180, "Red Dead Redemption 2", 68, 0, daysAgo(500), false, ["Action", "Adventure"]),
    game(105600, "Terraria", 53, 0.8, daysAgo(8), true, ["Adventure", "Sandbox"]),
    game(367520, "Hollow Knight", 32, 0, daysAgo(150), false, ["Action", "Metroidvania"]),
    game(646570, "Slay the Spire", 61, 1.2, daysAgo(5), true, ["Strategy", "Card Game"]),
    game(892970, "Valheim", 44, 0, daysAgo(300), false, ["Survival", "Open World"]),
    game(1627720, "Lies of P", 0, 0, 0, false, ["Action", "Souls-like"]),
    game(1940350, "Nine Sols", 0, 0, 0, false, ["Action", "Metroidvania"]),
    game(1086940, "Baldur's Gate 3", 155, 4.7, daysAgo(2), true, ["RPG", "Strategy"]),
  ],
};

function game(appid, name, hours, hours2Weeks, lastPlayed, recentlyPlayed, genres) {
  return {
    appid,
    name,
    icon: "",
    header: `/images/${appid}.jpg`,
    minutes: Math.round(hours * 60),
    hours,
    minutes2Weeks: Math.round(hours2Weeks * 60),
    hours2Weeks,
    lastPlayed,
    recentlyPlayed,
    pageUrl: `/app/${appid}`,
    steamUrl: `https://store.steampowered.com/app/${appid}/`,
    genres,
    developers: [],
    publishers: [],
    releaseDate: "",
    metacritic: null,
  };
}

function daysAgo(days) {
  return Math.floor(Date.now() / 1000) - days * 86400;
}

let statusTimer = 0;
let accountPage = "overview";
let storefrontCache = null;

function dismissStatus() {
  window.clearTimeout(statusTimer);
  statusEl.hidden = true;
  statusEl.textContent = "";
  statusEl.classList.remove("fading", "error");
}

function setStatus(message, isError = false) {
  window.clearTimeout(statusTimer);
  statusEl.hidden = !message;
  statusEl.textContent = message;
  statusEl.classList.toggle("error", isError);
  statusEl.classList.remove("fading");
  if (!message || message === "Talking to Steam…") return;
  const hold = isError ? 4200 : 2800;
  statusTimer = window.setTimeout(() => {
    statusEl.classList.add("fading");
    statusTimer = window.setTimeout(dismissStatus, 450);
  }, hold);
}

statusEl.addEventListener("click", dismissStatus);

function formatHours(hours) {
  if (!hours) return "Unplayed";
  if (hours < 1) return `${Math.round(hours * 60)}m`;
  return `${hours.toFixed(hours >= 10 ? 0 : 1)}h`;
}

function formatDate(unix) {
  if (!unix) return "Never";
  return new Date(unix * 1000).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function gamePageUrl(item) {
  if (item?.pageUrl) return item.pageUrl;
  if (item?.appid) return `/app/${item.appid}`;
  return item?.steamUrl || "#";
}

function csvEscape(value) {
  const text = value == null ? "" : String(value);
  if (/[",\n]/.test(text)) return `"${text.replaceAll('"', '""')}"`;
  return text;
}

function isRecentlyPlayed(game) {
  if (game.recentlyPlayed || game.hours2Weeks) return true;
  if (game.lastPlayed && Date.now() / 1000 - game.lastPlayed <= 14 * 86400) return true;
  return false;
}

function visibleGames() {
  const q = state.search.trim().toLowerCase();
  let list = state.games.filter((game) => {
    if (state.filter === "recent" && !isRecentlyPlayed(game)) return false;
    if (state.filter === "played" && game.minutes <= 0) return false;
    if (state.filter === "unplayed" && game.minutes > 0) return false;
    if (!q) return true;
    const hay = [game.name, ...(game.tags || []), ...(game.genres || [])].join(" ").toLowerCase();
    return hay.includes(q);
  });

  const sorters = {
    hours: (a, b) => b.minutes - a.minutes || a.name.localeCompare(b.name),
    recent: (a, b) => (b.lastPlayed || 0) - (a.lastPlayed || 0) || b.minutes - a.minutes,
    name: (a, b) => a.name.localeCompare(b.name),
    genre: (a, b) =>
      (a.tags?.[0] || a.genres?.[0] || "zzz").localeCompare(b.tags?.[0] || b.genres?.[0] || "zzz") ||
      b.minutes - a.minutes,
  };
  return list.sort(sorters[state.sort] || sorters.hours);
}

function genreWeights() {
  const counts = new Map();
  for (const game of state.games) {
    if (!game.minutes) continue;
    const labels = game.tags?.length ? game.tags : game.genres || [];
    for (const label of labels) {
      counts.set(label, (counts.get(label) || 0) + game.hours);
    }
  }
  return [...counts.entries()].sort((a, b) => b[1] - a[1]);
}

function heaviestGenre() {
  const top =
    state.recs?.taste?.clusters?.[0] ||
    state.taste?.clusters?.[0] ||
    null;
  if (top?.name) return top.name;
  return genreWeights()[0]?.[0] || "Pending";
}

function sparklineSvg(values) {
  if (!values || values.length < 2) return null;
  const w = 220;
  const h = 28;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = Math.max(1, max - min);
  const pts = values
    .map((v, i) => {
      const x = (i / (values.length - 1)) * (w - 2) + 1;
      const y = h - 2 - ((v - min) / span) * (h - 4);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "sparkline");
  svg.setAttribute("viewBox", `0 0 ${w} ${h}`);
  svg.setAttribute("aria-hidden", "true");
  const poly = document.createElementNS("http://www.w3.org/2000/svg", "polyline");
  poly.setAttribute("fill", "none");
  poly.setAttribute("stroke", "currentColor");
  poly.setAttribute("stroke-width", "2");
  poly.setAttribute("points", pts);
  svg.append(poly);
  return svg;
}

function decorateCover(cover, { url, saleText, appid } = {}) {
  cover.replaceChildren();
  cover.hidden = false;
  const fallbacks = [];
  const push = (src) => {
    if (src && !fallbacks.includes(src)) fallbacks.push(src);
  };
  if (appid) push(`/images/${appid}.jpg`);
  push(url);
  if (appid) {
    const id = String(appid);
    push(`https://cdn.akamai.steamstatic.com/steam/apps/${id}/header.jpg`);
    push(`https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/${id}/header.jpg`);
    push(`https://cdn.cloudflare.steamstatic.com/steam/apps/${id}/header.jpg`);
    push(`https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/${id}/capsule_616x353.jpg`);
    push(`https://cdn.akamai.steamstatic.com/steam/apps/${id}/capsule_616x353.jpg`);
    push(`https://cdn.akamai.steamstatic.com/steam/apps/${id}/library_hero.jpg`);
    push(`https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/${id}/library_hero.jpg`);
  }
  if (!fallbacks.length) {
    cover.hidden = true;
  } else {
    const img = document.createElement("img");
    img.className = "cover-photo";
    img.alt = "";
    img.loading = "lazy";
    let idx = 0;
    img.src = fallbacks[0];
    img.addEventListener("error", () => {
      idx += 1;
      if (idx < fallbacks.length) img.src = fallbacks[idx];
      else cover.hidden = true;
    });
    cover.append(img);
  }
  if (saleText) {
    const badge = document.createElement("span");
    badge.className = "sale-badge";
    badge.textContent = saleText;
    cover.append(badge);
  }
}

function sessionMetaText() {
  const games = state.games;
  const hours = games.reduce((sum, game) => sum + game.hours, 0);
  const genre = heaviestGenre();
  const parts = [
    `${games.length.toLocaleString()} games`,
    `${Math.round(hours).toLocaleString()} hours`,
  ];
  if (genre && genre !== "Pending") parts.push(genre.replaceAll("-", " "));
  return parts.join(" · ");
}

function renderPlayer() {
  const player = state.player;
  playerCard.replaceChildren();
  if (!player) {
    const copy = document.createElement("div");
    const name = document.createElement("h2");
    name.textContent = "Browse without a library";
    const meta = document.createElement("p");
    meta.className = "player-meta";
    meta.textContent = "No library loaded. Sorting from prefer / avoid tags and filters.";
    copy.append(name, meta);
    playerCard.append(copy);
    return;
  }
  const img = document.createElement("img");
  img.alt = "";
  img.src =
    player.avatar ||
    "data:image/svg+xml," +
      encodeURIComponent(
        `<svg xmlns='http://www.w3.org/2000/svg' width='64' height='64'><rect fill='#2a231b' width='64' height='64'/><text x='50%' y='54%' text-anchor='middle' fill='#f3eadc' font-size='18'>SP</text></svg>`
      );
  const copy = document.createElement("div");
  const name = document.createElement("h2");
  name.textContent = player.name;
  const meta = document.createElement("p");
  meta.className = "player-meta";
  meta.textContent = sessionMetaText();
  copy.append(name, meta);
  playerCard.append(img, copy);
}

function renderStats() {
  const meta = playerCard.querySelector(".player-meta");
  if (meta) meta.textContent = sessionMetaText();
}

function renderGames() {
  const games = visibleGames();
  gamesEl.replaceChildren();
  if (!games.length) {
    const empty = document.createElement("p");
    empty.className = "hint";
    empty.textContent = "No games match this filter.";
    gamesEl.append(empty);
    return;
  }

  if (state.view === "table") {
    renderTable(games);
    return;
  }

  gamesEl.className = "game-grid";
  for (const game of games) {
    const card = document.createElement("article");
    card.className = "game-card";
    const cover = document.createElement("div");
    cover.className = "cover";
    decorateCover(cover, { url: game.header });
    const body = document.createElement("div");
    body.className = "card-body";
    const title = document.createElement("h3");
    title.textContent = game.name;
    const hours = document.createElement("div");
    hours.className = "hours";
    hours.textContent = game.lastPlayed
      ? `${formatHours(game.hours)} · last ${formatDate(game.lastPlayed)}`
      : formatHours(game.hours);
    const pills = document.createElement("div");
    pills.className = "pills";
    if (isRecentlyPlayed(game)) {
      const pill = document.createElement("span");
      pill.className = "pill recent";
      pill.textContent = "Recent";
      pills.append(pill);
    }
    for (const label of (game.tags || game.genres || []).slice(0, 3)) {
      const pill = document.createElement("span");
      pill.className = "pill";
      pill.textContent = label;
      pills.append(pill);
    }
    body.append(title, hours, pills);
    card.append(cover, body);
    gamesEl.append(card);
  }
}

function renderTable(games) {
  gamesEl.className = "table-wrap";
  const table = document.createElement("table");
  table.innerHTML = `
    <thead>
      <tr>
        <th>Game</th>
        <th class="num">Hours</th>
        <th class="num">2 weeks</th>
        <th>Last played</th>
        <th>Genres</th>
      </tr>
    </thead>
  `;
  const tbody = document.createElement("tbody");
  for (const game of games) {
    const tr = document.createElement("tr");
    const name = document.createElement("td");
    const link = document.createElement("a");
    link.href = gamePageUrl(game);
    link.textContent = game.name;
    name.append(link);
    const hours = document.createElement("td");
    hours.className = "num";
    hours.textContent = formatHours(game.hours);
    const recent = document.createElement("td");
    recent.className = "num";
    recent.textContent = game.hours2Weeks ? formatHours(game.hours2Weeks) : "—";
    const last = document.createElement("td");
    last.textContent = game.lastPlayed ? formatDate(game.lastPlayed) : "—";
    const genres = document.createElement("td");
    genres.textContent = (game.tags || game.genres || []).join(", ") || "…";
    tr.append(name, hours, recent, last, genres);
    tbody.append(tr);
  }
  table.append(tbody);
  gamesEl.append(table);
}

function nameKey(name) {
  return String(name || "")
    .toLowerCase()
    .replace(/[™®©]/g, "")
    .replace(/[-–—_:]+/g, " ")
    .replace(/[^a-z0-9\u00c0-\u024f]+/gi, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function isOwnedRec(item) {
  const ownedIds = new Set(state.games.map((game) => game.appid));
  if (ownedIds.has(item.appid)) return true;
  const key = nameKey(item.name);
  if (!key) return false;
  return state.games.some((game) => nameKey(game.name) === key);
}

function renderRecs() {
  const recs = state.recs;
  if (!recs) {
    if (state.games.length) {
      recsEl.hidden = false;
      tasteProfileEl.hidden = false;
      if (!tasteLine.textContent) tasteLine.textContent = "Finding matches…";
    } else {
      recsEl.hidden = true;
      tasteProfileEl.hidden = true;
    }
    if (!document.body.classList.contains("account")) searchCriteriaEl.hidden = true;
    return;
  }
  recsEl.hidden = false;
  searchCriteriaEl.hidden = false;
  const recsTitle = document.querySelector("#recs-title");
  if (recsTitle) recsTitle.textContent = state.games.length ? "What to play next" : "Games in this sift";
  const clusters = recs.taste?.clusters || [];
  const intro = clusters.length
    ? "From your playtime. Click a bar to prefer it; shift-click to avoid."
    : state.games.length
      ? "From games you’ve played."
      : "Prefer / avoid tags rank the list; include / exclude cut it.";
  tasteLine.replaceChildren();
  tasteLine.append(document.createTextNode(intro));
  if (recs.pool) {
    const pool = document.createElement("span");
    pool.className = "taste-pool";
    pool.textContent = `${recs.pool.toLocaleString()} games still in play.`;
    tasteLine.append(document.createTextNode(" "), pool);
  }
  tasteProfileEl.hidden = false;
  tasteBarsEl.replaceChildren();
  const shown = clusters.slice(0, 6);
  const maxPct = Math.max(1, ...shown.map((cluster) => clusterPercent(cluster, clusters)));
  for (const cluster of shown) {
    const name = cluster.name.replaceAll("-", " ");
    const pct = clusterPercent(cluster, clusters);
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "taste-bar";
    if (tagIn(state.includeTags, name)) btn.classList.add("include");
    if (tagIn(state.excludeTags, name)) btn.classList.add("exclude");
    if (state.tagWeights.some((item) => tagKey(item.name) === tagKey(name) && item.weight > 0)) btn.classList.add("include");
    if (state.tagWeights.some((item) => tagKey(item.name) === tagKey(name) && item.weight < 0)) btn.classList.add("exclude");
    btn.title = "Click to prefer this tag. Shift-click to avoid it.";
    const label = document.createElement("span");
    label.className = "taste-name";
    const tagName = document.createElement("span");
    tagName.textContent = name;
    label.append(tagName);
    const track = document.createElement("span");
    track.className = "taste-track";
    const fill = document.createElement("span");
    fill.className = "taste-fill";
    fill.style.width = `${Math.round((pct / maxPct) * 100)}%`;
    track.append(fill);
    btn.append(label, track);
    btn.addEventListener("click", (event) => {
      toggleWeightedTag(name, event.shiftKey);
    });
    tasteBarsEl.append(btn);
  }
  recGroupsEl.replaceChildren();
  const groups = [
    ["more_like", "More like your library"],
    ["adjacent", "Nearby experiments"],
    ["wildcard", "Wildcards"],
  ];
  for (const [key, title] of groups) {
    const items = (recs.groups?.[key] || []).filter((item) => !isOwnedRec(item));
    if (!items.length) continue;
    const section = document.createElement("div");
    section.className = "rec-group";
    const heading = document.createElement("h3");
    heading.textContent = title;
    const count = document.createElement("span");
    count.className = "rec-count";
    count.textContent = String(items.length);
    heading.append(count);
    const grid = document.createElement("div");
    grid.className = items.length === 1 ? "rec-grid rec-grid-one" : "rec-grid";
    for (const item of items) {
      const card = document.createElement("article");
      card.className = "game-card";
      const cover = document.createElement("a");
      cover.className = "cover";
      cover.href = gamePageUrl(item);
      cover.setAttribute("aria-label", item.name);
      decorateCover(cover, {
        url: item.header,
        appid: item.appid,
        saleText: item.onSale && item.discount
          ? item.price
            ? `−${item.discount}% · ${item.price}`
            : `−${item.discount}%`
          : "",
      });
      const body = document.createElement("div");
      body.className = "card-body";
      const name = document.createElement("h3");
      const link = document.createElement("a");
      link.href = gamePageUrl(item);
      link.textContent = item.name;
      name.append(link);
      const hours = document.createElement("div");
      hours.className = "hours";
      hours.textContent = (item.clusters.slice(0, 2).join(" · ") || "fit").replaceAll("-", " ");
      if (item.positive) {
        hours.textContent += ` · ${item.positive}%`;
      }
      if (item.reviews) {
        hours.textContent += ` · ${item.reviews.toLocaleString()} reviews`;
      }
      if (item.onWishlist) hours.textContent += " · wishlist";
      if (item.atLow) hours.textContent += " · lowest we've seen";
      else if (item.lowPrice && item.onSale) hours.textContent += ` · low ${item.lowPrice}`;
      const reason = document.createElement("p");
      reason.className = "reason";
      reason.textContent = item.reason;
      body.append(name, hours, reason);
      const spark = sparklineSvg(item.sparkline);
      if (spark) body.append(spark);
      const hide = document.createElement("button");
      hide.type = "button";
      hide.className = "hide-rec";
      hide.textContent = "Not for me";
      hide.addEventListener("click", () => hideGame(item.appid));
      body.append(hide);
      card.append(cover, body);
      grid.append(card);
    }
    section.append(heading, grid);
    recGroupsEl.append(section);
  }
  if (!recGroupsEl.children.length) {
    const empty = document.createElement("p");
    empty.className = "hint";
    empty.textContent = emptyRecsMessage();
    recGroupsEl.append(empty);
  }
}

function emptyRecsMessage() {
  if (state.onSale) {
    return "None of these picks are on sale right now. Turn the sale filter off, or slide toward Mainstream.";
  }
  if (state.skipShovelware || state.minReviews || state.minPositive || state.includeTags.length || state.excludeTags.length) {
    return "Nothing left after those filters. Try fewer limits — junk skip, review count, or tags.";
  }
  return "No unowned games left to rank. That shouldn’t happen when you only move the sliders.";
}

function clusterPercent(cluster, all = []) {
  const n = Number(cluster?.percent);
  if (Number.isFinite(n) && n > 0) return n;
  const total = all.reduce((sum, item) => sum + (Number(item.hours) || 0), 0);
  if (!total) return 0;
  return Math.round((100 * (Number(cluster?.hours) || 0)) / total);
}

function tagKey(name) {
  return String(name || "").trim().toLowerCase();
}

function tagIn(list, name) {
  const key = tagKey(name);
  return list.some((item) => tagKey(item) === key);
}

function renderTagChips() {
  paintChips(includeChipsEl, state.includeTags, "include");
  paintChips(excludeChipsEl, state.excludeTags, "exclude");
  paintChips(preferChipsEl, preferTagNames(), "prefer");
  paintChips(avoidChipsEl, avoidTagNames(), "avoid");
}

const PREFER_WEIGHT = 80;
const AVOID_WEIGHT = -80;

function preferTagNames() {
  return state.tagWeights.filter((item) => item.weight > 0).map((item) => item.name);
}

function avoidTagNames() {
  return state.tagWeights.filter((item) => item.weight < 0).map((item) => item.name);
}

function normalizeTagWeights(list) {
  const seen = new Map();
  const out = [];
  for (const item of list || []) {
    const name = String(item?.name || "").trim();
    const raw = Number(item?.weight);
    if (!name || !Number.isFinite(raw) || raw === 0) continue;
    const weight = raw > 0 ? PREFER_WEIGHT : AVOID_WEIGHT;
    const key = tagKey(name);
    if (seen.has(key)) {
      out[seen.get(key)].weight = weight;
      continue;
    }
    seen.set(key, out.length);
    out.push({ name, weight });
  }
  return out;
}

function addTagWeight(name, weight) {
  const label = String(name || "").trim();
  if (!label) return;
  state.includeTags = state.includeTags.filter((item) => tagKey(item) !== tagKey(label));
  state.excludeTags = state.excludeTags.filter((item) => tagKey(item) !== tagKey(label));
  if (!weight) {
    state.tagWeights = state.tagWeights.filter((item) => tagKey(item.name) !== tagKey(label));
  } else {
    state.tagWeights = normalizeTagWeights([...state.tagWeights, { name: label, weight }]);
  }
  saveFilters();
  renderTagChips();
  if (state.recs) renderRecs();
  queueRecs();
}

function toggleWeightedTag(name, avoid) {
  const current = state.tagWeights.find((item) => tagKey(item.name) === tagKey(name));
  if (avoid) {
    if (current && current.weight < 0) addTagWeight(name, 0);
    else addTagWeight(name, AVOID_WEIGHT);
    return;
  }
  if (current && current.weight > 0) addTagWeight(name, 0);
  else addTagWeight(name, PREFER_WEIGHT);
}

function commitTagInput(input, weight) {
  const label = input.value.trim();
  if (!label) return;
  addTagWeight(label, weight);
  input.value = "";
}

function paintChips(el, tags, kind) {
  if (!el) return;
  el.replaceChildren();
  for (const name of tags) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = `tag-chip ${kind}`;
    const label = document.createElement("span");
    label.textContent = name;
    const x = document.createElement("span");
    x.className = "x";
    x.setAttribute("aria-hidden", "true");
    x.textContent = "×";
    btn.append(label, x);
    btn.addEventListener("click", () => {
      if (kind === "prefer" || kind === "avoid") addTagWeight(name, 0);
      else toggleTag(kind, name, true);
    });
    el.append(btn);
  }
}

function toggleTag(kind, name, removeOnly = false) {
  const key = tagKey(name);
  if (!key) return;
  const label = String(name).trim();
  const other = kind === "include" ? "excludeTags" : "includeTags";
  const mine = kind === "include" ? "includeTags" : "excludeTags";
  state[other] = state[other].filter((item) => tagKey(item) !== key);
  state.tagWeights = state.tagWeights.filter((item) => tagKey(item.name) !== key);
  if (removeOnly || tagIn(state[mine], label)) {
    state[mine] = state[mine].filter((item) => tagKey(item) !== key);
  } else {
    state[mine] = [...state[mine], label];
  }
  saveFilters();
  renderTagChips();
  if (state.recs) renderRecs();
  queueRecs();
}

function bindTagPicker(input, suggestEl, kind) {
  let timer;
  let items = [];
  let active = -1;
  const applyPick = (name) => {
    if (typeof kind === "function") kind(name);
    else toggleTag(kind, name);
  };

  const hide = () => {
    suggestEl.hidden = true;
    suggestEl.replaceChildren();
    input.setAttribute("aria-expanded", "false");
    items = [];
    active = -1;
  };

  const paint = () => {
    suggestEl.replaceChildren();
    items.forEach((tag, i) => {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.textContent = tag.name;
      btn.className = i === active ? "active" : "";
      btn.addEventListener("mousedown", (event) => {
        event.preventDefault();
        applyPick(tag.name);
        input.value = "";
        hide();
      });
      suggestEl.append(btn);
    });
    suggestEl.hidden = !items.length;
    input.setAttribute("aria-expanded", items.length ? "true" : "false");
  };

  input.addEventListener("input", () => {
    clearTimeout(timer);
    const q = input.value.trim();
    if (q.length < 1) {
      hide();
      return;
    }
    timer = setTimeout(async () => {
      const res = await fetch(`/api/tags?q=${encodeURIComponent(q)}`);
      const data = await res.json().catch(() => []);
      items = Array.isArray(data) ? data.slice(0, 8) : [];
      active = items.length ? 0 : -1;
      paint();
    }, 160);
  });

  input.addEventListener("keydown", (event) => {
    if (event.key === "ArrowDown" && items.length) {
      event.preventDefault();
      active = (active + 1) % items.length;
      paint();
    } else if (event.key === "ArrowUp" && items.length) {
      event.preventDefault();
      active = (active - 1 + items.length) % items.length;
      paint();
    } else if (event.key === "Enter") {
      event.preventDefault();
      const pick = items[active];
      if (pick?.name) {
        applyPick(pick.name);
        input.value = "";
      }
      hide();
    } else if (event.key === "Escape") {
      hide();
    }
  });

  input.addEventListener("blur", () => setTimeout(hide, 120));
}

function signedIn() {
  return Boolean(config.user?.steamid);
}

function showView(view) {
  const onWorkspace = view === "workspace";
  const onProfile = view === "profile";
  document.body.classList.toggle("session", onWorkspace);
  document.body.classList.toggle("account", onProfile);
  workspace.hidden = !onWorkspace;
  accountEl.hidden = !onProfile;
  mountSearchCriteria(view);
  syncAuthChrome();
}

function mountSearchCriteria(view) {
  const slot = view === "profile" ? accountSearchSlot : recsSearchSlot;
  if (slot && searchCriteriaEl.parentElement !== slot) slot.append(searchCriteriaEl);
  const show = (view === "profile" && accountPage === "watchlists") || (view !== "profile" && Boolean(state.recs));
  searchCriteriaEl.hidden = !show;
  if (advancedSearchEl) advancedSearchEl.hidden = false;
}

function showAccountPage(page) {
  accountPage = page === "watchlists" ? "watchlists" : "overview";
  accountOverview.hidden = accountPage !== "overview";
  accountWatchlists.hidden = accountPage !== "watchlists";
  document.querySelectorAll("[data-account-page]").forEach((btn) => {
    const active = btn.dataset.accountPage === accountPage;
    btn.classList.toggle("active", active);
    if (active) btn.setAttribute("aria-current", "page");
    else btn.removeAttribute("aria-current");
  });
  if (document.body.classList.contains("account")) mountSearchCriteria("profile");
}

function syncAuthChrome() {
  if (topSignin) topSignin.hidden = signedIn();
  profileBtn.hidden = !signedIn();
  if (signedIn()) {
    changeProfileBtn.hidden = false;
    changeProfileBtn.textContent = "Sign out";
    return;
  }
  if (document.body.classList.contains("session")) {
    changeProfileBtn.hidden = false;
    changeProfileBtn.textContent = "Change profile";
    return;
  }
  changeProfileBtn.hidden = true;
}

function formatWhen(unix) {
  if (!unix) return "Unknown";
  const date = new Date(Number(unix) * 1000);
  if (Number.isNaN(date.getTime())) return "Unknown";
  const delta = Date.now() - date.getTime();
  if (delta < 120000) return "Just now";
  if (delta < 3600000) return `${Math.max(1, Math.round(delta / 60000))} minutes ago`;
  if (delta < 86400000) return `${Math.max(1, Math.round(delta / 3600000))} hours ago`;
  return date.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

function renderAccount() {
  const user = config.user || {};
  const name = user.name || "Steam account";
  accountName.textContent = name;
  accountInitials.textContent = (name.trim().charAt(0) || "?").toUpperCase();
  accountAvatar.onload = () => {
    accountAvatar.hidden = false;
  };
  accountAvatar.onerror = () => {
    accountAvatar.hidden = true;
    accountAvatar.removeAttribute("src");
  };
  if (user.avatar) {
    accountAvatar.hidden = true;
    accountAvatar.src = user.avatar;
  } else {
    accountAvatar.removeAttribute("src");
    accountAvatar.hidden = true;
  }
  accountSteamName.textContent = name;
  accountSteamID.textContent = user.steamid || "—";
  if (user.profileUrl || user.steamid) {
    accountProfileLink.href = user.profileUrl || `https://steamcommunity.com/profiles/${user.steamid}`;
    accountProfileLink.hidden = false;
  } else {
    accountProfileLink.removeAttribute("href");
    accountProfileLink.hidden = true;
  }
  accountLastLogin.textContent = formatWhen(user.lastLoginAt);
  accountJoined.textContent = formatWhen(user.createdAt);
  accountRecsBtn.hidden = !state.games.length;
  if (state.games.length) {
    accountNote.textContent = `${state.games.length.toLocaleString()} games loaded from your public library.`;
  } else {
    accountNote.textContent =
      "Game details are private, so we cannot load your library. You can still save searches.";
  }
}

function renderSpotlight(el, items, { sale = false, empty = "", metaOf } = {}) {
  el.replaceChildren();
  if (!items.length) {
    const emptyEl = document.createElement("p");
    emptyEl.className = "hint";
    emptyEl.textContent =
      empty ||
      (sale ? "No featured sales from Steam right now." : "No upcoming titles from Steam right now.");
    el.append(emptyEl);
    return;
  }
  for (const item of items.slice(0, 8)) {
    const card = document.createElement("article");
    card.className = "game-card";
    const cover = document.createElement("a");
    cover.className = "cover";
    cover.href = gamePageUrl(item);
    cover.setAttribute("aria-label", item.name);
    const saleText = sale && item.discount
      ? item.formatted
        ? `−${item.discount}% · ${item.formatted}`
        : `−${item.discount}%`
      : item.formatted || "";
    decorateCover(cover, { url: item.header, saleText });
    const body = document.createElement("div");
    body.className = "card-body";
    const name = document.createElement("h3");
    const link = document.createElement("a");
    link.href = gamePageUrl(item);
    link.textContent = item.name;
    name.append(link);
    const meta = document.createElement("div");
    meta.className = "hours";
    if (typeof metaOf === "function") {
      meta.textContent = metaOf(item) || "";
    } else {
      meta.textContent = sale && item.discount ? `${item.discount}% off` : "Coming soon on Steam";
    }
    body.append(name, meta);
    card.append(cover, body);
    el.append(card);
  }
}

async function loadHomeBrowse() {
  if (typeof window.__sipLoadHome === "function") {
    return window.__sipLoadHome();
  }
}

async function loadStorefront() {
  if (storefrontCache) {
    renderSpotlight(accountSales, storefrontCache.specials, { sale: true });
    renderSpotlight(accountUpcoming, storefrontCache.comingSoon);
    return;
  }
  accountSales.replaceChildren();
  accountUpcoming.replaceChildren();
  const loadingSales = document.createElement("p");
  loadingSales.className = "hint";
  loadingSales.textContent = "Loading Steam sales…";
  accountSales.append(loadingSales);
  const loadingSoon = document.createElement("p");
  loadingSoon.className = "hint";
  loadingSoon.textContent = "Loading upcoming games…";
  accountUpcoming.append(loadingSoon);
  try {
    const res = await fetch("/api/storefront");
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || "Could not load Steam storefront.");
    storefrontCache = {
      specials: data.specials || [],
      comingSoon: data.comingSoon || [],
    };
    renderSpotlight(accountSales, storefrontCache.specials, { sale: true });
    renderSpotlight(accountUpcoming, storefrontCache.comingSoon);
  } catch {
    renderSpotlight(accountSales, []);
    renderSpotlight(accountUpcoming, []);
  }
}

function currentSearchFilters() {
  return {
    popularity: state.popularity,
    weirdness: state.weirdness,
    onSale: state.onSale,
    hideAdult: state.hideAdult,
    hideGore: state.hideGore,
    skipShovelware: state.skipShovelware,
    minReviews: state.minReviews,
    maxReviews: state.maxReviews,
    minPositive: state.minPositive,
    includeTags: [...state.includeTags],
    excludeTags: [...state.excludeTags],
    tagWeights: state.tagWeights.map((item) => ({ ...item })),
  };
}

function applySearchFilters(filters = {}) {
  if (Number.isFinite(filters.popularity)) state.popularity = filters.popularity;
  if (Number.isFinite(filters.weirdness)) state.weirdness = filters.weirdness;
  if (typeof filters.onSale === "boolean") state.onSale = filters.onSale;
  if (typeof filters.hideAdult === "boolean") state.hideAdult = filters.hideAdult;
  if (typeof filters.hideGore === "boolean") state.hideGore = filters.hideGore;
  if (typeof filters.skipShovelware === "boolean") state.skipShovelware = filters.skipShovelware;
  state.minReviews = filters.minReviews != null ? parseOptionalInt(filters.minReviews) : null;
  state.maxReviews = filters.maxReviews != null ? parseOptionalInt(filters.maxReviews) : null;
  state.minPositive = filters.minPositive != null ? parseOptionalInt(filters.minPositive) : null;
  if (state.minPositive != null) state.minPositive = Math.min(100, state.minPositive);
  state.includeTags = Array.isArray(filters.includeTags) ? filters.includeTags.map(String) : [];
  state.excludeTags = Array.isArray(filters.excludeTags) ? filters.excludeTags.map(String) : [];
  state.tagWeights = Array.isArray(filters.tagWeights) ? normalizeTagWeights(filters.tagWeights) : [];
  saveFilters();
  applyFiltersToDom();
}

function splitTerms(terms) {
  return String(terms || "")
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

function defaultSearchTerms() {
  const parts = [
    ...state.includeTags,
    ...state.tagWeights.filter((item) => item.weight > 0).map((item) => item.name),
  ];
  if (state.search.trim()) parts.push(state.search.trim());
  return [...new Set(parts)].join(", ");
}

function searchKindLabel(kind) {
  return kind === "library" ? "Library" : "Sift";
}

function searchSummary(item) {
  const filters = item.filters || {};
  const parts = [searchKindLabel(item.kind)];
  if (item.terms) parts.push(item.terms);
  if (filters.onSale) parts.push("On sale");
  if (Number.isFinite(filters.popularity)) {
    parts.push(filters.popularity < 0.45 ? "Indie" : filters.popularity > 0.55 ? "Mainstream" : "Any audience");
  }
  if (Number.isFinite(filters.weirdness)) {
    parts.push(filters.weirdness < 0.2 ? "Familiar" : filters.weirdness > 0.4 ? "Weird" : "Mixed taste");
  }
  if (filters.tagWeights?.length) {
    const prefer = filters.tagWeights.filter((entry) => entry.weight > 0).map((entry) => entry.name);
    const avoid = filters.tagWeights.filter((entry) => entry.weight < 0).map((entry) => entry.name);
    if (prefer.length) parts.push(`prefer ${prefer.join(", ")}`);
    if (avoid.length) parts.push(`avoid ${avoid.join(", ")}`);
  }
  if (filters.excludeTags?.length) parts.push(`not ${filters.excludeTags.join(", ")}`);
  return parts.join(" · ");
}

async function loadSavedSearches() {
  if (!signedIn()) {
    savedSearchesEl.replaceChildren();
    return;
  }
  const res = await fetch("/api/searches", { credentials: "same-origin" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Could not load saved searches.");
  renderSavedSearches(data.searches || []);
}

function renderSavedSearches(items) {
  savedSearchesEl.replaceChildren();
  if (!items.length) {
    const empty = document.createElement("li");
    empty.className = "hint";
    empty.textContent = "No saved sifts yet.";
    savedSearchesEl.append(empty);
    return;
  }
  for (const item of items) {
    const li = document.createElement("li");
    li.className = "saved-search";
    const copy = document.createElement("div");
    const title = document.createElement("h4");
    title.textContent = item.name;
    const summary = document.createElement("p");
    summary.className = "hint";
    summary.textContent = searchSummary(item);
    copy.append(title, summary);
    const actions = document.createElement("div");
    actions.className = "saved-search-actions";
    const applyBtn = document.createElement("button");
    applyBtn.type = "button";
    applyBtn.className = "primary";
    applyBtn.textContent = "Show games";
    applyBtn.addEventListener("click", () => {
      applySearchFilters(item.filters || {});
      const terms = splitTerms(item.terms);
      if (item.kind === "library") {
        state.search = item.terms || "";
        searchInput.value = state.search;
        showView("workspace");
        renderGames();
        return;
      }
      if (terms.length) {
        const extra = terms
          .filter((term) => !tagIn(state.includeTags, term) && !state.tagWeights.some((entry) => tagKey(entry.name) === tagKey(term)))
          .map((name) => ({ name, weight: PREFER_WEIGHT }));
        if (extra.length) {
          state.tagWeights = normalizeTagWeights([...state.tagWeights, ...extra]);
          saveFilters();
          applyFiltersToDom();
        }
      }
      runSift();
    });
    const del = document.createElement("button");
    del.type = "button";
    del.className = "ghost";
    del.textContent = "Delete";
    del.addEventListener("click", async () => {
      const res = await fetch(`/api/searches/${item.id}`, { method: "DELETE", credentials: "same-origin" });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        setStatus(data.error || "Could not delete search.", true);
        return;
      }
      await loadSavedSearches();
    });
    actions.append(applyBtn, del);
    li.append(copy, actions);
    savedSearchesEl.append(li);
  }
}

async function openProfile() {
  dismissStatus();
  renderAccount();
  if (searchNameInput && !searchNameInput.value) {
    searchNameInput.value = defaultSearchTerms();
  }
  showAccountPage(accountPage);
  showView("profile");
  try {
    await Promise.all([loadSavedSearches(), loadStorefront()]);
  } catch (err) {
    setStatus(err.message, true);
  }
}

function renderAll() {
  showView("workspace");
  renderPlayer();
  renderStats();
  renderRecs();
  renderGames();
  if (libraryToggle) libraryToggle.hidden = !state.games.length;
  const libraryPanel = document.querySelector("#library-panel");
  if (libraryPanel) libraryPanel.hidden = !state.games.length;
  if (libraryBody && !state.games.length) {
    libraryBody.hidden = true;
    if (libraryToggle) libraryToggle.setAttribute("aria-expanded", "false");
  }
}

function canRunSift() {
  return (
    state.games.length > 0 ||
    state.includeTags.length > 0 ||
    state.tagWeights.some((item) => item.weight > 0)
  );
}

function runSift() {
  if (!canRunSift()) {
    setStatus("Add a prefer or include tag, or load a public library.", true);
    return;
  }
  showView("workspace");
  loadRecommendations();
}

async function loadRecommendations() {
  if (!canRunSift()) return;
  try {
    tasteLine.textContent = "Finding matches…";
    const res = await fetch("/api/recommend", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        games: state.games.map((game) => ({
          appid: game.appid,
          name: game.name,
          hours: game.hours,
          recentlyPlayed: game.recentlyPlayed,
          genres: game.genres || [],
        })),
        popularity: state.popularity,
        weirdness: state.weirdness,
        onSale: state.onSale,
        hideAdult: state.hideAdult,
        hideGore: state.hideGore,
        skipShovelware: state.skipShovelware,
        minReviews: state.minReviews,
        maxReviews: state.maxReviews,
        minPositive: state.minPositive,
        includeTags: state.includeTags,
        excludeTags: state.excludeTags,
        tagWeights: state.tagWeights,
        hidden: state.hidden,
        wishlist: state.wishlist,
      }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "Recommendation failed.");
    state.recs = data;
    if (data.taste) state.taste = data.taste;
    renderAll();
  } catch (err) {
    tasteLine.textContent = err.message;
    recsEl.hidden = false;
  }
}

function toCsv(games = state.games) {
  const headers = [
    "appid",
    "name",
    "hours_total",
    "hours_2weeks",
    "last_played",
    "recently_played",
    "unplayed",
    "genres",
    "developers",
    "publishers",
    "release_date",
    "metacritic",
    "steam_url",
  ];
  const rows = games.map((game) =>
    [
      game.appid,
      game.name,
      game.hours,
      game.hours2Weeks,
      game.lastPlayed ? new Date(game.lastPlayed * 1000).toISOString().slice(0, 10) : "",
      isRecentlyPlayed(game) ? "yes" : "no",
      game.minutes ? "no" : "yes",
      (game.genres || []).join("; "),
      (game.developers || []).join("; "),
      (game.publishers || []).join("; "),
      game.releaseDate || "",
      game.metacritic ?? "",
      game.steamUrl,
    ]
      .map(csvEscape)
      .join(",")
  );
  return [headers.join(","), ...rows].join("\n");
}

function llmPrompt() {
  const top = [...state.games].filter((g) => g.minutes > 0).slice(0, 25);
  const recent = state.games.filter((g) => isRecentlyPlayed(g));
  const tasteTags = (state.recs?.taste?.clusters || state.taste?.clusters || [])
    .slice(0, 8)
    .map((c) => `${c.name} (${Math.round(c.hours)}h)`)
    .join(", ");
  const genres = tasteTags || genreWeights()
    .slice(0, 8)
    .map(([genre, hours]) => `${genre} (${hours.toFixed(0)}h)`)
    .join(", ");
  const owned = state.games.map((g) => g.name).join(" | ");
  return `You are recommending video games I do not already own.

Taste signals from my Steam library:
- Heaviest tags by hours: ${genres || "unknown"}
- Most played: ${top.map((g) => `${g.name} (${g.hours}h, ${(g.tags || g.genres || []).join("/")} )`).join("; ") || "none"}
- Played in the last two weeks: ${recent.map((g) => g.name).join(", ") || "none"}

Do not recommend anything in this owned list:
${owned}

Using the attached CSV (or the owned list above if no CSV is available), suggest 12 games:
1. Skip titles I already own.
2. Weight recommendations toward games I have actually played, especially high hours and recent activity. Unplayed owned games are backlog, not taste.
3. Give Steam-available titles first, with a one-line reason that cites a comparable game from my library.
4. Group them into: more of what I already like, adjacent experiments, and one wildcard.

CSV of the full library follows:

${toCsv()}`;
}

function download(filename, text, type = "text/plain") {
  const blob = new Blob([text], { type });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.rel = "noopener";
  a.style.display = "none";
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Revoke after the click has a chance to start the download.
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}

async function enrichGenres(games) {
  const pending = games
    .filter((game) => !game.genres)
    .sort((a, b) => b.minutes - a.minutes);
  if (!pending.length) {
    enrichStatus.textContent = "Genres are ready.";
    return;
  }
  enrichStatus.textContent = `Loading genres for ${pending.length} games…`;
  const chunkSize = 12;
  for (let i = 0; i < pending.length; i += chunkSize) {
    const chunk = pending.slice(i, i + chunkSize);
    const res = await fetch("/api/genres", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ appids: chunk.map((game) => game.appid) }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || "Genre lookup failed.");
    for (const game of chunk) {
      const info = data.details?.[game.appid];
      if (!info) continue;
      game.genres = info.genres || [];
      game.developers = info.developers || [];
      game.publishers = info.publishers || [];
      game.releaseDate = info.releaseDate || "";
      game.metacritic = info.metacritic ?? null;
    }
    enrichStatus.textContent = `Loaded genres for ${Math.min(i + chunkSize, pending.length)} / ${pending.length} games.`;
    renderAll();
  }
  enrichStatus.textContent = "Genres are ready. Export whenever you want a second opinion.";
}

async function fetchLibrary(identifier) {
  const body = { identifier };
  if (apiKeyField && !apiKeyField.hidden && apiKeyInput?.value) {
    body.apiKey = apiKeyInput.value;
  }
  const res = await fetch("/api/library", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Could not load library.");
  return data;
}

function applyLibrary(data) {
  state.player = data.player;
  state.games = Array.isArray(data.games) ? data.games : [];
  state.taste = data.taste || null;
  state.wishlist = data.wishlist || [];
  state.libraryOpen = state.games.length > 0;
  setStatus(`Loaded ${state.games.length} games.`);
  applyFiltersToDom();
  renderAll();
  loadRecommendations();
}

async function loadLibrary(event) {
  event?.preventDefault?.();
  const identifier =
    (typeof event === "string" ? event : "") ||
    pendingGuestIdentifier ||
    identifierInput?.value?.trim() ||
    "";
  if (!identifier) {
    setStatus("Type a Steam username or profile URL in the top search.", true);
    document.querySelector("[data-top-search-input]")?.focus();
    return;
  }
  if (loadBtn) loadBtn.disabled = true;
  setStatus("Talking to Steam…");
  try {
    const data = await fetchLibrary(identifier);
    applyLibrary(data);
    try {
      sessionStorage.setItem("steam-identifier", identifier);
    } catch {
      /* guest / private profiles may block storage */
    }
  } catch (err) {
    setStatus(err.message, true);
  } finally {
    if (loadBtn) loadBtn.disabled = false;
  }
}
window.__sipLoad = loadLibrary;
window.__sipApply = applyLibrary;

async function loadSignedInLibrary() {
  try {
    applyLibrary(await fetchLibrary(""));
    return true;
  } catch {
    dismissStatus();
    await openProfile();
    return false;
  }
}

function hideGame(appid) {
  if (!state.hidden.includes(appid)) {
    state.hidden = [...state.hidden, appid];
    saveHidden(state.hidden);
  }
  queueRecs();
}

function loadDemo() {
  state.player = DEMO.player;
  state.games = DEMO.games.map((game) => ({ ...game, genres: [...game.genres] }));
  state.wishlist = [];
  setStatus("Showing demo library. Connect a real profile whenever you are ready.");
  enrichStatus.textContent = "Demo library — recommendations are scored locally.";
  renderAll();
  loadRecommendations();
}

document.querySelectorAll("[data-filter]").forEach((btn) => {
  btn.addEventListener("click", () => {
    state.filter = btn.dataset.filter;
    document.querySelectorAll("[data-filter]").forEach((el) => el.classList.toggle("active", el === btn));
    renderGames();
  });
});

document.querySelectorAll("[data-view]").forEach((btn) => {
  btn.addEventListener("click", () => {
    state.view = btn.dataset.view;
    document.querySelectorAll("[data-view]").forEach((el) => el.classList.toggle("active", el === btn));
    renderGames();
  });
});

searchInput.addEventListener("input", () => {
  state.search = searchInput.value;
  renderGames();
});

let dialTimer;
function queueRecs() {
  clearTimeout(dialTimer);
  dialTimer = setTimeout(() => loadRecommendations(), 280);
}

popularityDial.addEventListener("input", () => {
  state.popularity = Number(popularityDial.value) / 100;
  saveFilters();
  queueRecs();
});
weirdnessDial.addEventListener("input", () => {
  state.weirdness = Number(weirdnessDial.value) / 100;
  saveFilters();
  queueRecs();
});
saleDial.addEventListener("change", () => {
  state.onSale = saleDial.checked;
  saveFilters();
  loadRecommendations();
});
adultDial.addEventListener("change", () => {
  state.hideAdult = adultDial.checked;
  saveFilters();
  loadRecommendations();
});
goreDial.addEventListener("change", () => {
  state.hideGore = goreDial.checked;
  saveFilters();
  loadRecommendations();
});
shovelDial.addEventListener("change", () => {
  state.skipShovelware = shovelDial.checked;
  minReviewsInput.placeholder = state.skipShovelware ? "500" : "none";
  minPositiveInput.placeholder = state.skipShovelware ? "70" : "none";
  saveFilters();
  loadRecommendations();
});
function onNumberFilter() {
  state.minReviews = parseOptionalInt(minReviewsInput.value);
  state.maxReviews = parseOptionalInt(maxReviewsInput.value);
  state.minPositive = parseOptionalInt(minPositiveInput.value);
  if (state.minPositive != null) state.minPositive = Math.min(100, state.minPositive);
  saveFilters();
  queueRecs();
}
minReviewsInput.addEventListener("change", onNumberFilter);
maxReviewsInput.addEventListener("change", onNumberFilter);
minPositiveInput.addEventListener("change", onNumberFilter);
bindTagPicker(includeTagInput, includeSuggestEl, "include");
bindTagPicker(excludeTagInput, excludeSuggestEl, "exclude");
bindTagPicker(preferTagInput, preferSuggestEl, (name) => {
  addTagWeight(name, PREFER_WEIGHT);
});
bindTagPicker(avoidTagInput, avoidSuggestEl, (name) => {
  addTagWeight(name, AVOID_WEIGHT);
});
preferAddBtn.addEventListener("click", () => commitTagInput(preferTagInput, PREFER_WEIGHT));
avoidAddBtn.addEventListener("click", () => commitTagInput(avoidTagInput, AVOID_WEIGHT));
preferTagInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !preferSuggestEl.hidden) return;
  if (event.key === "Enter") {
    event.preventDefault();
    commitTagInput(preferTagInput, PREFER_WEIGHT);
  }
});
avoidTagInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !avoidSuggestEl.hidden) return;
  if (event.key === "Enter") {
    event.preventDefault();
    commitTagInput(avoidTagInput, AVOID_WEIGHT);
  }
});
function bindDisclosure(button, panel, key) {
  button.addEventListener("click", () => {
    const open = panel.hidden;
    setDisclosure(button, panel, open);
    state[key] = open;
    saveFilters();
    if (key === "libraryOpen" && open) renderGames();
  });
}
if (libraryToggle && libraryBody) bindDisclosure(libraryToggle, libraryBody, "libraryOpen");
function bindIntInput(input) {
  input.addEventListener("keydown", (event) => {
    if (["e", "E", "+", "-", ".", ","].includes(event.key)) event.preventDefault();
  });
  input.addEventListener("input", () => {
    input.value = input.value.replace(/[^\d]/g, "");
  });
}
if (minReviewsInput) bindIntInput(minReviewsInput);
if (maxReviewsInput) bindIntInput(maxReviewsInput);
if (minPositiveInput) bindIntInput(minPositiveInput);
function goHome() {
  document.body.classList.remove("session", "account");
  workspace.hidden = true;
  accountEl.hidden = true;
  mountSearchCriteria("workspace");
  searchCriteriaEl.hidden = true;
  syncAuthChrome();
  document.querySelector("[data-top-search-input]")?.focus();
  window.scrollTo({ top: 0, behavior: "smooth" });
}

changeProfileBtn.addEventListener("click", async () => {
  if (signedIn()) {
    await fetch("/auth/logout", { method: "POST", credentials: "same-origin" }).catch(() => {});
    config.user = null;
  }
  resetOptions();
  goHome();
});
profileBtn.addEventListener("click", () => {
  accountPage = "overview";
  openProfile();
});
document.querySelectorAll("[data-account-page]").forEach((btn) => {
  btn.addEventListener("click", () => {
    showAccountPage(btn.dataset.accountPage);
    if (accountPage === "watchlists") loadSavedSearches().catch((err) => setStatus(err.message, true));
  });
});
accountRecsBtn.addEventListener("click", () => {
  if (state.games.length || state.recs) showView("workspace");
  else runSift();
});
document.querySelector("#account-library").addEventListener("click", () => {
  document.body.classList.remove("session", "account");
  workspace.hidden = true;
  accountEl.hidden = true;
  mountSearchCriteria("workspace");
  searchCriteriaEl.hidden = true;
  syncAuthChrome();
  const top = document.querySelector("[data-top-search-input]");
  if (top) {
    top.focus();
    setStatus("Type a Steam username or profile URL in search, then press Enter.");
  }
});
brandHome.addEventListener("click", (event) => {
  event.preventDefault();
  if (state.games.length || state.recs) {
    showView("workspace");
    return;
  }
  if (signedIn()) {
    openProfile();
    return;
  }
  goHome();
});
const WATCHLIST_KEY = "steam-pending-watchlist";

function watchlistPayload() {
  const terms = defaultSearchTerms();
  return {
    name: (searchNameInput.value || "").trim() || terms || "Watchlist",
    terms,
    kind: "recs",
    filters: currentSearchFilters(),
  };
}

async function postWatchlist(payload) {
  const res = await fetch("/api/searches", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || "Could not save this sift.");
  return data;
}

async function saveToWatchlist() {
  const payload = watchlistPayload();
  if (!signedIn()) {
    sessionStorage.setItem(WATCHLIST_KEY, JSON.stringify(payload));
    location.href = "/auth/steam";
    return;
  }
  await postWatchlist(payload);
  if (searchNameInput) searchNameInput.value = "";
  setStatus("Saved this sift. Showing matching games.");
  if (!accountEl.hidden) await loadSavedSearches();
  runSift();
}

async function flushPendingWatchlist() {
  const raw = sessionStorage.getItem(WATCHLIST_KEY);
  if (!raw || !signedIn()) return false;
  sessionStorage.removeItem(WATCHLIST_KEY);
  try {
    await postWatchlist(JSON.parse(raw));
    setStatus("Saved this sift.");
    return true;
  } catch (err) {
    setStatus(err.message, true);
    return false;
  }
}

watchlistForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await saveToWatchlist();
  } catch (err) {
    setStatus(err.message, true);
  }
});

sortSelect.addEventListener("change", () => {
  state.sort = sortSelect.value;
  renderGames();
});

document.querySelector("#export-csv").addEventListener("click", () => {
  download("steam-library.csv", toCsv(), "text/csv");
});

document.querySelector("#copy-prompt").addEventListener("click", async () => {
  await navigator.clipboard.writeText(llmPrompt());
  setStatus("Copied a prompt for your favorite AI — paste it anywhere.");
});

document.querySelector("#export-prompt").addEventListener("click", () => {
  download("steam-suggestion-prompt.txt", llmPrompt());
  setStatus("Downloaded a prompt plus your library CSV.");
});

if (demoBtn) demoBtn.addEventListener("click", loadDemo);
applyFiltersToDom();
window.__sipLoad = loadLibrary;
window.__sipApply = applyLibrary;

async function start() {
  if (identifierInput && !identifierInput.value) {
    try {
      identifierInput.value = sessionStorage.getItem("steam-identifier") || "";
    } catch {
      /* guest profiles may block storage */
    }
  }

  try {
    const res = await fetch("/api/config", { credentials: "same-origin" });
    Object.assign(config, await res.json());
  } catch {
    Object.assign(config, { hasServerKey: false, user: null });
  }

  if (apiKeyField && apiKeyInput) {
    if (config.hasServerKey) {
      apiKeyField.hidden = true;
      apiKeyInput.required = false;
      apiKeyInput.value = "";
    } else {
      apiKeyField.hidden = false;
      apiKeyInput.required = true;
      apiKeyInput.value = "";
    }
  }

  syncAuthChrome();

  loadHomeBrowse();

  const loginParam = new URLSearchParams(location.search).get("login");
  if (loginParam) {
    history.replaceState({}, "", location.pathname || "/");
    if (loginParam === "error") {
      setStatus("Steam login failed. Try again, or search a public Steam username.", true);
    }
  }
  if (config.user?.steamid) {
    const savedPending = await flushPendingWatchlist();
    await loadSignedInLibrary();
    if (savedPending) await openProfile();
  } else if (pendingGuestIdentifier) {
    await loadLibrary(pendingGuestIdentifier);
  }
}

start().catch((err) => {
  setStatus(err.message || String(err), true);
});
