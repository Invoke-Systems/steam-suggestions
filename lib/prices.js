const STEAM_UA = "Mozilla/5.0 (compatible; SteamSuggestions/1.0; +https://localhost)";
const ITEMS_URL = "https://api.steampowered.com/IStoreBrowseService/GetItems/v1/";
const FRESH_MS = 3 * 60 * 60 * 1000;

function countryCode() {
  return (process.env.STEAM_CC || "US").toUpperCase();
}

function currencyCode() {
  return (process.env.STEAM_CURRENCY || "USD").toUpperCase();
}

export function snapshotFromPurchase(item, now = Date.now()) {
  const opt = item?.best_purchase_option;
  if (item?.is_free && !opt) {
    return {
      name: item.name || "",
      t: now,
      currency: currencyCode(),
      initial: 0,
      final: 0,
      discount: 0,
      formatted: "Free",
    };
  }
  if (!opt) {
    return {
      name: item?.name || "",
      t: now,
      currency: "",
      initial: 0,
      final: 0,
      discount: 0,
      formatted: "Free or unlisted",
    };
  }
  const final = Number(opt.final_price_in_cents) || 0;
  const initial = Number(opt.original_price_in_cents) || final;
  const discount = Number(opt.discount_pct) || 0;
  return {
    name: item.name || "",
    t: now,
    currency: currencyCode(),
    initial,
    final,
    discount,
    formatted: opt.formatted_final_price || "",
  };
}

export function createPriceStore(db) {
  async function fetchItems(appids) {
    const input = {
      ids: appids.map((appid) => ({ appid: Number(appid) })),
      context: { language: "english", country_code: countryCode() },
      data_request: { include_tag_count: 20, include_all_purchase_options: true },
    };
    const url = `${ITEMS_URL}?input_json=${encodeURIComponent(JSON.stringify(input))}`;
    const res = await fetch(url, { headers: { "User-Agent": STEAM_UA } });
    if (res.status === 429) {
      const err = new Error("rate");
      err.retry = true;
      throw err;
    }
    if (!res.ok) return [];
    const json = await res.json();
    return json.response?.store_items || [];
  }

  async function fillMissing(appids) {
    const prices = {};
    const chunk = 40;
    for (let i = 0; i < appids.length; i += chunk) {
      const batch = appids.slice(i, i + chunk);
      for (let attempt = 0; attempt < 3; attempt += 1) {
        try {
          const items = await fetchItems(batch);
          const now = Date.now();
          const seen = new Set();
          for (const item of items) {
            const appid = Number(item.appid);
            seen.add(appid);
            const snapshot = snapshotFromPurchase(item, now);
            db.setPrice(appid, snapshot);
          }
          for (const appid of batch) {
            if (!seen.has(Number(appid))) {
              db.setPrice(appid, snapshotFromPurchase({ name: "" }, now));
            }
          }
          Object.assign(prices, db.getPrices(batch));
          break;
        } catch (err) {
          if (err.retry && attempt < 2) {
            await new Promise((resolve) => setTimeout(resolve, 800 * (attempt + 1)));
            continue;
          }
        }
      }
      if (i + chunk < appids.length) await new Promise((resolve) => setTimeout(resolve, 150));
    }
    return prices;
  }

  async function getPrices(appids) {
    const unique = [...new Set(appids.map(Number).filter(Boolean))];
    const cached = db.getPrices(unique);
    const now = Date.now();
    const prices = {};
    const missing = [];
    for (const appid of unique) {
      const latest = cached[appid];
      if (latest && now - latest.t < FRESH_MS) prices[appid] = latest;
      else missing.push(appid);
    }
    if (missing.length) {
      Object.assign(prices, await fillMissing(missing));
    }
    return prices;
  }

  async function featuredSales() {
    const url = new URL("https://store.steampowered.com/api/featuredcategories");
    url.searchParams.set("cc", countryCode());
    url.searchParams.set("l", "english");
    const res = await fetch(url, { headers: { "User-Agent": STEAM_UA } });
    if (!res.ok) return [];
    const json = await res.json();
    return (json?.specials?.items || []).map((item) => ({
      appid: Number(item.id),
      name: item.name,
      discount: item.discount_percent || 0,
      formatted: item.final_price != null ? `$${(item.final_price / 100).toFixed(2)}` : "",
      final: item.final_price ?? null,
      header: item.large_capsule_image || item.small_capsule_image || "",
    }));
  }

  return { load: async () => {}, getPrices, featuredSales };
}
