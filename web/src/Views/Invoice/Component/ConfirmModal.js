import React, { useCallback, useEffect, useMemo, useState } from "react";
import { ModalBody, Col, Modal, ModalHeader, Row, ModalFooter, Button, FormGroup, Input } from "reactstrap";
import { Plus, Trash2, Zap, Sliders } from "react-feather";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { invoiceBackend } from "../invoice_backend";
import DataTable from "../../../Common/DataTable/DataTable";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import { notifyError } from "../../../global/toast";
import { bankBackend } from "../../../Common/bank_backend";
import DateField from "../../../Common/DateField";

const today = () => new Date().toISOString().slice(0, 10);

// Greedy fill in the invoice's own entry order, mirrors routes/Invoice.js's /paid "auto"
// partial-payment branch - kept in sync by hand (see BatchReceiveFormCard's
// computeAutoPreview for precedent of this kind of intentional duplication).
const computeAutoPreview = (dueEntries, amount) => {
    let remaining = Number(amount) || 0;
    const result = [];
    for (const entry of dueEntries) {
        if (remaining <= 0) break;
        const due = Number(entry.total) || 0;
        if (due <= 0) continue;
        const applied = Math.min(due, remaining);
        result.push({ ...entry, due, applied });
        remaining = Number((remaining - applied).toFixed(2));
    }
    return result;
};

const emptyManualRow = () => ({ entryId: "", entryLabel: "", amount: "" });

const ConfirmModal = ({ onCancelHandler, onPaid, modal, invoice }) => {
    const [amount, setAmount] = useState("");
    const [mode, setMode] = useState("auto");
    const [manualRows, setManualRows] = useState([emptyManualRow()]);
    const [jobLabels, setJobLabels] = useState({});
    const [history, setHistory] = useState({ list: [], loaded: false });
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState("");
    const [banks, setBanks] = useState([]);
    const [bankId, setBankId] = useState("");
    const [bankName, setBankName] = useState("");
    const [date, setDate] = useState(today());
    const [note, setNote] = useState("");

    useEffect(() => {
        bankBackend.list().then((res) => setBanks(res.data));
    }, []);

    const bankOptions = banks.map((b) => ({ id: b._id, name: b.name }));

    const onBankCreateNew = async (query) => {
        if (!query || !query.trim()) return;
        const formData = new FormData();
        formData.set("name", query.trim());
        const res = await bankBackend.create(formData);
        setBanks((prev) => [...prev, res.data]);
        setBankId(res.data._id);
        setBankName(res.data.name);
    };

    const dueEntries = useMemo(() => (invoice?.entries || []).filter((entry) => Number(entry.total) > 0), [invoice]);
    const dueAmount = Number(invoice?.total || 0) - Number(invoice?.receivedAmount || 0);

    const getHistory = useCallback(async () => {
        try {
            const formData = new FormData();
            formData.set("uid", localStorage.getItem("uid"));
            formData.set("invoice_id", invoice?._id);
            const res = await invoiceBackend.getInvoiceReceives(formData);
            setHistory({ list: res.data, loaded: true });
        } catch (error) {
            setHistory({ list: [], loaded: true });
        }
    }, [invoice]);

    useEffect(() => {
        if (!invoice?._id) return;
        getHistory();
        setAmount("");
        setMode("auto");
        setManualRows([emptyManualRow()]);
        setError("");
        setBankId("");
        setBankName("");
        setDate(today());
        setNote("");
    }, [invoice, getHistory]);

    useEffect(() => {
        if (dueEntries.length === 0) {
            setJobLabels({});
            return;
        }
        invoiceBackend
            .entriesJobs(dueEntries.map((entry) => entry._id))
            .then((res) => setJobLabels(res.data || {}))
            .catch(() => setJobLabels({}));
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [invoice]);

    const labelFor = (entry) => {
        const job = jobLabels[entry._id];
        const item = entry.material || entry.description || "Entry";
        return `${job ? `Job ${job} — ` : ""}${item} (due ₹${RoundOff(entry.total)})`;
    };

    const entryOptions = dueEntries.map((entry) => ({ id: entry._id, name: labelFor(entry) }));

    const autoPreview = useMemo(() => computeAutoPreview(dueEntries, amount), [dueEntries, amount]);

    const updateManualRow = (index, patch) => {
        setManualRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
    };
    const addManualRow = () => setManualRows((prev) => [...prev, emptyManualRow()]);
    const removeManualRow = (index) => setManualRows((prev) => prev.filter((_, i) => i !== index));

    const manualAllocated = manualRows.reduce((sum, r) => sum + (Number(r.amount) || 0), 0);
    const remainingToAllocate = Number((Number(amount || 0) - manualAllocated).toFixed(2));

    const canSubmit = Number(amount) > 0 && (mode === "auto" || (manualRows.length > 0 && manualRows.every((r) => r.entryId)));

    const onSubmit = async () => {
        if (saving) return;
        if (!(Number(amount) > 0)) {
            notifyError("Enter an amount before saving.");
            return;
        }
        if (mode === "manual") {
            if (!canSubmit) {
                notifyError("Pick a job/entry for every allocation row.");
                return;
            }
            if (remainingToAllocate < -0.01) {
                notifyError("Allocated amount exceeds the amount received.");
                return;
            }
            if (remainingToAllocate > 0.01) {
                notifyError("Allocate the full amount before saving.");
                return;
            }
        }
        setSaving(true);
        setError("");
        try {
            const formData = new FormData();
            formData.set("invoice_id", invoice._id);
            formData.set("receivedAmount", amount);
            formData.set("mode", mode);
            formData.set("date", date);
            formData.set("note", note);
            if (bankId) formData.set("bank_id", bankId);
            if (mode === "manual") {
                formData.set("allocations", JSON.stringify(manualRows.map((r) => ({ entry_id: r.entryId, amount: Number(r.amount) }))));
            }
            await invoiceBackend.paidInvoice(formData);
            onPaid();
        } catch (err) {
            setError(err.message || "Couldn't record this payment.");
        } finally {
            setSaving(false);
        }
    };

    return (
        <Modal isOpen={modal} toggle={onCancelHandler} style={{ zIndex: 10000000000 }}>
            <ModalHeader className="bg " toggle={onCancelHandler}>
                <h3 className="text fs-24 geb">Close Invoice</h3>
            </ModalHeader>
            <ModalBody className="bg">
                <div className="d-flex align-items-center" style={{ gap: 6, marginBottom: 16 }}>
                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Due Amount:
                    </span>
                    <span className="cell-mono" style={{ color: "var(--status-red-text)", fontWeight: 700, fontSize: 16 }}>
                        ₹{RoundOff(dueAmount)}
                    </span>
                </div>
                <Row>
                    <Col xl="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                                Bank
                            </label>
                            <AssigneeDropdown
                                value={bankName}
                                placeholder="Select bank"
                                options={bankOptions}
                                fullWidth
                                onSelect={(id, name) => {
                                    setBankId(id || "");
                                    setBankName(name || "");
                                }}
                                onCreateNew={onBankCreateNew}
                            />
                        </FormGroup>
                    </Col>
                    <Col xl="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Date</label>
                            <DateField value={date} onChange={(e) => setDate(e.target.value)} />
                        </FormGroup>
                    </Col>
                </Row>
                <Row>
                    <Col xl="12">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Amount</label>
                            <Input
                                className="form-control-alternative nn"
                                placeholder="0"
                                type="number"
                                value={amount}
                                onChange={(e) => setAmount(e.target.value)}
                            />
                        </FormGroup>
                    </Col>
                </Row>
                <Row>
                    <Col xl="12">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Notes</label>
                            <Input
                                className="form-control-alternative nn"
                                value={note}
                                onChange={(e) => setNote(e.target.value)}
                                placeholder="Reference number, remarks..."
                            />
                        </FormGroup>
                    </Col>
                </Row>

                {/* Same segmented control as the bank transfer forms - two checkboxes acting
                    as radios were semantically wrong, and the label text was unreadable on a
                    dark field. segmented--field sizes it like a half-width input so it lines
                    up with the fields above. */}
                <div className="segmented segmented--field" role="radiogroup" aria-label="Allocation mode">
                    <button
                        type="button"
                        role="radio"
                        aria-checked={mode === "auto"}
                        className={["segmented-option", mode === "auto" ? "active" : ""].filter(Boolean).join(" ")}
                        onClick={() => setMode("auto")}
                    >
                        <Zap size={14} />
                        <span>
                            Auto-close<small>oldest job first</small>
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
                            Manual<small>choose the entries</small>
                        </span>
                    </button>
                </div>

                {mode === "auto" && (
                    <div style={{ marginBottom: 16 }}>
                        <div className="text-label-caps" style={{ color: "var(--text-tertiary)", marginBottom: 8 }}>
                            Will close
                        </div>
                        {autoPreview.length === 0 ? (
                            <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                {Number(amount) > 0 ? "Nothing left to apply this to." : "Enter an amount to preview."}
                            </p>
                        ) : (
                            <DataTable>
                                <thead>
                                    <tr>
                                        <th scope="col">Job / Entry</th>
                                        <th scope="col">Due</th>
                                        <th scope="col">Applied</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {autoPreview.map((entry) => (
                                        <tr key={entry._id}>
                                            <td>{jobLabels[entry._id] ? `Job ${jobLabels[entry._id]}` : entry.material || entry.description || "Entry"}</td>
                                            <td className="cell-mono">₹{RoundOff(entry.due)}</td>
                                            <td className="cell-mono">₹{RoundOff(entry.applied)}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </DataTable>
                        )}
                    </div>
                )}

                {mode === "manual" && (
                    <div style={{ marginBottom: 16 }}>
                        <div className="text-label-caps" style={{ color: "var(--text-tertiary)", marginBottom: 8 }}>
                            Allocate manually
                        </div>
                        <DataTable>
                            <thead>
                                <tr>
                                    <th scope="col" style={{ width: 32 }} />
                                    <th scope="col" style={{ minWidth: 260 }}>
                                        Job / Entry
                                    </th>
                                    <th scope="col" style={{ minWidth: 100 }}>
                                        Amount
                                    </th>
                                </tr>
                            </thead>
                            <tbody>
                                {manualRows.map((row, index) => (
                                    <tr key={index}>
                                        <td>
                                            <button
                                                type="button"
                                                className="shell-icon-btn"
                                                aria-label="Remove row"
                                                disabled={manualRows.length <= 1}
                                                onClick={() => removeManualRow(index)}
                                                style={{ color: "var(--xan-rose)" }}
                                            >
                                                <Trash2 size={14} />
                                            </button>
                                        </td>
                                        <td>
                                            <AssigneeDropdown
                                                value={row.entryLabel}
                                                placeholder="Select job/entry"
                                                menuWidth={320}
                                                options={entryOptions.filter((o) => o.id === row.entryId || !manualRows.some((r) => r.entryId === o.id))}
                                                onSelect={(id, name) => updateManualRow(index, { entryId: id || "", entryLabel: name || "" })}
                                            />
                                        </td>
                                        <td>
                                            <Input
                                                className="cell-input"
                                                type="number"
                                                step="0.01"
                                                value={row.amount}
                                                onChange={(e) => updateManualRow(index, { amount: e.target.value })}
                                            />
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </DataTable>
                        <div className="d-flex justify-content-between align-items-center" style={{ marginTop: 8 }}>
                            <Button
                                type="button"
                                size="sm"
                                className="shell-btn shell-btn-sm shell-btn-secondary d-flex align-items-center"
                                style={{ gap: 4 }}
                                onClick={addManualRow}
                            >
                                <Plus size={13} />
                                Add Row
                            </Button>
                            <span
                                className="text-body-small cell-mono"
                                style={{ color: remainingToAllocate < 0 ? "var(--status-red-text)" : "var(--text-tertiary)" }}
                            >
                                Available to allocate: ₹{RoundOff(remainingToAllocate)}
                            </span>
                        </div>
                    </div>
                )}

                {error && (
                    <div className="text-body-small" style={{ color: "var(--status-red-text)", marginBottom: 8 }}>
                        {error}
                    </div>
                )}

                {history.loaded && (
                    <Row>
                        <Col xl="12">
                            <FormGroup>
                                <label className="form-control-label pp fs-12">Previous Received</label>
                                <DataTable>
                                    <thead>
                                        <tr>
                                            <th scope="col">#</th>
                                            <th scope="col">Date</th>
                                            <th scope="col">Bank</th>
                                            <th scope="col">Amount</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {history.list.map((el, index) => (
                                            <tr key={index}>
                                                <td className="cell-mono">{index + 1}</td>
                                                <td className="cell-mono">{getDate(el.date)}</td>
                                                <td>{el.bank_id?.name || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                                <td className="cell-mono">{RoundOff(el.amount)}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </DataTable>
                            </FormGroup>
                        </Col>
                    </Row>
                )}
            </ModalBody>
            <ModalFooter className="bg">
                <Button color="primary" className="btn fira btn-success" onClick={onSubmit} disabled={!canSubmit || saving}>
                    Paid
                </Button>{" "}
                <Button color="secondary" className="btn fira btn-warning" onClick={onCancelHandler}>
                    Cancel
                </Button>
            </ModalFooter>
        </Modal>
    );
};

export default ConfirmModal;
