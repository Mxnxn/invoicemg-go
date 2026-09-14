import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { FormGroup, Form, Input, Button, Row, Col } from "reactstrap";
import { Zap, Sliders } from "react-feather";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import DataTable from "../../../Common/DataTable/DataTable";
import { bankBackend } from "../../../Common/bank_backend";
import { useBanks, addBank } from "../../../Common/bankStore";
import { lifecycleBackend } from "../../Lifecycle/lifecycle_backend";
import { batchReceiveBackend } from "../batch_receive_backend";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { notifyError } from "../../../global/toast";
import { triggerFormAttention } from "../../../Common/attention";
import DateField from "../../../Common/DateField";
import Blank from "../../../Common/DataTable/Blank";

const today = () => new Date().toISOString().slice(0, 10);

const initialForm = { bankId: "", bankName: "", clientId: "", clientName: "", date: today(), amount: "", note: "" };

// Greedy oldest-first fill, mirrors routes/BatchReceive.js's /create "auto" branch exactly -
// kept in sync by hand since it's a small, self-contained bit of math (see extractNumericParts
// for precedent of this kind of intentional duplication in this codebase).
// Every open job is returned, whether or not this amount reaches it - a job that gets
// nothing still shows with applied 0. Previously the loop broke as soon as the money ran
// out, so entering an amount that covered the first job hid every other pending job, and
// the next one only "appeared" once the first was maxed out. The allocation itself is
// unchanged: oldest first, capped at each job's remaining. (The server re-does this; the
// preview only exists so the user can see it before saving.)
const computeAutoPreview = (openJobs, amount) => {
    let remaining = Number(amount) || 0;
    return (openJobs || []).map((job) => {
        const due = Number(job.remaining) || 0;
        if (due <= 0 || remaining <= 0) return { ...job, applied: 0 };
        const applied = Math.min(due, remaining);
        remaining = Number((remaining - applied).toFixed(2));
        return { ...job, applied };
    });
};


// The "New batch receive" form itself - shared between the global More > Batch Receive hub
// (rendered inline beside the list) and the Client page's batch-receive modal (same form,
// just with the client already fixed and no picker shown for it).
// showHeader: the standalone More > Batch Receive hub wants the card title, but inside
// BatchReceiveModal the reactstrap ModalHeader already says the same thing - rendering
// both stacked two identical headings on top of each other.
const BatchReceiveFormCard = ({ fixedClient, onCreated, showHeader = true }) => {
    const banks = useBanks();
    const [clients, setClients] = useState([]);
    const [form, setForm] = useState(() => (fixedClient ? { ...initialForm, clientId: fixedClient.id, clientName: fixedClient.name } : initialForm));
    const [mode, setMode] = useState("auto"); // "auto" (default) or "manual"
    const [openJobs, setOpenJobs] = useState([]);
    // Keyed by job id rather than a list of rows: manual mode now lists every pending job
    // the same way auto does, so there is nothing to add or remove - you just type against
    // the job you mean. Mirrors the Paid form (SupplierPaymentFormCard).
    const [manualAmounts, setManualAmounts] = useState({});
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState("");
    const formCardRef = useRef(null);
    const clientFieldRef = useRef(null);
    // The Create button used to be disabled until a client was picked, so clicking it did
    // nothing and said nothing. Now it stays live and points at the field that is missing.
    const [clientMissing, setClientMissing] = useState(false);
    const isFilling = Number(form.amount) > 0;

    useEffect(() => {
        if (isFilling) triggerFormAttention(formCardRef.current);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isFilling]);

    useEffect(() => {
        if (!fixedClient) lifecycleBackend.lookupClients().then((res) => setClients(res.data));
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    useEffect(() => {
        if (!form.clientId) {
            setOpenJobs([]);
            return;
        }
        batchReceiveBackend.lookupOpenJobs(form.clientId).then((res) => setOpenJobs(res.data));
        setManualAmounts({});
    }, [form.clientId]);

    const bankOptions = banks.map((b) => ({ id: b._id, name: b.name }));
    const clientOptions = clients.map((c) => ({
        id: c._id,
        name: c.clientFirm || c.clientName,
        search: [c.clientFirm, c.clientName, c.clientPhone].filter(Boolean).join(" ").toLowerCase(),
    }));

    const onBankCreateNew = async (query) => {
        if (!query || !query.trim()) return;
        const formData = new FormData();
        formData.set("name", query.trim());
        const res = await bankBackend.create(formData);
        addBank(res.data);
        setForm((f) => ({ ...f, bankId: res.data._id, bankName: res.data.name }));
    };

    const autoPreview = useMemo(() => computeAutoPreview(openJobs, form.amount), [openJobs, form.amount]);
    // What this client still owes across every pending job, and how much of the entered
    // amount actually lands. Shown so the figure being allocated can be read against the
    // total it is settling.
    const clientDue = useMemo(
        () => (openJobs || []).reduce((sum, j) => sum + (Number(j.remaining) || 0), 0),
        [openJobs]
    );
    const autoApplied = useMemo(() => autoPreview.reduce((sum, j) => sum + (Number(j.applied) || 0), 0), [autoPreview]);

    const setManualAmount = (jobId, value) => setManualAmounts((prev) => ({ ...prev, [jobId]: value }));


    const manualAllocated = Object.values(manualAmounts).reduce((sum, v) => sum + (Number(v) || 0), 0);
    const remainingToAllocate = Number((Number(form.amount || 0) - manualAllocated).toFixed(2));

    // Structural minimum only (who the money's for, and that manual rows point at real
    // jobs) - amount/allocation problems get a toast + blocked submit instead of a silently
    // disabled button, so the user knows *why* nothing happened.
    const canSubmit =
        form.clientId && (mode === "auto" || Object.values(manualAmounts).some((v) => Number(v) > 0));

    const resetForm = () => {
        setForm(fixedClient ? { ...initialForm, clientId: fixedClient.id, clientName: fixedClient.name } : initialForm);
        setMode("auto");
        setManualAmounts({});
        setOpenJobs([]);
        setError("");
    };

    const onSubmit = async (e) => {
        e.preventDefault();
        if (saving) return;
        if (!form.clientId) {
            setClientMissing(true);
            const field = clientFieldRef.current;
            if (field) {
                triggerFormAttention(field);
                const focusable = field.querySelector("input, button, [tabindex]");
                if (focusable) focusable.focus();
            }
            // Let the breathing settle, then clear so the next attempt can re-trigger it.
            window.setTimeout(() => setClientMissing(false), 2400);
            return;
        }
        if (!canSubmit) return;
        if (!(Number(form.amount) > 0)) {
            notifyError("Enter an amount before saving.");
            return;
        }
        if (mode === "manual") {
            if (remainingToAllocate < -0.01) {
                notifyError("Allocated amount exceeds the batch amount.");
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
            formData.set("client_id", form.clientId);
            if (form.bankId) formData.set("bank_id", form.bankId);
            formData.set("date", form.date);
            formData.set("amount", form.amount);
            formData.set("note", form.note);
            formData.set("mode", mode);
            if (mode === "manual") {
                // Only jobs actually given something - a blank row is not an allocation.
                formData.set(
                    "allocations",
                    JSON.stringify(
                        Object.entries(manualAmounts)
                            .filter(([, v]) => Number(v) > 0)
                            .map(([job_id, v]) => ({ job_id, amount: Number(v) }))
                    )
                );
            }
            const res = await batchReceiveBackend.createReceive(formData);
            onCreated(res.data);
            resetForm();
        } catch (err) {
            setError(err.message || "Couldn't record this batch receive.");
        } finally {
            setSaving(false);
        }
    };

    return (
        <div
            className={["shell-card", isFilling ? "shell-attention-active" : ""].filter(Boolean).join(" ")}
            ref={formCardRef}
            style={{
                "--pulse-color": "rgba(59, 130, 246, 0.55)",
                "--pulse-color-strong": "var(--xan-blue)",
                border: isFilling ? "2px solid var(--xan-blue)" : undefined,
            }}
        >
            {showHeader && (
                <div className="shell-card-header">
                    <span className="text-heading-brand">New Payment</span>
                </div>
            )}
            <div style={{ padding: 20 }}>
                <Form autoComplete="off" onSubmit={onSubmit}>
                    <Row>
                        <Col md="6">
                            <FormGroup>
                                <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                                    Bank
                                </label>
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
                        <Col md="6">
                            <FormGroup>
                                <label className="form-control-label pp fs-12">Date</label>
                                <DateField value={form.date} onChange={(e) => setForm((f) => ({ ...f, date: e.target.value }))} />
                            </FormGroup>
                        </Col>
                    </Row>
                    <Row>
                        {!fixedClient && (
                            <Col md="6">
                                <FormGroup innerRef={clientFieldRef} className={["brf-client-field", clientMissing ? "brf-client-missing" : ""].filter(Boolean).join(" ")}>
                                    <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                                        Client
                                    </label>
                                    <AssigneeDropdown
                                        value={form.clientName}
                                        placeholder="Select client"
                                        options={clientOptions}
                                        fullWidth
                                        menuWidth={320}
                                        onSelect={(id, name) => setForm((f) => ({ ...f, clientId: id || "", clientName: name || "" }))}
                                    />
                                </FormGroup>
                            </Col>
                        )}
                        <Col md={fixedClient ? "12" : "6"}>
                            <FormGroup>
                                <label className="form-control-label pp fs-12">Amount</label>
                                <Input
                                    type="number"
                                    value={form.amount}
                                    onChange={(e) => setForm((f) => ({ ...f, amount: e.target.value }))}
                                    placeholder="0"
                                />
                            </FormGroup>
                        </Col>
                    </Row>
                    <Row>
                        <Col md="12">
                            <FormGroup>
                                <label className="form-control-label pp fs-12">Notes</label>
                                <Input
                                    value={form.note}
                                    onChange={(e) => setForm((f) => ({ ...f, note: e.target.value }))}
                                    placeholder="Reference number, remarks..."
                                />
                            </FormGroup>
                        </Col>
                    </Row>

                    {/* Two checkboxes behaving as radios was both semantically wrong and ugly.
                        A segmented control says "pick exactly one" on sight. */}
                    <div className="segmented" role="radiogroup" aria-label="Allocation mode">
                        <button
                            type="button"
                            role="radio"
                            aria-checked={mode === "auto"}
                            className={["segmented-option", mode === "auto" ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setMode("auto")}
                        >
                            <Zap size={14} />
                            <span>Auto-allocate<small>oldest job first</small></span>
                        </button>
                        <button
                            type="button"
                            role="radio"
                            aria-checked={mode === "manual"}
                            className={["segmented-option", mode === "manual" ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setMode("manual")}
                        >
                            <Sliders size={14} />
                            <span>Manual<small>choose the jobs</small></span>
                        </button>
                    </div>

                    {mode === "auto" && form.clientId && (
                        <div style={{ marginBottom: 16 }}>
                            <div
                                className="text-label-caps"
                                style={{ color: "var(--text-tertiary)", marginBottom: 8, display: "flex", gap: 12 }}
                            >
                                <span>Pending job-ids</span>
                                <span style={{ marginLeft: "auto", color: "var(--status-red-text)" }}>
                                    Customer due ₹{RoundOff(clientDue)}
                                </span>
                            </div>
                            {autoPreview.length === 0 ? (
                                <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    No open jobs for this client.
                                </p>
                            ) : (
                                <DataTable>
                                    <thead>
                                        <tr>
                                            <th scope="col">Date</th>
                                            <th scope="col">Job</th>
                                            <th scope="col">Due</th>
                                            <th scope="col">Applied</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {autoPreview.map((j) => (
                                            <tr key={j._id} style={j.applied > 0 ? undefined : { opacity: 0.55 }}>
                                                <td className="cell-mono">{j.receivedDate || <Blank />}</td>
                                                <td className="cell-mono">
                                                    <Input className="cell-input" value={j.challanNumber} disabled />
                                                </td>
                                                <td className="cell-mono">₹{RoundOff(j.remaining)}</td>
                                                <td
                                                    className="cell-mono"
                                                    style={{ color: j.applied > 0 ? "var(--status-lime-text)" : "var(--text-tertiary)" }}
                                                >
                                                    {j.applied > 0 ? `₹${RoundOff(j.applied)}` : "—"}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                    <tfoot>
                                        <tr>
                                            <td colSpan={2} style={{ fontWeight: 600 }}>Total</td>
                                            <td className="cell-mono" style={{ fontWeight: 600 }}>₹{RoundOff(clientDue)}</td>
                                            <td className="cell-mono" style={{ fontWeight: 600, color: "var(--status-lime-text)" }}>
                                                ₹{RoundOff(autoApplied)}
                                            </td>
                                        </tr>
                                    </tfoot>
                                </DataTable>
                            )}
                        </div>
                    )}

                    {mode === "manual" && form.clientId && (
                        <div style={{ marginBottom: 16 }}>
                            <div
                                className="text-label-caps"
                                style={{ color: "var(--text-tertiary)", marginBottom: 8, display: "flex", gap: 12 }}
                            >
                                <span>Pending job-ids</span>
                                <span style={{ marginLeft: "auto", color: "var(--status-red-text)" }}>
                                    Customer due ₹{RoundOff(clientDue)}
                                </span>
                            </div>
                            {openJobs.length === 0 ? (
                                <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    No open jobs for this client.
                                </p>
                            ) : (
                                <>
                                    <DataTable>
                                        <thead>
                                            <tr>
                                                <th scope="col">Date</th>
                                                <th scope="col">Job</th>
                                                <th scope="col">Due</th>
                                                <th scope="col">Apply</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {openJobs.map((job) => {
                                                const value = manualAmounts[job._id] || "";
                                                // Typing more than a job is owed is the one mistake worth
                                                // flagging inline - the server rejects it, but not until save.
                                                const over = Number(value) > Number(job.remaining);
                                                return (
                                                    <tr key={job._id}>
                                                        <td className="cell-mono">{job.receivedDate || <Blank />}</td>
                                                        <td className="cell-mono">{job.challanNumber}</td>
                                                        <td className="cell-mono">₹{RoundOff(job.remaining)}</td>
                                                        <td>
                                                            <Input
                                                                className={`cell-input${over ? " is-required-missing" : ""}`}
                                                                type="number"
                                                                step="0.01"
                                                                placeholder="0.00"
                                                                value={value}
                                                                onChange={(e) => setManualAmount(job._id, e.target.value)}
                                                            />
                                                        </td>
                                                    </tr>
                                                );
                                            })}
                                        </tbody>
                                        <tfoot>
                                            <tr>
                                                <td colSpan={2} style={{ fontWeight: 600 }}>Total</td>
                                                <td className="cell-mono" style={{ fontWeight: 600 }}>
                                                    ₹{RoundOff(clientDue)}
                                                </td>
                                                <td className="cell-mono" style={{ fontWeight: 600, color: "var(--status-lime-text)" }}>
                                                    ₹{RoundOff(manualAllocated)}
                                                </td>
                                            </tr>
                                        </tfoot>
                                    </DataTable>
                                    <div className="d-flex justify-content-end" style={{ marginTop: 8 }}>
                                        <span
                                            className="text-body-small cell-mono"
                                            style={{ color: remainingToAllocate < 0 ? "var(--status-red-text)" : "var(--text-tertiary)" }}
                                        >
                                            Available to allocate: ₹{RoundOff(remainingToAllocate)}
                                        </span>
                                    </div>
                                </>
                            )}
                        </div>
                    )}

                    {error && (
                        <div className="text-body-small" style={{ color: "var(--status-red-text)", marginBottom: 8 }}>
                            {error}
                        </div>
                    )}

                    <div className="d-flex align-items-center" style={{ gap: 8 }}>
                        <Button type="submit" className="shell-btn shell-btn-primary" disabled={saving}>
                            Save Batch Receive
                        </Button>
                        <Button type="button" className="shell-btn shell-btn-secondary" onClick={resetForm} disabled={saving}>
                            Reset
                        </Button>
                    </div>
                </Form>
            </div>
        </div>
    );
};

export default BatchReceiveFormCard;
