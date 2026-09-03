import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";

interface EntityRow {
  connection_id: string; connection_name: string;
  entity_name: string; record_count: number; last_synced_at: string | null;
}
interface Rec { external_id: string; data: Record<string, any>; synced_at: string }

export default function Data() {
  const [sel, setSel] = useState<EntityRow | null>(null);
  const [page, setPage] = useState(1);

  const summary = useQuery({
    queryKey: ["data-summary"],
    queryFn: async () => (await api.get<{ entities: EntityRow[] }>("/data/summary")).data.entities,
  });

  const records = useQuery({
    queryKey: ["records", sel?.connection_id, sel?.entity_name, page],
    queryFn: async () =>
      (await api.get<{ records: Rec[]; total: number; per_page: number }>("/data/records", {
        params: { connection_id: sel!.connection_id, entity: sel!.entity_name, page },
      })).data,
    enabled: !!sel,
  });

  const cols = (() => {
    const first = records.data?.records?.[0]?.data;
    if (!first) return [];
    return Object.keys(first).filter((k) => typeof first[k] !== "object").slice(0, 6);
  })();

  const totalPages = records.data ? Math.max(1, Math.ceil(records.data.total / records.data.per_page)) : 1;

  return (
    <>
      <h1>Data</h1>
      <p className="page-sub">Sinxronlaşmış datalara baxış</p>

      {!sel ? (
        summary.isLoading ? <p>Yüklənir...</p> : !summary.data?.length ? (
          <div className="empty-state">
            <h3>Hələ data yoxdur</h3>
            <p>Bağlantılarım bölməsindən entity sinxronlaşdırın.</p>
          </div>
        ) : (
          <table className="tbl">
            <thead><tr><th>Bağlantı</th><th>Entity</th><th>Qeyd sayı</th><th>Son sinxronizasiya</th><th></th></tr></thead>
            <tbody>
              {summary.data.map((e) => (
                <tr key={e.connection_id + e.entity_name}>
                  <td>{e.connection_name}</td>
                  <td>{e.entity_name}</td>
                  <td>{e.record_count}</td>
                  <td>{e.last_synced_at ? new Date(e.last_synced_at).toLocaleString("az") : "—"}</td>
                  <td><button className="btn-primary" onClick={() => { setSel(e); setPage(1); }}>Bax</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        )
      ) : (
        <>
          <div className="data-head">
            <button onClick={() => setSel(null)}>← Geri</button>
            <strong>{sel.connection_name} / {sel.entity_name}</strong>
            <span>{records.data?.total ?? 0} qeyd</span>
          </div>
          {records.isLoading ? <p>Yüklənir...</p> : (
            <>
              <div className="tbl-wrap">
                <table className="tbl">
                  <thead>
                    <tr>{cols.map((c) => <th key={c}>{c}</th>)}<th>JSON</th></tr>
                  </thead>
                  <tbody>
                    {records.data?.records.map((r) => (
                      <tr key={r.external_id}>
                        {cols.map((c) => <td key={c}>{String(r.data[c] ?? "")}</td>)}
                        <td>
                          <details>
                            <summary>bax</summary>
                            <pre className="json-view">{JSON.stringify(r.data, null, 2)}</pre>
                          </details>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="pager">
                <button disabled={page <= 1} onClick={() => setPage(page - 1)}>← Əvvəlki</button>
                <span>{page} / {totalPages}</span>
                <button disabled={page >= totalPages} onClick={() => setPage(page + 1)}>Növbəti →</button>
              </div>
            </>
          )}
        </>
      )}
    </>
  );
}
