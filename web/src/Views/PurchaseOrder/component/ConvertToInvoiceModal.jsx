import React, { useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter } from "reactstrap";
import { FileText } from "react-feather";

import { purchaseOrderBackend } from "../purchaseOrder_backend";
import { notifySuccess } from "../../../global/toast";

// Turning an approved order into the supplier's bill - the end of the order's life.
//
// It asks for two things the order cannot know: the number on the SUPPLIER'S invoice, and the
// date they billed it. Neither is derivable from the order, and guessing either would put a
// wrong reference on a financial record, so both are typed.
//
// The route refuses anything not approved, or approved and since edited. That check is the
// server's and stays there - this only offers the action where it can succeed.
export default function ConvertToInvoiceModal({ po, isOpen, toggle, onConverted }) {
    const [invoiceNumber, setInvoiceNumber] = useState("");
    const [date, setDate] = useState(() => new Date().toISOString().slice(0, 10));
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");

    const submit = async (e) => {
        e.preventDefault();
        setError("");
        if (!invoiceNumber.trim()) return setError("Enter the number on the supplier's invoice.");
        if (!date) return setError("Enter the date the supplier billed it.");

        setBusy(true);
        try {
            const formData = new FormData();
            formData.set("po_id", po._id);
            formData.set("invoiceNumber", invoiceNumber.trim());
            formData.set("date", date);
            const res = await purchaseOrderBackend.convert(formData, window.localStorage.getItem("session_token"));
            notifySuccess(res.message || "Purchase invoice created.");
            onConverted?.(res.data);
            toggle();
        } catch (err) {
            setError(err?.message || "Couldn't create the purchase invoice.");
        } finally {
            setBusy(false);
        }
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} centered>
            <ModalHeader toggle={toggle}>Generate purchase invoice</ModalHeader>
            <form onSubmit={submit}>
                <ModalBody>
                    <p className="text-body-small" style={{ color: "var(--text-tertiary)", marginTop: 0 }}>
                        Creates the purchase invoice for <strong>{po?.poNumber}</strong> from the lines on this
                        order. {/* Said before the button, not discovered after it. */}
                        The supplier link stops working once this is done, and the order can no longer be edited.
                    </p>

                    <div style={{ marginBottom: 14 }}>
                        <label className="form-control-label pp fs-12" htmlFor="convert-invoice-number">
                            Supplier&rsquo;s invoice number
                        </label>
                        <input
                            id="convert-invoice-number"
                            className="form-control"
                            value={invoiceNumber}
                            onChange={(e) => setInvoiceNumber(e.target.value)}
                            placeholder="the number on their bill"
                            autoComplete="off"
                        />
                    </div>

                    <div>
                        <label className="form-control-label pp fs-12" htmlFor="convert-invoice-date">
                            Invoice date
                        </label>
                        <input
                            id="convert-invoice-date"
                            type="date"
                            className="form-control"
                            value={date}
                            onChange={(e) => setDate(e.target.value)}
                        />
                    </div>

                    {error && (
                        <span className="text-body-small" style={{ color: "var(--status-red-text)" }}>
                            {error}
                        </span>
                    )}
                </ModalBody>
                <ModalFooter>
                    <button type="button" className="shell-btn shell-btn-secondary" onClick={toggle} disabled={busy}>
                        Cancel
                    </button>
                    <button type="submit" className="shell-btn shell-btn-primary" disabled={busy}>
                        <FileText size={14} /> {busy ? "Creating…" : "Create purchase invoice"}
                    </button>
                </ModalFooter>
            </form>
        </Modal>
    );
}
