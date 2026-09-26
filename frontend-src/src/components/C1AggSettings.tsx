import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import Modal from "./Modal";
import { Ic } from "./Icons";

interface Ref { id: string; name: string }

// Per-connection 1C setting: which contragents are payment channels
// (aggregators like "KAPİTAL BANK"). Their money is B2C inflow, not a
// customer advance, and they are removed from customer reports.
export default function C1AggSettings({ connId, name, onClose }: {
  connId: string; name: string; onClose: () => void;
}) {
  const qc = useQueryClient();
  const [items, setItems] = useState<Ref[] | null>(null);
  const [search, setSearch] = useState("");
  const [saved, setSaved] = useState(false);

  const cur = useQuery({
    queryKey: ["c1agg", connId],
    queryFn: async () => (await api.get<{ aggregators: Ref[] }>("/reports/c1/settings", {
      params: { connection: connId },
    })).data.aggregators,
  });
  const list = items ?? cur.data ?? [];

  const hits = useQuery({
    queryKey: ["c1aggsearch", connId, search],
    queryFn: async () => (await api.get<{ customers: Ref[] }>("/reports/c1/customersearch", {
      params: { connection: connId, q: search },
    })).data.customers,
    enabled: search.trim().length >= 2,
  });

  const save = useMutation({
    mutationFn: () => api.put("/reports/c1/settings", { aggregators: list }, { params: { connection: connId } }),
    onSuccess: () => {
      setSaved(true);
      // report caches must recompute with the new classification
      qc.invalidateQueries({ predicate: (q) => String(q.queryKey[0]).startsWith("c1") });
      setTimeout(onClose, 700);
    },
  });

  const add = (r: Ref) => {
    if (!list.some((x) => x.id === r.id)) setItems([...list, r]);
    setSearch("");
  };
  const remove = (id: string) => setItems(list.filter((x) => x.id !== id));

  return (
    <Modal title={`${name} — ödəniş kanalı kontragentləri`} onClose={onClose}>
      <p className="page-sub" style={{ marginTop: 0 }}>
        Burada seçilən kontragentlər (məs. bank/aqreqator üzərindən abunəçi yığımları)
        hesabatlarda müştəri və ya avans kimi yox, <b>B2C daxilolma kanalı</b> kimi sayılır.
      </p>

      <div style={{ display: "flex", gap: ".4rem", flexWrap: "wrap", margin: ".6rem 0" }}>
        {list.map((r) => (
          <span key={r.id} style={{ display: "inline-flex", alignItems: "center", gap: ".35rem",
            background: "#fdf8e4", border: "1px solid #f0c000", padding: ".3rem .55rem", fontSize: ".84rem" }}>
            {r.name}
            <a style={{ cursor: "pointer", fontWeight: 700 }} onClick={() => remove(r.id)}>✕</a>
          </span>
        ))}
        {!list.length && <span style={{ color: "#8b90a3", fontSize: ".85rem" }}>Hələ seçilməyib</span>}
      </div>

      <label>Kontragent axtar (1C-dən)</label>
      <input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="məs. KAPİTAL" />
      {hits.data && search.trim().length >= 2 && (
        <div style={{ display: "grid", gap: ".25rem", marginTop: ".4rem", maxHeight: 220, overflow: "auto" }}>
          {hits.data.filter((h) => !list.some((x) => x.id === h.id)).map((h) => (
            <div key={h.id} onClick={() => add(h)}
              style={{ padding: ".45rem .6rem", border: "1px solid #eef0f5", cursor: "pointer" }}>
              + {h.name}
            </div>
          ))}
          {!hits.data.length && <span style={{ color: "#8b90a3", fontSize: ".85rem" }}>Tapılmadı</span>}
        </div>
      )}

      <div className="modal-actions">
        <button onClick={onClose}>Ləğv et</button>
        <button className="btn-primary" onClick={() => save.mutate()} disabled={save.isPending}>
          {Ic.check} {save.isPending ? "Yadda saxlanılır..." : saved ? "✓ Yadda saxlandı" : "Yadda saxla"}
        </button>
      </div>
    </Modal>
  );
}
