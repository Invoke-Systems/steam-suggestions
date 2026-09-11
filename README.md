# Should I Play

Load a public Steam library, inspect hours and genres, and get recommendations from Steam community tags. The app is a single Go executable: the UI is embedded, and visitors cannot swap out source on a live host.

## Run

```bash
cp .env.example .env
# put your Steam Web API key in .env
go run ./cmd/server
# or: npm start
```

Open [http://localhost:3847](http://localhost:3847).

Build a binary with `go build -o steam-suggestions ./cmd/server` (or `npm run build`). The UI is embedded. Put `data/steam.sqlite` next to the binary (or run from this repo) so tags and prices stay available.

## Production (shouldiplay.co)

Infra (Linode nanode + DNS) and **host deploy** live in
[tf-invoke-systems-linode](https://github.com/Invoke-Systems/tf-invoke-systems-linode)
under `shouldiplay.co/`. This repo only **builds and pushes** the app image
(`.github/workflows/publish-image.yml` → `ghcr.io/invoke-systems/steam-suggestions`).
Secrets split is documented in [SECRETS.md](SECRETS.md).

## Steam API key

Steam keys belong to the **app host**, not each visitor.

| Mode | What happens |
| --- | --- |
| Public / shared | Set `STEAM_API_KEY` on the server. Visitors can **Sign in through Steam** (OpenID) or paste a public profile. Client-supplied keys are ignored. |
| Local without `.env` | The UI asks for a key and keeps it in `sessionStorage` for this browser session. It is POSTed to this server and never written to disk. |

A Steam Web API key can read **any public profile**. You do not need each user's key. Their Game details setting must be Public. Wishlists are used when the profile's wishlist is public.

**Sign in through Steam** uses Valve’s OpenID 2.0 flow. This server verifies the assertion with Steam, then stores a session cookie (`HttpOnly`, `SameSite=Lax`, `Secure` on HTTPS). You never see a Steam password or the visitor’s API key. Set `PUBLIC_URL=https://shouldiplay.co` in production so the OpenID return URL matches. Signed-in users and sessions live in `data/steam.sqlite`, keyed by SteamID64. A private library does not block login; they can still save searches on their profile. Guest paste-a-profile still works without an account.

Libraries are cached in memory for 10 minutes. Genre metadata is cached in `data/genre-cache.json`. Steam app names, community tags, prices, and review counts live in `data/steam.sqlite` (gitignored). Cover art is fetched from Steam on first display and cached in `data/images/` (also gitignored). Rate limits: 20 library loads and 40 recommendation calls per IP per hour.

The worker fills `IStoreService/GetAppList` then tags, prices, and review summaries for those apps. The web server scores that tagged catalog. It does not run the full crawl on start. Until review rows exist, skip-shovelware recs can look sparse; run the worker with `-reviews` (or with no flags, which now does tags, prices, and reviews).

Get a key at [steamcommunity.com/dev/apikey](https://steamcommunity.com/dev/apikey).

## Recommendation engine

The engine is deterministic and runs on this server. It does not call an LLM.

1. Pull Steam community tags for played games.
2. Build an hours-weighted tag vector, with a boost for the last two weeks. Generic tags like Singleplayer are downweighted.
3. Ignore unplayed backlog. Lightly penalize tags you bounced off.
4. Pre-filter the tagged SQLite catalog by those distinctive tags (thousands of neighbors, not a 64-game list), **drop shovelware by default**, skip owned games, demos, soundtracks, DLC, and dedicated servers, then rank and group into **more of what you like**, **adjacent experiments**, and a **wildcard**.

The ranker uses three signals, in this order:

1. **Hard filters** — min/max review counts, min % positive, include/exclude tags. These cut the pool before scoring. They do not get inverted by sliders.
2. **% positive** — a small fixed weight (~22%). Better-reviewed games beat Mixed review-bombed junk even when tag fit is similar.
3. **Indie ↔ Mainstream** and **Familiar ↔ Weird** — rank among games that already passed the floor. Indie prefers lower review volume; it is not a shovelware gate.

Discovery dials and filters:

- **Skip shovelware** — on by default. Requires a review row, at least **500** reviews, and **70%** positive unless you set those fields yourself.
- **Indie ↔ Mainstream** — log(review count) among games that passed the floor. Midpoint is neutral on this axis.
- **Familiar ↔ Weird** — temperature against your taste mix (tag likelihoods from hours). Familiar stays in that mix; weird walks away from it.
- **Min / max reviews** and **min % positive** — whole-number bounds. Empty fields use the shovelware defaults when that toggle is on.
- **Must include / Exclude tags** — typeahead over Steam community tags. Taste-mix tags are clickable (shift-click excludes).
- **On sale** — keep recs that are discounted in SQLite price snapshots.
- **Hide adult / Hide gore** — optional filters on Sexual Content, Nudity, Hentai, and Gore. Off by default; the engine does not silently drop them.
- **Not for me** — hides a card in this browser and re-ranks.
- Wishlist games get a ranking boost when the wishlist is public.
- Price history sparklines when the worker has more than one daily snapshot.

Filters persist in this browser. The loaded library stays on this page and is not published. Cards show `% positive · review count` when we have them.

Prices are one region per database (`STEAM_CC` / `STEAM_CURRENCY`, default US / USD). Switch region by setting those and recrawling prices. Peak players and price history grow from **our** Steam samples over time — not from SteamDB.

## Steam worker (independent scrapers)

On boot the worker logs catalog status (`games` / `tagged` / `priced` / …) and whether catch-up is needed. With `-every`, each scraper still runs an **immediate first pass**, then sleeps — Docker/systemd long-running services are not “wait then crawl.”

Each Steam endpoint is its own goroutine. Appid is the primary key. No flags (or compose `worker -every 6h`) enables all seven Steam scrapers:

| Goroutine | Flag | Steam / data |
|-----------|------|----------------|
| `taglist` | `-taglist` | `IStoreService/GetTagList` |
| `applist` | `-applist` | `IStoreService/GetAppList` |
| `items` | `-items` (or `-tags`/`-prices`/`-reviews`) | `IStoreBrowseService/GetItems` → tags, prices, reviews |
| `players` | `-players` | charts + `GetNumberOfCurrentPlayers` (default 8 HTTP workers) |
| `details` | `-details` | `store/api/appdetails` |
| `news` | `-news` | `ISteamNews/GetNewsForApp` |
| `achievements` | `-achievements` | `GetGlobalAchievementPercentagesForApp` |

Separate (not in the default Steam set):

| Job | Flag | Notes |
|-----|------|--------|
| ITAD lows | `-itad-lows` | Official ITAD API → `price_low`; needs `ITAD_API_KEY` |

The **web** process also runs a light `Warmup()` goroutine: tag dict, EnsureTags/Reviews for the embedded catalog, and AppList ingest if the DB is thin (`<1000` games) or AppList is stale (`>24h`).

```bash
go run ./cmd/worker
go run ./cmd/worker -players -details -limit 200
go run ./cmd/worker -players -workers 8 -limit 1000   # charts top-100 + parallel samples
go run ./cmd/worker -every 6h
# legacy aliases still work: -tags -prices -reviews (drive GetItems writes)
```

### IsThereAnyDeal historical lows (one-time)

With an [ITAD API app](https://isthereanydeal.com) key in `.env` as `ITAD_API_KEY`, backfill Steam store historical lows for your region (`STEAM_CC`):

```bash
go run ./cmd/worker -itad-lows
go run ./cmd/worker -itad-lows -limit 500   # smoke test
```

This is resumable (progress in `itad_map` / `price_low`). It uses the official ITAD API only (lookup by Steam app id → Steam shop #61 store low). Respects ITAD’s rate window (~1000 req / 5 min).

Useful flags: `-delay 2s`, `-batch 40`, `-max-429 5`, `-limit 200` (per scraper), `-db data/steam.sqlite`, `-every 6h`. Progress is the database, so you can stop and resume. Scrapers rate-limit independently; SQLite writes are serialized.

## Deploy

Docker:

```bash
docker compose up --build
```

systemd units live in `deploy/`. Copy the server and worker binaries to `/opt/steam-suggestions`, then:

```bash
sudo cp deploy/steam-suggestions.service /etc/systemd/system/
sudo cp deploy/steam-suggestions-worker.service /etc/systemd/system/
sudo cp deploy/steam-suggestions-worker.timer /etc/systemd/system/
sudo systemctl enable --now steam-suggestions steam-suggestions-worker.timer
```

Set `STEAM_API_KEY` and `ALLOW_CLIENT_API_KEY=false`. Do not put a Steam key in frontend code or a public repo.

CSV export and “Ask your AI” are still there if you want a second opinion.
