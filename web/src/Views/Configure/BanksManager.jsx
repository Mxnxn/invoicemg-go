import React, { useEffect, useState } from "react";
import { Input } from "reactstrap";
import { Edit, Trash2, Plus, Check, X } from "react-feather";
import DataTable from "../../Common/DataTable/DataTable";
import Pagination from "../../Shell/Pagination";
import usePagedRows from "../../Common/useRowsPerPage";
import RowActionMenu, { RowActionMenuItem } from "../../Common/DataTable/RowActionMenu";
import ConfirmDialog from "../../Common/ConfirmDialog";
import OpeningBalanceField from "../../Common/OpeningBalanceField";
import { RoundOff } from "../../Common/DateAndTime/RoundOff";
import { bankBackend } from "../../Common/bank_backend";
import { useBanks, addBank, refreshBanks } from "../../Common/bankStore";
import { notifySuccess } from "../../global/toast";

const EMPTY = { name: "", openingBalance: 0 };

// A bank account is now entered on a form rather than typed into a lone box in the card
// header. It stopped being a single field the moment it gained an opening balance - and the
// header input had no room to ask which way that balance went, which is the one thing about
// it that must not be guessed.
//
// Renaming loads the account back into the same form instead of editing in place: the balance
// needs the amount and the direction together, and an inline row cannot hold both without
// becoming a form anyway.
//
// Removal is refused by the server for a bank that money has moved through: receipts, supplier
// payments and expenses all point at it, and deleting it would leave those rows dangling and
// the Bank Report's arithmetic quietly wrong.
const BanksManager = () => {
    const banks = useBanks();

    // Honours Account settings > Appearance > Rows per page, like every other list.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(banks);
    const [form, setForm] = useState(EMPTY);
    const [editingId, setEditingId] = useState(null);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [busy, setBusy] = useState(false);

    useEffect(() => {
        refreshBanks();
    }, []);

    const reset = () => {
        setForm(EMPTY);
        setEditingId(null);
    };

    const submit = async () => {
        const name = form.name.trim();
        if (!name || busy) return;
        setBusy(true);
        try {
            const formData = new FormData();
            formData.set("name", name);
            formData.set("openingBalance", String(Number(form.openingBalance) || 0));
            if (editingId) {
                formData.set("bank_id", editingId);
                await bankBackend.update(formData);
                await refreshBanks();
                notifySuccess("Bank updated.");
            } else {
                const res = await bankBackend.create(formData);
                addBank(res.data);
                notifySuccess("Bank added.");
            }
            reset();
        } catch (err) {
            // The global interceptor raises the toast.
        } finally {
            setBusy(false);
        }
    };

    const confirmDelete = async () => {
        const bank = deleteTarget;
        setDeleteTarget(null);
        if (!bank) return;
        // The row being removed may be the one loaded in the form.
        if (editingId === bank._id) reset();
        try {
            const formData = new FormData();
            formData.set("bank_id", bank._id);
            await bankBackend.remove(formData);
            await refreshBanks();
            notifySuccess("Bank removed.");
        } catch (err) {
            // The server explains when a bank still has transactions against it; the global
            // interceptor shows that message.
        }
    };

    // Shown the way it was entered rather than as a signed number: "-1,500" reads as a typo,
    // "1,500 overdrawn" reads as a fact.
    const balanceCell = (value) => {
        const amount = Number(value) || 0;
        if (!amount) return <span style={{ color: "var(--text-tertiary)" }}>—</span>;
        return (
            <span style={{ color: amount < 0 ? "var(--status-red-text)" : "var(--text-primary)" }}>
                ₹{RoundOff(Math.abs(amount))}
                {amount < 0 ? " overdrawn" : ""}
            </span>
        );
    };

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">Banks</span>
            </div>

            <div className="bank-form">
                <div className="bank-form-field">
                    <label className="form-control-label pp fs-12" htmlFor="bank-name">
                        Name
                    </label>
                    <Input
                        id="bank-name"
                        placeholder="e.g. HDFC Current"
                        value={form.name}
                        onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
                        onKeyDown={(e) => e.key === "Enter" && submit()}
                    />
                </div>

                <OpeningBalanceField
                    side="bank"
                    value={form.openingBalance}
                    onChange={(openingBalance) => setForm((prev) => ({ ...prev, openingBalance }))}
                />

                <div className="bank-form-actions">
                    <button
                        type="button"
                        className="shell-btn shell-btn-primary"
                        onClick={submit}
                        disabled={busy || !form.name.trim()}
                    >
                        {editingId ? <Check size={14} /> : <Plus size={14} />} {editingId ? "Save" : "Add bank"}
                    </button>
                    {editingId && (
                        <button type="button" className="shell-btn shell-btn-secondary" onClick={reset}>
                            <X size={14} /> Cancel
                        </button>
                    )}
                </div>
            </div>

            <DataTable>
                <thead>
                    <tr>
                        <th scope="col">Name</th>
                        <th scope="col">Opening balance</th>
                        <th scope="col">Action</th>
                    </tr>
                </thead>
                <tbody>
                    {banks.length === 0 ? (
                        <tr>
                            <td colSpan={3} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                No banks yet. Add one above, or inline from any Bank Transfers form.
                            </td>
                        </tr>
                    ) : (
                        pageRows.map((bank, index) => (
                            <tr key={bank._id} className={editingId === bank._id ? "row-editing" : ""}>
                                <td className="text-body-medium">{bank.name}</td>
                                <td className="cell-mono">{balanceCell(bank.openingBalance)}</td>
                                <td>
                                    <RowActionMenu
                                        open={moreMenu === index}
                                        onOpenChange={(next) => setMoreMenu(next ? index : -1)}
                                    >
                                        <RowActionMenuItem
                                            icon={Edit}
                                            variant="warning"
                                            onClick={() => {
                                                setEditingId(bank._id);
                                                setForm({
                                                    name: bank.name,
                                                    openingBalance: Number(bank.openingBalance) || 0,
                                                });
                                                setMoreMenu(-1);
                                            }}
                                        >
                                            Edit
                                        </RowActionMenuItem>
                                        <RowActionMenuItem
                                            icon={Trash2}
                                            variant="danger"
                                            onClick={() => {
                                                setDeleteTarget(bank);
                                                setMoreMenu(-1);
                                            }}
                                        >
                                            Remove
                                        </RowActionMenuItem>
                                    </RowActionMenu>
                                </td>
                            </tr>
                        ))
                    )}
                </tbody>
            </DataTable>
            <div className="table-foot">
                <span className="text-body-small table-foot-count">
                    {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} banks` : `${total} banks`}
                </span>
                <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
            </div>

            <ConfirmDialog
                open={!!deleteTarget}
                message={`Remove ${deleteTarget?.name || "this bank"}?`}
                confirmLabel="Remove"
                position="bottom"
                danger
                onConfirm={confirmDelete}
                onCancel={() => setDeleteTarget(null)}
            />
        </div>
    );
};

export default BanksManager;
