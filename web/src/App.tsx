import { useEffect, useMemo, useState } from "react";

type StateDTO = {
  name: string;
  url: string;
  up: boolean;
  last_checked: string;
  latency_ms: number;
  status_code: number;
  last_error: string;
  consecutive_success: number;
  consecutive_fail: number;
  total_checks: number;
  total_fails: number;
};

type UptimeItem = {
  target: string;
  window: string;
  from: string;
  total_checks: number;
  total_up: number;
  uptime_pct: number;
};

type UptimeResponse = {
  from: string;
  generated_at: string;
  items: UptimeItem[];
  window: string;
};


function classNames(...xs: Array<string | false | undefined>) {
  return xs.filter(Boolean).join(" ");
}

function formatMs(ms: number) {
  if (!Number.isFinite(ms)) return "—";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
}

function formatPct(pct?: number) {
  if (pct === undefined || Number.isNaN(pct)) return "—";
  if (pct === 100) return "100%";
  return `${pct.toFixed(2)}%`;
}

function pctTone(pct: number) {
  if (!Number.isFinite(pct)) return "bg-slate-100 text-slate-700 ring-slate-200";
  if (pct >= 99.5) return "bg-emerald-50 text-emerald-800 ring-emerald-200";
  if (pct >= 95) return "bg-amber-50 text-amber-800 ring-amber-200";
  return "bg-rose-50 text-rose-800 ring-rose-200";
}

function StatusBadge({ up }: { up: boolean }) {
  return (
    <span
      className={classNames(
        "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold ring-1",
        up
          ? "bg-emerald-50 text-emerald-800 ring-emerald-200"
          : "bg-rose-50 text-rose-800 ring-rose-200"
      )}
    >
      <span
        className={classNames(
          "h-2 w-2 rounded-full",
          up ? "bg-emerald-600" : "bg-rose-600"
        )}
      />
      {up ? "UP" : "DOWN"}
    </span>
  );
}

function StatPill({
  label,
  value,
}: {
  label: string;
  value: string | number;
}) {
  return (
    <div className="rounded-2xl bg-white/70 px-4 py-3 ring-1 ring-slate-200 backdrop-blur">
      <div className="text-xs font-medium text-slate-500">{label}</div>
      <div className="mt-1 text-lg font-semibold text-slate-900">{value}</div>
    </div>
  );
}

export default function App() {
  const [data, setData] = useState<StateDTO[]>([]);
  const [lastUpdated, setLastUpdated] = useState<string>("");
  const [q, setQ] = useState("");
  const [onlyDown, setOnlyDown] = useState(false);
  const [err, setErr] = useState<string>("");
  const [uptime, setUptime] = useState<UptimeResponse | null>(null);
  const [uptimeErr, setUptimeErr] = useState<string>("");
  const telegramChannel = "https://t.me/+u0viVzoo9hM5ZTQ0"

  async function loadStatus() {
    try {
      setErr("");
      const res = await fetch("/status", { cache: "no-store" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const json = await res.json()

      // Accept both API shapes (All/all) so UI keeps working if backend changes casing.
      const items = Array.isArray(json.All)
        ? json
        : Array.isArray(json)
        ? json
        : [];
      setData(items);
      setLastUpdated(new Date().toISOString());
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to load /status";
      setErr(msg);
    }
  }

  async function loadUptime() {
    try {
      setUptimeErr("");
      const res = await fetch("/uptime/all", { cache: "no-store" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const json = (await res.json()) as UptimeResponse;
      setUptime(json);
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to load /uptime/all";
      setUptimeErr(msg);
    }
  }

  useEffect(() => {
    loadStatus();
    loadUptime();
    const t = setInterval(loadStatus, 10000);
    const u = setInterval(loadUptime, 60000);
    return () => {
      clearInterval(t);
      clearInterval(u);
    };
  }, []);

  const { upCount, downCount } = useMemo(() => {
    let up = 0,
      down = 0;
    for (const it of data) {
      if (it.up) {
        up++;
      } else {
        down++;
      }
    }
    return { upCount: up, downCount: down };
  }, [data]);

  const filtered = useMemo(() => {
    const qq = q.trim().toLowerCase();
    return data
      .slice()
      .sort((a, b) => {
        if (a.up !== b.up) return a.up ? 1 : -1; // DOWN first
        return (a.name || "").localeCompare(b.name || "");
      })
      .filter((it) => (onlyDown ? !it.up : true))
      .filter((it) =>
        qq
          ? (it.name || "").toLowerCase().includes(qq) ||
            (it.url || "").toLowerCase().includes(qq) ||
            (it.last_error || "").toLowerCase().includes(qq)
          : true
      );
  }, [data, q, onlyDown]);

  return (
    <div className="min-h-screen bg-[radial-gradient(1200px_circle_at_20%_0%,rgba(14,165,233,0.12),transparent_60%),radial-gradient(900px_circle_at_80%_20%,rgba(245,158,11,0.10),transparent_55%),linear-gradient(to_bottom,rgba(2,6,23,0.02),rgba(2,6,23,0.00))]">
      <div className="mx-auto max-w-6xl px-4 py-10">
        {/* Header */}
        <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <div className="flex items-center gap-3">
              {/* Cypriot “island dot” mark */}
              <img
                src="/cyobserver.png"
                alt="CyPing"
                className="h-10 w-10 rounded-2xl bg-slate-900 object-cover shadow-sm"
              />
              <div>
                <h1 className="text-2xl font-bold tracking-tight text-slate-900">
                  CyObserver
                </h1>
                <p className="text-sm text-slate-600">
                  Live status monitor for Cyprus public platforms (unofficial).
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <button
              onClick={loadStatus}
              className="inline-flex items-center justify-center rounded-xl bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-slate-800 active:bg-slate-950"
            >
              Refresh
            </button>
            {telegramChannel ? (
              <a
                href={telegramChannel}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center justify-center rounded-xl bg-sky-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-sky-500 active:bg-sky-700"
              >
                Join Telegram (live alerts)
              </a>
            ) : null}
            <div className="text-xs text-slate-600">
              <div className="font-medium text-slate-700">Auto-refresh</div>
              <div>every 10s</div>
            </div>
          </div>
        </div>

        {/* Stats */}
        <div className="mt-8 grid grid-cols-1 gap-4 sm:grid-cols-3">
          <StatPill label="Platforms UP" value={upCount} />
          <StatPill label="Platforms DOWN" value={downCount} />
          <StatPill
            label="Last updated (UTC)"
            value={lastUpdated ? lastUpdated.replace("T", " ").replace("Z", "") : "—"}
          />
        </div>

        {/* Uptime digest */}
        <div className="mt-8 rounded-3xl bg-white/80 p-6 ring-1 ring-slate-200 backdrop-blur">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="text-sm font-semibold text-slate-800">Uptime last 24h</div>
              <div className="text-xs text-slate-500">
                {uptime?.from ? `From ${uptime.from} to ${uptime.generated_at}` : "Window: 24h"}
              </div>
            </div>
            <button
              onClick={loadUptime}
              className="inline-flex items-center justify-center rounded-xl bg-slate-900 px-3 py-2 text-xs font-semibold text-white shadow-sm hover:bg-slate-800 active:bg-slate-950"
            >
              Refresh uptime
            </button>
          </div>

          {uptimeErr ? (
            <div className="mt-4 rounded-2xl bg-rose-50 p-3 text-xs text-rose-800 ring-1 ring-rose-200">
              {uptimeErr}
            </div>
          ) : null}

          <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
            {(uptime?.items ?? []).map((item) => (
              <div
                key={item.target}
                className={
                  "rounded-2xl px-4 py-3 ring-1 backdrop-blur " + pctTone(item.uptime_pct)
                }
              >
                <div className="text-sm font-semibold text-slate-900">{item.target}</div>
                <div className="mt-1 flex items-baseline gap-2 text-2xl font-bold text-slate-900">
                  {formatPct(item.uptime_pct)}
                  <span className="text-xs font-semibold text-slate-500">/ {item.window}</span>
                </div>
                <div className="mt-1 text-xs text-slate-600">
                  Checks: {item.total_checks} • Up: {item.total_up}
                </div>
              </div>
            ))}
            {(uptime?.items?.length ?? 0) === 0 && !uptimeErr ? (
              <div className="text-sm text-slate-500">No uptime data yet.</div>
            ) : null}
          </div>
        </div>

        {/* Controls */}
        <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="relative w-full sm:max-w-md">
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search platform, URL, error…"
              className="w-full rounded-2xl bg-white/70 px-4 py-3 text-sm text-slate-900 ring-1 ring-slate-200 placeholder:text-slate-400 backdrop-blur focus:outline-none focus:ring-2 focus:ring-sky-300"
            />
          </div>

          <label className="inline-flex items-center gap-2 text-sm text-slate-700">
            <input
              type="checkbox"
              checked={onlyDown}
              onChange={(e) => setOnlyDown(e.target.checked)}
              className="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-sky-300"
            />
            Show only DOWN
          </label>
        </div>

        {/* Error banner */}
        {err ? (
          <div className="mt-6 rounded-2xl bg-rose-50 p-4 text-sm text-rose-800 ring-1 ring-rose-200">
            <div className="font-semibold">Could not load status</div>
            <div className="mt-1 opacity-90">{err}</div>
          </div>
        ) : null}

        {/* Table */}
        <div className="mt-6 overflow-hidden rounded-3xl bg-white/70 ring-1 ring-slate-200 backdrop-blur">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-slate-50/70">
                <tr className="text-left text-xs font-semibold uppercase tracking-wide text-slate-600">
                  <th className="px-5 py-4">Status</th>
                  <th className="px-5 py-4">Platform</th>
                  <th className="px-5 py-4">Last checked (UTC)</th>
                  <th className="px-5 py-4 text-right">Latency</th>
                  <th className="px-5 py-4 text-right">HTTP</th>
                  <th className="px-5 py-4">Error</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {filtered.length === 0 ? (
                  <tr>
                    <td className="px-5 py-8 text-sm text-slate-500" colSpan={6}>
                      No results.
                    </td>
                  </tr>
                ) : (
                  filtered.map((it) => (
                    <tr key={it.name} className="hover:bg-slate-50/40">
                      <td className="px-5 py-4">
                        <StatusBadge up={it.up} />
                      </td>
                      <td className="px-5 py-4">
                        <div className="font-semibold text-slate-900">{it.name}</div>
                        {it.url ? (
                          <a
                            className="mt-1 block text-xs text-sky-700 hover:underline"
                            href={it.url}
                            target="_blank"
                            rel="noreferrer"
                          >
                            {it.url}
                          </a>
                        ) : (
                          <div className="mt-1 text-xs text-slate-400">—</div>
                        )}
                      </td>
                      <td className="px-5 py-4 text-sm text-slate-700">
                        {it.last_checked || "—"}
                      </td>
                      <td className="px-5 py-4 text-right text-sm font-semibold text-slate-900">
                        {formatMs(it.latency_ms ?? 0)}
                      </td>
                      <td className="px-5 py-4 text-right text-sm text-slate-700">
                        {it.status_code ? it.status_code : "—"}
                      </td>
                      <td className="px-5 py-4 text-sm text-slate-600">
                        <div className="line-clamp-2 max-w-xl">
                          {it.last_error || "—"}
                        </div>
                        {/* subtle health hint */}
                        <div className="mt-1 text-xs text-slate-400">
                          checks: {it.total_checks} • fails: {it.total_fails} • streak:{" "}
                          {it.up ? it.consecutive_success : it.consecutive_fail}
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>

        {/* Footer */}
        <div className="mt-8 text-xs text-slate-500">
          <div>
            <span className="font-semibold text-slate-600">Disclaimer:</span>{" "}
            CyObserver is community-run and unofficial. It monitors only publicly accessible
            endpoints and has no access to internal systems or user data.
          </div>
        </div>
      </div>
    </div>
  );
}
