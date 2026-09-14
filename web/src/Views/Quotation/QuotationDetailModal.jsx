import React, { useRef, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, Button, FormGroup, Input, Row, Col } from "reactstrap";
import { isIgstJob, totalTaxPercent } from "../Lifecycle/jobTax";
import { Briefcase, Trash2, Check } from "react-feather";
import DataTable from "../../Common/DataTable/DataTable";
import { getDateForEntry } from "../../Common/DateAndTime/getDate";
import { quotationBackend } from "./quotation_backend";
import { rowAmount, rowTotal, quotationGrandTotal } from "./quotationMath";
import { hasDimensions } from "../../Common/rowPricing";
import { RoundOff } from "../../Common/DateAndTime/RoundOff";
import { notifyError, notifySuccess } from "../../global/toast";
import ConfirmDialog from "../../Common/ConfirmDialog";

// Read-only summary of a quotation - everything here is disabled by design (spec point 9).
// Row actions (Add to Job / Trash) call back to the server individually and patch the
// quotation in place rather than reopening the create flow.
const QuotationDetailModal = ({ quotation, onClose, onChange }) => {
    const [busyRowId, setBusyRowId] = useState(null);
    const [deleteTarget, setDeleteTarget] = useState(null);
    // Synchronous guard against double-clicks - React state updates (busyRowId) don't
    // land until the next render, so two fast clicks can both pass the disabled check.
    // A ref updates immediately, closing that gap.
    const inFlightRowIds = useRef(new Set());

    if (!quotation) return null;
    const rows = quotation.rows || [];

    const onTrashRow = async (rowId) => {
        try {
            setBusyRowId(rowId);
            const formData = new FormData();
            formData.set("quotation_id", quotation._id);
            formData.set("row_id", rowId);
            const res = await quotationBackend.deleteRow(formData);
            onChange(res.data);
        } catch (error) {
            notifyError(error.message || "Couldn't remove the row.");
        } finally {
            setBusyRowId(null);
            setDeleteTarget(null);
        }
    };

    const onAddToJob = async (rowId) => {
        if (inFlightRowIds.current.has(rowId)) return;
        inFlightRowIds.current.add(rowId);
        setBusyRowId(rowId);
        try {
            const formData = new FormData();
            formData.set("quotation_id", quotation._id);
            formData.set("row_id", rowId);
            const res = await quotationBackend.addRowToJob(formData);
            onChange(res.data.quotation);
            notifySuccess(`${res.message} (Job ${res.data.job.challanNumber})`);
        } catch (error) {
            notifyError(error.message || "Couldn't create a job from this row.");
        } finally {
            inFlightRowIds.current.delete(rowId);
            setBusyRowId(null);
        }
    };

    // A quotation is interstate or it is not - the place of supply belongs to the sale, not to
    // one line of it. Derived from the rows rather than stored, the same way the job does it.
    const interstate = isIgstJob(rows);

    return (
        <Modal isOpen={!!quotation} toggle={onClose} size="xl" style={{ maxWidth: "95vw" }}>
            <ModalHeader toggle={onClose}>
                {quotation.quotationNumber} — {quotation.client_id?.clientFirm || quotation.client_id?.clientName}
            </ModalHeader>
            <ModalBody>
                <div className="text-body-small" style={{ color: "var(--text-tertiary)", marginBottom: 12 }}>
                    Date: {getDateForEntry(quotation.date)}
                </div>
                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">Material</th>
                            <th scope="col">Description</th>
                            <th scope="col">Length</th>
                            <th scope="col">Width</th>
                            <th scope="col">Qty</th>
                            <th scope="col">Rate</th>
                            {/* One column, not two. A quotation is read by a customer, and
                                CGST 9 beside SGST 9 asks them to add the halves back together
                                to find the 18 they were quoted. The split is a filing detail
                                that belongs on the invoice, not on the offer - and it cannot
                                describe an interstate sale at all, where the whole rate sits
                                on IGST and both these columns read 0. */}
                            <th scope="col">{interstate ? "IGST%" : "GST%"}</th>
                            <th scope="col">Discount</th>
                            <th scope="col">Charges</th>
                            <th scope="col">Amount</th>
                            <th scope="col">Total</th>
                            <th scope="col">Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row, index) => (
                                <tr
                                    key={row._id}
                                    className={row.job_id ? "row-success" : ""}
                                    style={row.job_id ? { background: "var(--xan-emerald-bg)" } : undefined}
                                >
                                    <td>
                                        <Input value={row.material} disabled />
                                    </td>
                                    <td>
                                        <Input value={row.description} disabled />
                                    </td>
                                    {/* A by-quantity row has no size to show. */}
                                    <td className="cell-mono">
                                        {hasDimensions(row) ? row.length : <span className="xan-cell-na">&mdash;</span>}
                                    </td>
                                    <td className="cell-mono">
                                        {hasDimensions(row) ? row.width : <span className="xan-cell-na">&mdash;</span>}
                                    </td>
                                    <td className="cell-mono">{row.qty}</td>
                                    <td className="cell-mono">₹{row.rate}</td>
                                    <td className="cell-mono">{totalTaxPercent(row)}%</td>
                                    <td className="cell-mono">₹{RoundOff(row.discount || 0)}</td>
                                    <td className="cell-mono">₹{RoundOff(row.charges || 0)}</td>
                                    <td className="cell-mono">₹{RoundOff(rowAmount(row))}</td>
                                    <td className="cell-mono">₹{RoundOff(rowTotal(row))}</td>
                                    <td>
                                        <div className="d-flex align-items-center" style={{ gap: 6, flexWrap: "wrap" }}>
                                            {row.job_id ? (
                                                <Button
                                                    className="shell-btn shell-btn-sm d-flex align-items-center"
                                                    disabled
                                                    aria-label="Job added"
                                                    style={{
                                                        color: "var(--xan-emerald)",
                                                        background: "var(--xan-emerald-bg)",
                                                        border: "1px solid var(--xan-emerald)",
                                                        opacity: 1,
                                                        cursor: "not-allowed",
                                                    }}
                                                >
                                                    <Check size={13} />
                                                </Button>
                                            ) : (
                                                <Button
                                                    className="shell-btn shell-btn-sm shell-btn-primary d-flex align-items-center"
                                                    style={{ gap: 4 }}
                                                    disabled={busyRowId === row._id}
                                                    onClick={() => onAddToJob(row._id)}
                                                >
                                                    <Briefcase size={13} />
                                                    Add to Job
                                                </Button>
                                            )}
                                            <Button
                                                className="shell-btn shell-btn-sm shell-btn-danger d-flex align-items-center"
                                                disabled={busyRowId === row._id}
                                                onClick={() => setDeleteTarget(row._id)}
                                                aria-label="Delete"
                                            >
                                                <Trash2 size={13} />
                                            </Button>
                                        </div>
                                    </td>
                                </tr>
                        ))}
                        {rows.length === 0 && (
                            <tr>
                                <td colSpan={13} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                    No rows left on this quotation.
                                </td>
                            </tr>
                        )}
                    </tbody>
                </DataTable>
                <div className="text-body-medium" style={{ textAlign: "right", marginTop: 12 }}>
                    Grand Total: <strong>₹{RoundOff(quotationGrandTotal(rows))}</strong>
                </div>
            </ModalBody>
            <ModalFooter>
                <Button className="shell-btn shell-btn-secondary" onClick={onClose}>
                    Close
                </Button>
            </ModalFooter>
            <ConfirmDialog
                open={!!deleteTarget}
                message="Delete this row?"
                confirmLabel="Yes"
                cancelLabel="No"
                position="bottom"
                danger
                onConfirm={() => onTrashRow(deleteTarget)}
                onCancel={() => setDeleteTarget(null)}
            />
        </Modal>
    );
};

export default QuotationDetailModal;
