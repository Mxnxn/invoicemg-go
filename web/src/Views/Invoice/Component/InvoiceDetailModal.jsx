import React, { useCallback, useEffect, useState } from "react";
import { Modal } from "reactstrap";
import { X } from "react-feather";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import DataTable from "../../../Common/DataTable/DataTable";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import ReceiptList from "../../../Common/receipts/ReceiptList";
import { describeGoods, rowNetAmount } from "../invoiceFormat";
import { invoiceBackend } from "../invoice_backend";
import "../invoice.css";
import Blank from "../../../Common/DataTable/Blank";

// One invoice, in detail.
//
// This used to open the customer's ENTIRE invoice list, which answered a question nobody had
// asked: clicking a single invoice should show that invoice, not its forty siblings. To see
// one customer's invoices, the list itself now has a Client filter that narrows it in place;
// /invoice/:cid is still a real URL for deep links.
//
// The payments panel sits on the LEFT and is opened by a labelled tab rather than a bare
// hamburger icon. The icon gave no clue what it did, and the panel it opened listed every
// payment the customer had ever made rather than the ones that settled this invoice.
const InvoiceDetailModal = ({ invoice, onClose }) => {
    const [receipts, setReceipts] = useState({ rows: [], total: 0, loaded: false });
    const [showReceipts, setShowReceipts] = useState(true);

    const load = useCallback(async () => {
        if (!invoice?._id) return;
        try {
            const formData = new FormData();
            formData.set("invoice_id", invoice._id);
            const res = await invoiceBackend.getInvoiceReceives(formData);
            setReceipts({ rows: res.data || [], total: res.total || 0, loaded: true });
        } catch (error) {
            // The interceptor toasts the reason; the panel shows its empty state.
            setReceipts((prev) => ({ ...prev, loaded: true }));
        }
    }, [invoice?._id]);

    useEffect(() => {
        let live = true;
        load().then(() => {
            if (!live) setReceipts((prev) => prev);
        });
        return () => {
            live = false;
        };
    }, [load]);

    const entries = invoice?.entries || [];
    const total = Number(invoice?.total) || 0;
    // The invoice's own recorded figure, not a sum of the receipts panel: a receipt list that
    // failed to load must not make a paid invoice look unpaid.
    const received = Number(invoice?.receivedAmount) || 0;
    const due = total - received;

    return (
        <Modal isOpen toggle={onClose} size="xl" style={{ maxWidth: "90vw", width: "90vw" }}>
            <div
                className="shell-card-header"
                style={{ borderBottom: "1px solid var(--border-default)", alignItems: "center" }}
            >
                <div>
                    <span className="text-heading-brand" style={{ color: "var(--text-link)" }}>
                        {invoice?.invoiceNumber || "Invoice"}
                    </span>
                    <span className="text-body-small" style={{ color: "var(--text-tertiary)", marginLeft: 10 }}>
                        {invoice?.clientFirm || invoice?.clientName} · {getDate(invoice?.date)}
                    </span>
                </div>
                <button type="button" className="slide-overlay-close" onClick={onClose} aria-label="Close">
                    <X size={18} />
                </button>
            </div>

            <div style={{ padding: 20 }}>
                <div className="invoice-detail-tabs" role="tablist">
                    <button
                        type="button"
                        role="tab"
                        aria-selected={showReceipts}
                        className={`invoice-detail-tab${showReceipts ? " is-active" : ""}`}
                        onClick={() => setShowReceipts((prev) => !prev)}
                    >
                        Received
                        <span className="invoice-detail-tab-count">{receipts.rows.length}</span>
                    </button>
                    <span className="text-body-small invoice-detail-summary">
                        Total ₹{RoundOff(total)} · Received ₹{RoundOff(received)} · Due ₹{RoundOff(due)}
                        {due > 0 ? (
                            <StatusBadge status="pending">pending</StatusBadge>
                        ) : (
                            <StatusBadge status="paid">paid</StatusBadge>
                        )}
                    </span>
                </div>

                {/* Payments first in the DOM, so they are the left column and the reading order
                    matches - this panel was previously on the right. */}
                <div className="invoice-detail-body">
                    {showReceipts && (
                        <aside className="invoice-detail-receipts" aria-label="Payments against this invoice">
                            <h3 className="text-heading-brand invoice-detail-receipts-title">Received</h3>
                            {!receipts.loaded ? (
                                <p className="text-body-small" style={{ color: "var(--text-tertiary)", margin: 0 }}>
                                    Loading…
                                </p>
                            ) : (
                                <ReceiptList
                                    rows={receipts.rows}
                                    emptyMessage="Nothing received against this invoice yet."
                                />
                            )}
                        </aside>
                    )}

                    <div className="invoice-detail-main">
                        <DataTable>
                            <thead>
                                <tr>
                                    <th scope="col">Description</th>
                                    <th scope="col">HSN/SAC</th>
                                    <th scope="col">Qty</th>
                                    <th scope="col">Rate</th>
                                    <th scope="col">Amount</th>
                                </tr>
                            </thead>
                            <tbody>
                                {entries.length === 0 ? (
                                    <tr>
                                        <td colSpan={5} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                            This invoice has no line items.
                                        </td>
                                    </tr>
                                ) : (
                                    entries.map((entry, index) => (
                                        <tr key={entry._id || index}>
                                            <td>{describeGoods(entry)}</td>
                                            <td className="cell-mono">{entry.hsn || <Blank />}</td>
                                            <td className="cell-mono">{entry.qty}</td>
                                            <td className="cell-mono">₹{RoundOff(entry.rate)}</td>
                                            <td className="cell-mono">₹{RoundOff(rowNetAmount(entry))}</td>
                                        </tr>
                                    ))
                                )}
                            </tbody>
                        </DataTable>
                    </div>
                </div>
            </div>
        </Modal>
    );
};

export default InvoiceDetailModal;
