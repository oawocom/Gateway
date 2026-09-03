import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import Modal from "../../components/Modal";
import { Ic } from "../../components/Icons";
import type { Tenant } from "./TenantList";

export default function DeleteTenantModal({ tenant, onClose, onDeleted }: {
  tenant: Tenant; onClose: () => void; onDeleted?: () => void;
}) {
  const qc = useQueryClient();
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");

  const del = useMutation({
    mutationFn: () => api.delete(`/admin/tenants/${tenant.id}`, { data: { confirm } }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-tenants"] });
      onClose();
      onDeleted?.();
    },
    onError: (e: any) => setError(e.response?.data?.error ?? "Silinə bilmədi"),
  });

  return (
    <Modal title="Tenantı sil" onClose={onClose}>
      <div className="danger-box">
        <strong>⚠ Bu əməliyyat geri qaytarıla bilməz!</strong>
        <p><b>{tenant.name}</b> tenantı, bütün istifadəçiləri və <code>{tenant.db_name}</code> bazası
          (bağlantılar, sinxronlaşmış data, avtomatlaşdırmalar) həmişəlik silinəcək.</p>
      </div>
      <label>Təsdiq üçün <code className="slug-code">{tenant.slug}</code> yazın</label>
      <input value={confirm} onChange={(e) => setConfirm(e.target.value)}
        placeholder={tenant.slug} autoFocus autoComplete="off" />
      {error && <p className="error">{error}</p>}
      <div className="modal-actions">
        <button onClick={onClose}>Ləğv et</button>
        <button className="btn-delete" disabled={confirm !== tenant.slug || del.isPending}
          onClick={() => del.mutate()}>
          {Ic.trash} {del.isPending ? "Silinir..." : "Həmişəlik sil"}
        </button>
      </div>
    </Modal>
  );
}
