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

type StatusResponse = {
  All: StateDTO[];
  ByName: Record<string, StateDTO>;
};

function classNames(...xs: Array<string | false | undefined>) {
  return xs.filter(Boolean).join(" ");
}

function formatMs(ms: number) {
  if (!Number.isFinite(ms)) return "—";
  if (ms < 1000) return `${ms} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
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

  async function load() {
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
    } catch (e: any) {
      setErr(e?.message ?? "Failed to load /status");
    }
  }

  useEffect(() => {
    load();
    const t = setInterval(load, 10000);
    return () => clearInterval(t);
  }, []);

  const { upCount, downCount } = useMemo(() => {
    let up = 0,
      down = 0;
    for (const it of data) it.up ? up++ : down++;
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
              <div className="relative h-10 w-10 rounded-2xl bg-slate-900 text-white shadow-sm">
                <div className="absolute left-3 top-3 h-2.5 w-2.5 rounded-full bg-sky-400" />
                <div className="absolute left-5 top-5 h-2.5 w-2.5 rounded-full bg-amber-300" />
              </div>
              <div>
                <h1 className="text-2xl font-bold tracking-tight text-slate-900">
                  CyPing
                </h1>
                <p className="text-sm text-slate-600">
                  Live status monitor for Cyprus public platforms (unofficial).
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <button
              onClick={load}
              className="inline-flex items-center justify-center rounded-xl bg-slate-900 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-slate-800 active:bg-slate-950"
            >
              Refresh
            </button>
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
            CyPing is community-run and unofficial. It monitors only publicly accessible
            endpoints and has no access to internal systems or user data.
          </div>
        </div>
      </div>
    </div>
  );
}
