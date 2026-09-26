import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import { Ic, connMeta } from "../../components/Icons";
import ConnectModal, { cardTypeOf } from "../../components/ConnectModal";
import type { Connector } from "../../components/ConnectModal";

interface ConnLite { connector_type: string; source?: string }

export default function Integrations() {
  const me = useAuth((s) => s.user);
  const nav = useNavigate();
  const [cat, setCat] = useState("Hamısı");
  const [selected, setSelected] = useState<Connector | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["connectors"],
    queryFn: async () => (await api.get<{ connectors: Connector[] }>("/connectors")).data.connectors,
  });
  const { data: conns } = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: ConnLite[] }>("/connections")).data.connections,
  });

  const countOf = (type: string) => (conns ?? []).filter((c) => cardTypeOf(c) === type).length;

  const cards = data?.filter((c) => !c.hidden);
  const cats = ["Hamısı", ...new Set(cards?.map((c) => c.category) ?? [])];
  const shown = cards?.filter((c) => cat === "Hamısı" || c.category === cat);
  const canManage = me?.role === "owner" || me?.role === "admin";

  return (
    <>
      <h1>İnteqrasiyalar</h1>
      <p className="page-sub">Sistemlərinizi Gateway-ə qoşun — hamısı bir mərkəzdə</p>

      <div className="chips">
        {cats.map((c) => (
          <button key={c} className={`chip ${cat === c ? "on" : ""}`} onClick={() => setCat(c)}>{c}</button>
        ))}
      </div>

      {isLoading ? <p>Yüklənir...</p> : (
        <div className="grid-cards">
          {shown?.map((c) => {
            const meta = connMeta[c.type] ?? { icon: "🔌", tint: "#f2f3f8" };
            const n = countOf(c.type);
            return (
              <div key={c.type} className="connector-card"
                style={c.available ? { cursor: "pointer" } : undefined}
                onClick={() => c.available && nav(`/integrations/${c.type}`)}>
                <div className="cc-top">
                  <span className="cc-tile" style={{ background: meta.tint }}>{meta.icon}</span>
                  <div>
                    <strong>{c.name}</strong>
                    <span className="cc-cat">{c.category}</span>
                  </div>
                  {n > 0 && <span className="cc-pop">{n} bağlantı</span>}
                </div>
                <p>{c.description}</p>
                {c.available ? (
                  canManage ? (
                    n > 0 ? (
                      <button className="btn-primary w-full" onClick={(e) => { e.stopPropagation(); nav(`/integrations/${c.type}`); }}>
                        Bağlantılara bax
                      </button>
                    ) : (
                      <button className="btn-primary w-full" onClick={(e) => { e.stopPropagation(); setSelected(c); }}>
                        {Ic.plus} Qoş
                      </button>
                    )
                  ) : <span className="cc-soon">Yalnız admin qoşa bilər</span>
                ) : (
                  <span className="cc-soon">{Ic.clock} Tezliklə</span>
                )}
              </div>
            );
          })}
        </div>
      )}

      {selected && data && (
        <ConnectModal connector={selected} all={data}
          onClose={() => setSelected(null)}
          onCreated={() => nav(`/integrations/${selected.type}`)} />
      )}
    </>
  );
}
