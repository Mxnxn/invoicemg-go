import { Zap, Sliders } from "react-feather";
import React, { useEffect, useMemo, useRef, useState } from "react";
import { FormGroup, Form, Input, Button, Row, Col } from "reactstrap";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import DataTable from "../../../Common/DataTable/DataTable";
import { bankBackend } from "../../../Common/bank_backend";
import { useBanks, addBank } from "../../../Common/bankStore";
import { personBackend } from "../../../Common/person_backend";
import { supplierPaymentBackend } from "../supplier_payment_backend";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { notifyError } from "../../../global/toast";
import { triggerFormAttention } from "../../../Common/attention";
import { previewAutoAllocation, allocationError, invoiceDue } from "../../PurchaseReport/purchaseReportMath";
import DateField from "../../../Common/DateField";

const today = () => new Date().toISOString().slice(0, 10);

const initialForm = { bankId: "", bankName: "", supplierId: "", supplierName: "", date: today(), amount: "", note: "" };

// "Record a payment" - the outgoing counterpart of BatchReceiveFormCard. Same two modes: auto
// settles the supplier's oldest open invoices first, manual lets you say exactly which invoice
// each rupee went against. The server re-does both calculations; this preview only exists so
// you can see where the money will land before saving.
const SupplierPaymentFormCard = ({ onCreated }) => {
    const banks = useBanks();
    const [suppliers, setSuppliers] = useState([]);
    const [form, setForm] = useState(initialForm);
    const [mode, setMode] = useState("auto");
    const [openInvoices, setOpenInvoices] = useState([]);
    const [manualAmounts, setManualAmounts] = useState({});
    const [saving, setSaving] = useState(false);
    const formCardRef = useRef(null);
    const isFilling = Number(form.amount) > 0;

    useEffect(() => {
        if (isFilling) triggerFormAttention(formCardRef.current);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isFilling]);

    useEffect(() => {
        personBackend.list("Supplier").then((res) => setSuppliers(res.data));
    }, []);

    useEffect(() => {
        if (!form.supplierId) {
            setOpenInvoices([]);
            return;
        }
        supplierPaymentBackend.lookupOpenInvoices(form.supplierId).then((res) => setOpenInvoices(res.data));
        setManualAmounts({});
    }, [form.supplierId]);

    const bankOptions = banks.map((b) => ({ id: b._id, name: b.name }));
    const supplierOptions = suppliers.map((s) => ({
        id: s._id,
        name: s.firm || s.name,
        search: [s.firm, s.name, s.phone].filter(Boolean).join(" ").toLowerCase(),
    }));

    const autoPreview = useMemo(() => previewAutoAllocation(openInvoices, form.amount), [openInvoices, form.amount]);
    // Everything this supplier is still owed across the listed invoices.
    const supplierDue = useMemo(
        () => (openInvoices || []).reduce((sum, inv) => sum + invoiceDue(inv), 0),
        [openInvoices]
    );

    const manualTotal = useMemo(
        () => Object.values(manualAmounts).reduce((sum, value) => sum + (Number(value) || 0), 0),
        [manualAmounts]
    );

    const allocated = mode === "auto" ? Number(form.amount || 0) - autoPreview.remaining : manualTotal;
    const error = form.supplierId ? allocationError(allocated, form.amount) : "";
    const canSave = Boolean(form.supplierId) && Number(form.amount) > 0 && !error && !saving;

    // The Received tab lets you create a bank inline; this form had no "+ Create new" on
    // either picker, so a supplier or bank that didn't exist yet was a dead end - you had to
    // leave, create it elsewhere, and come back.
    const onBankCreateNew = async (query) => {
        if (!query || !query.trim()) return;
        const formData = new FormData();
        formData.set("name", query.trim());
        const res = await bankBackend.create(formData);
        addBank(res.data);
        setForm((f) => ({ ...f, bankId: res.data._id, bankName: res.data.name }));
    };

    const onSupplierCreateNew = async (query) => {
        if (!query || !query.trim()) return;
        const formData = new FormData();
        formData.set("name", query.trim());
        formData.set("type", "Supplier");
        const res = await personBackend.create(formData);
        setSuppliers((prev) => [...prev, res.data]);
        setForm((f) => ({ ...f, supplierId: res.data._id, supplierName: res.data.firm || res.data.name }));
    };

    const onSave = async () => {
        if (!canSave) return;
        setSaving(true);
        try {
            const formData = new FormData();
            formData.set("supplier_id", form.supplierId);
            formData.set("amount", form.amount);
            formData.set("date", form.date);
            formData.set("note", form.note);
            formData.set("mode", mode);
            if (form.bankId) formData.set("bank_id", form.bankId);
            if (mode === "manual") {
                const allocations = Object.entries(manualAmounts)
                    .filter(([, value]) => Number(value) > 0)
                    .map(([purchase_invoice_id, value]) => ({ purchase_invoice_id, amount: Number(value) }));
                formData.set("allocations", JSON.stringify(allocations));
            }
            const res = await supplierPaymentBackend.createPayment(formData);
            onCreated(res.data);
            setForm({ ...initialForm, bankId: form.bankId, bankName: form.bankName });
            setManualAmounts({});
            setOpenInvoices([]);
        } catch (err) {
            notifyError(err.message || "Couldn't record this payment.");
        } finally {
            setSaving(false);
        }
    };

    return (
        <div
            className={["shell-card", isFilling ? "shell-attention-active" : ""].filter(Boolean).join(" ")}
            ref={formCardRef}
        >
            <div className="shell-card-header">
                <span className="text-heading-brand">Supplier Payment</span>
            </div>
            <div style={{ padding: 20 }}>
                <Form autoComplete="off">
                    <Row>
                        <Col md="6">
                            <FormGroup>
                                <label className="form-control-label">Supplier</label>
                                <AssigneeDropdown
                                    value={form.supplierName}
                                    placeholder="Select supplier"
                                    options={supplierOptions}
                                    fullWidth
                                    menuWidth={320}
                                    onSelect={(id, name) => setForm((f) => ({ ...f, supplierId: id || "", supplierName: name || "" }))}
                                    onCreateNew={onSupplierCreateNew}
                                />
                            </FormGroup>
                        </Col>
                        <Col md="6">
                            <FormGroup>
                                <label className="form-control-label">Paid from</label>
                                <AssigneeDropdown
                                    value={form.bankName}
                                    placeholder="Select bank"
                                    options={bankOptions}
                                    fullWidth
                                    onSelect={(id, name) => setForm((f) => ({ ...f, bankId: id || "", bankName: name || "" }))}
                                    onCreateNew={onBankCreateNew}
                                />
                            </FormGroup>
                        </Col>
                        <Col md="4">
                            <FormGroup>
                                <label className="form-control-label">Date</label>
                                <DateField value={form.date} onChange={(e) => setForm((f) => ({ ...f, date: e.target.value }))} />
                            </FormGroup>
                        </Col>
                        <Col md="4">
                            <FormGroup>
                                <label className="form-control-label">Amount</label>
                                <Input
                                    type="number"
                                    placeholder="0.00"
                                    value={form.amount}
                                    onChange={(e) => setForm((f) => ({ ...f, amount: e.target.value }))}
                                />
                            </FormGroup>
                        </Col>
                        <Col md="4">
                            <FormGroup>
                                <label className="form-control-label">Reference</label>
                                <Input
                                    type="text"
                                    placeholder="Cheque no. / UTR"
                                    value={form.note}
                                    onChange={(e) => setForm((f) => ({ ...f, note: e.target.value }))}
                                />
                            </FormGroup>
                        </Col>
                    </Row>

                    {/* Same control as the Received side - the two forms are mirror images
                        and should not offer the same choice in two different shapes. */}
                    <div className="segmented" role="radiogroup" aria-label="Allocation mode">
                        <button
                            type="button"
                            role="radio"
                            aria-checked={mode === "auto"}
                            className={["segmented-option", mode === "auto" ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setMode("auto")}
                        >
                            <Zap size={14} />
                            <span>
                                Auto<small>oldest invoice first</small>
                            </span>
                        </button>
                        <button
                            type="button"
                            role="radio"
                            aria-checked={mode === "manual"}
                            className={["segmented-option", mode === "manual" ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setMode("manual")}
                        >
                            <Sliders size={14} />
                            <span>
                                Manual<small>choose the invoices</small>
                            </span>
                        </button>
                    </div>

                    {form.supplierId && openInvoices.length === 0 && (
                        <div className="text-body-small" style={{ color: "var(--text-tertiary)", marginBottom: 12 }}>
                            This supplier has no outstanding invoices.
                        </div>
                    )}

                    {openInvoices.length > 0 && (
                        <>
                        {/* The total being settled, shown beside the amount being allocated -
                            same reading the Received side gives for a customer. */}
                        <div
                            className="text-label-caps"
                            style={{ color: "var(--text-tertiary)", marginBottom: 8, display: "flex", gap: 12 }}
                        >
                            <span>Outstanding invoices</span>
                            <span style={{ marginLeft: "auto", color: "var(--status-red-text)" }}>
                                Supplier due ₹{RoundOff(supplierDue)}
                            </span>
                        </div>
                        <DataTable>
                            <thead>
                                <tr>
                                    <th scope="col">Invoice</th>
                                    <th scope="col">Date</th>
                                    <th scope="col">Outstanding</th>
                                    <th scope="col">{mode === "auto" ? "Will apply" : "Apply"}</th>
                                </tr>
                            </thead>
                            <tbody>
                                {openInvoices.map((invoice) => {
                                    const applied = autoPreview.allocations.find((a) => a._id === invoice._id)?.applied || 0;
                                    return (
                                        <tr key={invoice._id}>
                                            <td>{invoice.invoiceNumber}</td>
                                            <td className="cell-mono">{invoice.date}</td>
                                            <td className="cell-mono">₹{RoundOff(invoiceDue(invoice))}</td>
                                            <td>
                                                {mode === "auto" ? (
                                                    <span className="cell-mono">{applied > 0 ? `₹${RoundOff(applied)}` : "—"}</span>
                                                ) : (
                                                    <Input
                                                        type="number"
                                                        placeholder="0.00"
                                                        value={manualAmounts[invoice._id] || ""}
                                                        onChange={(e) =>
                                                            setManualAmounts((prev) => ({ ...prev, [invoice._id]: e.target.value }))
                                                        }
                                                    />
                                                )}
                                            </td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                            <tfoot>
                                <tr>
                                    <td colSpan={2} style={{ fontWeight: 600 }}>
                                        Total
                                    </td>
                                    <td className="cell-mono" style={{ fontWeight: 600 }}>₹{RoundOff(supplierDue)}</td>
                                    <td className="cell-mono" style={{ fontWeight: 600, color: "var(--status-lime-text)" }}>
                                        ₹{RoundOff(allocated)}
                                    </td>
                                </tr>
                            </tfoot>
                        </DataTable>
                        </>
                    )}

                    {error && (
                        <div className="text-body-small" style={{ color: "var(--xan-rose)", marginTop: 12 }}>
                            {error}
                        </div>
                    )}

                    <div className="d-flex justify-content-end" style={{ marginTop: 16 }}>
                        <Button type="button" className="shell-btn shell-btn-primary" disabled={!canSave} onClick={onSave}>
                            {saving ? "Saving…" : "Record payment"}
                        </Button>
                    </div>
                </Form>
            </div>
        </div>
    );
};

export default SupplierPaymentFormCard;
