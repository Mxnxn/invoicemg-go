import React from "react";
import { Modal, ModalHeader, ModalBody } from "reactstrap";
import NumberBadge from "../../../Common/DataTable/NumberBadge";

import DataTable from "../../../Common/DataTable/DataTable";
import Blank from "../../../Common/DataTable/Blank";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { getDateForEntry } from "../../../Common/DateAndTime/getDate";
import "../purchaseInvoice.css";
import { purchaseInvoiceGrandTotal, rowTotal } from "../purchaseInvoiceMath";
import { cardSize } from "../../Lifecycle/jobBoard";

// What is on one purchase invoice, without opening the form that edits it.
//
// The only way in used to be the edit modal, which means the way to LOOK at a bill was to open
// the thing that changes it - so reading one put you one stray keystroke from altering a
// financial record. This reads and nothing else.
//
// Totals come from purchaseInvoiceMath, the same module the form and the API's
// Helpers/PurchaseRowTotal use, so a figure here cannot disagree with the one that was saved.
export default function PurchaseInvoiceDetailModal({ invoice, isOpen, toggle }) {
    if (!invoice) return null;

    const rows = invoice.rows || [];
    const total = invoice.total ?? purchaseInvoiceGrandTotal(rows);

    return (
        <Modal isOpen={isOpen} toggle={toggle} size="lg" centered scrollable>
            <ModalHeader toggle={toggle}>
                <NumberBadge accent>{invoice.invoiceNumber}</NumberBadge>
            </ModalHeader>
            <ModalBody>
                <div className="pi-detail-meta">
                    <div>
                        <span className="text-label-caps">Supplier</span>
                        <p className="text-body-medium">
                            {invoice.supplier_id?.firm || invoice.supplier_id?.name || <Blank label="No supplier" />}
                        </p>
                    </div>
                    <div>
                        <span className="text-label-caps">Invoice date</span>
                        <p className="text-body-medium cell-mono">{getDateForEntry(invoice.date)}</p>
                    </div>
                    <div>
                        <span className="text-label-caps">Lines</span>
                        <p className="text-body-medium cell-mono">{rows.length}</p>
                    </div>
                </div>

                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">Material</th>
                            <th scope="col">Size / Qty</th>
                            <th scope="col">HSN</th>
                            <th scope="col">GST</th>
                            <th scope="col">Qty</th>
                            <th scope="col">Purchase rate</th>
                            <th scope="col">Amount</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.length === 0 ? (
                            <tr>
                                <td colSpan={7} className="text-body-small" style={{ padding: 20, color: "var(--text-tertiary)" }}>
                                    This invoice has no lines.
                                </td>
                            </tr>
                        ) : (
                            rows.map((row, i) => (
                                <tr key={row._id || i}>
                                    <td>{row.material || <Blank />}</td>
                                    {/* What was bought, not just what it was called - two
                                        lines reading "Vinyl" are the same purchase only if
                                        they are the same size. */}
                                    <td className="cell-mono">{cardSize(row) || <Blank label="No size" />}</td>
                                    <td className="cell-mono">{row.hsn || <Blank label="No HSN" />}</td>
                                    <td className="cell-mono">{row.gst ? `${row.gst}%` : <Blank label="No GST" />}</td>
                                    <td className="cell-mono">{row.qty}</td>
                                    <td className="cell-mono">₹{RoundOff(row.rate)}</td>
                                    <td className="cell-mono">₹{RoundOff(rowTotal(row))}</td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </DataTable>

                <p className="pi-detail-total text-body-medium">Grand total ₹{RoundOff(total)}</p>
            </ModalBody>
        </Modal>
    );
}
