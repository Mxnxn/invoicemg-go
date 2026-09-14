import React, { useEffect, useMemo, useState } from "react";
import { Button, Input } from "reactstrap";
import { Trash2, X, Eye, Search } from "react-feather";
import SlideOverlay from "../../../Common/SlideOverlay/SlideOverlay";
import DataTable from "../../../Common/DataTable/DataTable";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import InvoicePreviewModal from "../../Invoice/Component/InvoicePreviewModal";
import BatchReceiveModal from "../../BatchReceive/component/BatchReceiveModal";
import { batchReceiveBackend } from "../../BatchReceive/batch_receive_backend";
import { getDate, getDateForEntry } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { invoiceBackend } from "../../Invoice/invoice_backend";
import { quotationBackend } from "../../Quotation/quotation_backend";
import { quotationGrandTotal } from "../../Quotation/quotationMath";
import CreateQuotationModal from "../../Quotation/CreateQuotationModal";
import QuotationDetailModal from "../../Quotation/QuotationDetailModal";
import { notifyError } from "../../../global/toast";
import { useUndoDelete } from "../../../Common/undoDelete";
import ReceiptDestinations from "../../../Common/receipts/ReceiptDestinations";

const TABS = ["Invoices", "Bank Transfers", "Quotations"];

// Single tabbed panel replacing the separate sidebars (batch-receive history, invoices)
// the hamburger menu used to open one at a time. Materials are a shared global catalog
// now (managed from the Products page), so they no longer live here.
const ClientMenuPanel = ({ onClose, cname, uid, clientId, user }) => {
    const [tab, setTab] = useState("Invoices");
    const [moreMenu, setMoreMenu] = useState(-1);
    const [invoices, setInvoices] = useState([]);
    const [invoicesLoading, setInvoicesLoading] = useState(false);
    const [previewInvoice, setPreviewInvoice] = useState(null);
    const { scheduleDelete } = useUndoDelete();
    const [invoiceSearch, setInvoiceSearch] = useState("");
    const [batchSearch, setBatchSearch] = useState("");
    const [batches, setBatches] = useState([]);
    const [batchesLoading, setBatchesLoading] = useState(false);
    const [batchModal, setBatchModal] = useState(false);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const [quotations, setQuotations] = useState([]);
    const [quotationsLoading, setQuotationsLoading] = useState(false);
    const [createQuotationModal, setCreateQuotationModal] = useState(false);
    const [activeQuotation, setActiveQuotation] = useState(null);

    useEffect(() => {
        if (tab !== "Bank Transfers" || batches.length || !clientId) return;
        setBatchesLoading(true);
        batchReceiveBackend
            .listReceives({ client_id: clientId })
            .then((res) => setBatches(res.data))
            .finally(() => setBatchesLoading(false));
    }, [tab, clientId, batches.length]);

    useEffect(() => {
        if (tab !== "Invoices" || invoices.length || !uid) return;
        setInvoicesLoading(true);
        const formData = new FormData();
        formData.set("uid", uid);
        invoiceBackend
            .getInvoices(formData)
            .then((res) => setInvoices(res.data.filter((inv) => inv.client_id === clientId)))
            .finally(() => setInvoicesLoading(false));
    }, [tab, uid, clientId, invoices.length]);

    useEffect(() => {
        if (tab !== "Quotations" || quotations.length || !clientId) return;
        setQuotationsLoading(true);
        quotationBackend
            .listQuotations({ client_id: clientId })
            .then((res) => setQuotations(res.data))
            .finally(() => setQuotationsLoading(false));
    }, [tab, clientId, quotations.length]);

    const onCreateQuotation = (formData) => {
        return quotationBackend.createQuotation(formData).then((res) => {
            setQuotations((prev) => [res.data, ...prev]);
        });
    };

    const onQuotationChanged = (updated) => {
        setQuotations((prev) => prev.map((q) => (q._id === updated._id ? updated : q)));
        setActiveQuotation(updated);
    };

    const openPreview = (invoice) => setPreviewInvoice({ ...invoice, account: user.account, view: true });

    const filteredInvoices = useMemo(() => {
        const q = invoiceSearch.trim().toLowerCase();
        if (!q) return invoices;
        return invoices.filter(
            (inv) => String(inv.invoiceNumber).toLowerCase().includes(q) || getDate(inv.date).toLowerCase().includes(q)
        );
    }, [invoices, invoiceSearch]);

    const filteredBatches = useMemo(() => {
        const q = batchSearch.trim().toLowerCase();
        if (!q) return batches;
        return batches.filter(
            (el) =>
                (el.note || "").toLowerCase().includes(q) ||
                (el.bank_id?.name || "").toLowerCase().includes(q) ||
                getDate(el.date).toLowerCase().includes(q)
        );
    }, [batches, batchSearch]);

    const confirmDeleteBatch = () => {
        if (!deleteTarget) return;
        const batch = deleteTarget;
        const index = batches.findIndex((el) => el._id === batch._id);
        setBatches((prev) => prev.filter((el) => el._id !== batch._id));
        setDeleteTarget(null);
        scheduleDelete({
            label: "bank transfer",
            commit: () => {
                const formData = new FormData();
                formData.set("batch_id", batch._id);
                return batchReceiveBackend.deleteReceive(formData);
            },
            undo: () => setBatches((prev) => [...prev.slice(0, index), batch, ...prev.slice(index)]),
        });
    };

    return (
        <SlideOverlay onClose={onClose} width={760}>
            <span className="d-flex mb-3">
                <button type="button" className="slide-overlay-close ml-auto" onClick={onClose} aria-label="Close">
                    <X size={18} />
                </button>
            </span>
            <div className="d-flex align-items-center justify-content-between mb-3" style={{ flexWrap: "wrap", gap: 12 }}>
                <div>
                    {/* Shell typography, not the legacy Argon font utilities. `.geb` and
                        `fs-24` set GEB at a fixed 24px, so this header rendered in a
                        different family from the tab strip beside it - two type systems in
                        one row - and ignored Account settings > Appearance > Font size,
                        which every text-* class scales with. */}
                    <p className="text-label-caps client-panel-eyebrow">{`${cname}'s`}</p>
                    <h2 className="text-heading-page client-panel-title">{tab}</h2>
                </div>
                <div className="shell-segmented" role="tablist">
                    {TABS.map((t) => (
                        <button
                            key={t}
                            type="button"
                            role="tab"
                            aria-selected={tab === t}
                            className={["shell-segmented-btn", tab === t ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setTab(t)}
                        >
                            {t}
                        </button>
                    ))}
                </div>
            </div>

            {tab === "Bank Transfers" && (
                <>
                    <div className="d-flex justify-content-between mb-2" style={{ gap: 12 }}>
                        <div style={{ position: "relative", flex: 1, maxWidth: 320 }}>
                            <Search size={14} style={{ position: "absolute", left: 10, top: 10, color: "var(--text-tertiary)" }} />
                            <Input
                                type="text"
                                placeholder="Search by note or date..."
                                value={batchSearch}
                                onChange={(e) => setBatchSearch(e.target.value)}
                                style={{ paddingLeft: 32 }}
                            />
                        </div>
                        <Button className="shell-btn shell-btn-primary" onClick={() => setBatchModal(true)}>
                            Add new
                        </Button>
                    </div>
                    <DataTable>
                        <thead>
                            <tr>
                                <th scope="col">Date</th>
                                <th scope="col">Bank</th>
                                <th scope="col">Amount</th>
                                <th scope="col">Applied to</th>
                                <th scope="col">Mode</th>
                                <th scope="col">Note</th>
                                <th scope="col">Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            {batchesLoading ? (
                                <tr>
                                    <td colSpan={7} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                        Loading…
                                    </td>
                                </tr>
                            ) : filteredBatches.length === 0 ? (
                                <tr>
                                    <td colSpan={7} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                        No bank transfers found.
                                    </td>
                                </tr>
                            ) : (
                                filteredBatches.map((el, index) => (
                                    <tr key={el._id || index}>
                                        <td className="cell-mono">{getDate(el.date)}</td>
                                        <td>{el.bank_id?.name || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                        <td className="cell-mono">₹{RoundOff(el.amount)}</td>
                                        {/* Which jobs and invoices this transfer settled. The row used to be
                                            a date and an amount, which could not answer the one question
                                            anyone asks of a past payment. */}
                                        <td>
                                            <ReceiptDestinations destinations={el.destinations} />
                                        </td>
                                        <td>{el.mode || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                        <td>{el.note || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                        <td>
                                            <RowActionMenu open={moreMenu === `b${index}`} onOpenChange={(next) => setMoreMenu(next ? `b${index}` : -1)}>
                                                <RowActionMenuItem
                                                    icon={Trash2}
                                                    variant="danger"
                                                    onClick={() => {
                                                        setDeleteTarget(el);
                                                        setMoreMenu(-1);
                                                    }}
                                                >
                                                    Delete
                                                </RowActionMenuItem>
                                            </RowActionMenu>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </DataTable>
                </>
            )}

            {tab === "Invoices" && (
                <>
                    <div className="mb-2" style={{ position: "relative", maxWidth: 320 }}>
                        <Search size={14} style={{ position: "absolute", left: 10, top: 10, color: "var(--text-tertiary)" }} />
                        <Input
                            type="text"
                            placeholder="Search by invoice # or date..."
                            value={invoiceSearch}
                            onChange={(e) => setInvoiceSearch(e.target.value)}
                            style={{ paddingLeft: 32 }}
                        />
                    </div>
                    <DataTable>
                        <thead>
                            <tr>
                                <th scope="col">Invoice #</th>
                                <th scope="col">Date</th>
                                <th scope="col">Total</th>
                                <th scope="col">Action</th>
                            </tr>
                        </thead>
                        <tbody>
                            {invoicesLoading ? (
                                <tr>
                                    <td colSpan={4} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                        Loading…
                                    </td>
                                </tr>
                            ) : filteredInvoices.length === 0 ? (
                                <tr>
                                    <td colSpan={4} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                        No invoices found.
                                    </td>
                                </tr>
                            ) : (
                                filteredInvoices.map((inv) => (
                                    <tr key={inv._id}>
                                        <td className="cell-mono">{inv.invoiceNumber}</td>
                                        <td className="cell-mono">{getDate(inv.date)}</td>
                                        <td className="cell-mono">₹{RoundOff(inv.total)}</td>
                                        <td>
                                            <button
                                                type="button"
                                                className="shell-icon-btn"
                                                aria-label="Preview invoice"
                                                onClick={() => openPreview(inv)}
                                            >
                                                <Eye size={16} />
                                            </button>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </DataTable>
                </>
            )}

            {tab === "Quotations" && (
                <>
                    <div className="d-flex justify-content-end mb-2">
                        <Button className="shell-btn shell-btn-primary" onClick={() => setCreateQuotationModal(true)}>
                            Create Quotation
                        </Button>
                    </div>
                    <DataTable>
                        <thead>
                            <tr>
                                <th scope="col">Quotation #</th>
                                <th scope="col">Date</th>
                                <th scope="col">Rows</th>
                                <th scope="col">Grand Total</th>
                            </tr>
                        </thead>
                        <tbody>
                            {quotationsLoading ? (
                                <tr>
                                    <td colSpan={4} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                        Loading…
                                    </td>
                                </tr>
                            ) : quotations.length === 0 ? (
                                <tr>
                                    <td colSpan={4} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                        No quotations for this client yet.
                                    </td>
                                </tr>
                            ) : (
                                quotations.map((q) => (
                                    <tr key={q._id} style={{ cursor: "pointer" }} onClick={() => setActiveQuotation(q)}>
                                        <td className="cell-mono">
                                            <button type="button" className="lifecycle-link">
                                                {q.quotationNumber}
                                            </button>
                                        </td>
                                        <td className="cell-mono">{getDateForEntry(q.date)}</td>
                                        <td className="cell-mono">{q.rows?.length || 0}</td>
                                        <td className="cell-mono">₹{RoundOff(quotationGrandTotal(q.rows))}</td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </DataTable>
                </>
            )}

            <InvoicePreviewModal isOpen={!!previewInvoice} toggle={() => setPreviewInvoice(null)} invoice={previewInvoice} />
            <CreateQuotationModal
                isOpen={createQuotationModal}
                toggle={() => setCreateQuotationModal(false)}
                onCreate={onCreateQuotation}
                fixedClient={{ id: clientId, name: cname }}
            />
            <QuotationDetailModal quotation={activeQuotation} onClose={() => setActiveQuotation(null)} onChange={onQuotationChanged} />
            <BatchReceiveModal
                isOpen={batchModal}
                toggle={() => setBatchModal(false)}
                fixedClient={{ id: clientId, name: cname }}
                onCreated={(receive) => setBatches((prev) => [receive, ...prev])}
            />
            <ConfirmDialog
                open={!!deleteTarget}
                message={deleteTarget ? `Delete this bank transfer of ₹${deleteTarget.amount}? This can't be undone.` : ""}
                confirmLabel="Delete"
                onConfirm={confirmDeleteBatch}
                onCancel={() => setDeleteTarget(null)}
            />
        </SlideOverlay>
    );
};

export default ClientMenuPanel;
