import React, { useState, useEffect } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup, Input, Button, Row, Col } from "reactstrap";
import { Plus, Trash2, Lock, Unlock, X } from "react-feather";
import AssigneeDropdown from "./AssigneeDropdown";
import DataTable from "../../../Common/DataTable/DataTable";
import { lifecycleBackend } from "../lifecycle_backend";
import { rowAmount, rowTotal, jobGrandTotal, emptyJobRow, rowNeedsField, missingRowFields } from "../jobMath";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import QuickCreateClientModal from "../../Client/component/QuickCreateClientModal";
import QuickCreateProductModal from "../../Material/component/QuickCreateProductModal";
import OptionalRowField from "../../../Common/DataTable/OptionalRowField";
import AddRowButton from "../../../Common/DataTable/AddRowButton";
import RowSizeCells, { QTY_COL_PX, SIZE_COL_PX } from "../../../Common/DataTable/RowSizeCells";
import { splitGst } from "../../../Common/gst";
import { gstPercent, isIgstJob, applyIgstEdit, taxFromMaterial, toIgstRow, toGstRow } from "../jobTax";
import MissingFieldsHint from "../../../Common/MissingFieldsHint";
import { notifyWarning } from "../../../global/toast";
import "../lifecycle.css";
import DateField from "../../../Common/DateField";

const today = () => new Date().toISOString().slice(0, 10);

const initialHeader = {
    receivedDate: "",
    challanNumber: "",
    advance: "0",
    clientId: "",
    clientName: "",
};

// A job-id groups several line-item rows together (one estimation, several print jobs) -
// this form mirrors CreateQuotationModal's row-table shape.
//
// A billed row (row.entry_id set) used to render permanently disabled. entry_id is a record
// of what a row became, not a statement about the present: deleting the invoice clears
// has_issued but leaves entry_id behind, so those rows stayed frozen forever and the job
// could never be corrected again. The job's own lock decides now - which is the same rule
// the server applies (Helpers/JobLock.js), so the form can no longer disagree with it.
// GST is stored split across cgst/sgst (9 + 9 for an 18% intrastate sale) but entered and
// read as one number. These two keep the form showing 18 while the row keeps 9 + 9.


const CreateJobModal = ({ isOpen, toggle, onCreate, onUpdate, editingJob, fixedClient, onLockChange }) => {
    const [header, setHeader] = useState(initialHeader);
    const [rows, setRows] = useState([emptyJobRow()]);
    const [submitError, setSubmitError] = useState("");
    const [lockBusy, setLockBusy] = useState(false);
    // Server-computed (routes/Lifecycle.js puts `lock` on every job it returns).
    const lock = editingJob?.lock;
    const rowsFrozen = Boolean(lock?.invoiced) && !lock?.unlocked;

    const onToggleLock = async () => {
        if (!editingJob?._id) return;
        setLockBusy(true);
        try {
            const formData = new FormData();
            formData.set("job_id", editingJob._id);
            formData.set("unlocked", String(!lock?.unlocked));
            const res = await lifecycleBackend.unlockJob(formData);
            onLockChange?.(res.data);
            if (!lock?.unlocked) {
                notifyWarning("This job-id is billed on an invoice. Changes you save here rewrite that invoice.");
            }
        } catch (error) {
            // The global interceptor raises the toast.
        } finally {
            setLockBusy(false);
        }
    };
    const [clients, setClients] = useState([]);
    const [materials, setMaterials] = useState([]);
    const [quickCreateClient, setQuickCreateClient] = useState(null); // null closed, "" or a query string when open
    // Same shape, plus the row that asked - so the new product can be dropped straight into
    // the row the user was already filling in rather than making them pick it again.
    const [quickCreateProduct, setQuickCreateProduct] = useState(null); // { query, rowIndex } | null

    useEffect(() => {
        if (!isOpen) return;
        if (!fixedClient) lifecycleBackend.lookupClients().then((res) => setClients(res.data));
        lifecycleBackend.lookupMaterials({}).then((res) => setMaterials(res.data));

        if (editingJob) {
            setHeader({
                receivedDate: editingJob.receivedDate || "",
                challanNumber: editingJob.challanNumber || "",
                advance: String(editingJob.advance ?? "0"),
                clientId: editingJob.client_id?._id || "",
                clientName: editingJob.client_id?.clientFirm || editingJob.client_id?.clientName || "",
            });
            setRows(
                editingJob.rows && editingJob.rows.length
                    ? editingJob.rows.map((row) => ({
                          _id: row._id,
                          material: row.material || "",
                          description: row.description || "",
                          hasDimensions: row.hasDimensions !== false,
                          length: row.length || "1",
                          width: row.width || "1",
                          qty: row.qty ?? 1,
                          rate: row.rate ?? 0,
                          cgst: row.cgst ?? 0,
                          sgst: row.sgst ?? 0,
                          igst: row.igst ?? 0,
                          discount: row.discount ?? 0,
                          charges: row.charges ?? 0,
                          entry_id: row.entry_id || null,
                      }))
                    : [emptyJobRow()]
            );
            return;
        }
        setHeader(
            fixedClient
                ? { ...initialHeader, receivedDate: today(), clientId: fixedClient.id, clientName: fixedClient.name }
                : { ...initialHeader, receivedDate: today() }
        );
        setRows([emptyJobRow()]);
        lifecycleBackend.nextChallanNumber().then((res) => {
            if (res.data.challanNumber) {
                setHeader((f) => ({ ...f, challanNumber: res.data.challanNumber }));
            }
        });
    }, [isOpen, editingJob, fixedClient]);

    const clientOptions = clients.map((c) => ({
        id: c._id,
        name: c.clientFirm || c.clientName,
        search: [c.clientFirm, c.clientName, c.clientPhone].filter(Boolean).join(" ").toLowerCase(),
    }));
    const materialOptions = materials.map((m) => ({ id: m._id, name: m.material_name }));

    const onHeaderChange = (e) => setHeader({ ...header, [e.target.name]: e.target.value });

    const onClientSelect = (id, name) => setHeader({ ...header, clientId: id || "", clientName: name || "" });

    const onClientCreated = (client) => {
        setClients((prev) => [client, ...prev]);
        setHeader((h) => ({ ...h, clientId: client._id, clientName: client.clientFirm || client.clientName }));
    };

    const updateRow = (index, patch) => {
        setRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
    };

    const onMaterialSelect = (index, id, name) => {
        const material = materials.find((m) => m._id === id);
        // splitGst halves the product's combined rate across CGST and SGST (Common/gst.js).
        // Dumping the whole rate into cgst alone still charges the customer correctly, but
        // misreports the filing split - GstReport.js reads cgst/sgst per row.
        // On an IGST job the product's rate goes to IGST whole, rather than being halved
        // across CGST/SGST - an interstate sale has no intrastate split to report.
        if (!material) {
            updateRow(index, { material: name || "" });
            return;
        }
        // Which side the rate lands on depends on the regime the job is in - igstJob, not the
        // rows. A job switched to IGST before any rate was typed has igst 0 on every row, so
        // re-deriving it from the rows reads as intrastate and halves the product's tax into
        // CGST/SGST while the IGST column sits there showing nothing.
        updateRow(index, {
            material: name || "",
            rate: Number(material.material_rate),
            ...taxFromMaterial(material, igstJob),
        });
    };

    // Whether this job is interstate. Derived from the rows, so an edited job reads as
    // whatever it actually contains rather than trusting a stored flag.
    // IGST is a COLUMN when the job is interstate, not a chip tucked into Extra. Reaching for
    // it swaps the GST% column out for an IGST% one carrying the same rate; the X in its
    // header swaps back. Held as state as well as derived from the rows so a zero-rated job
    // can still be switched - isIgstJob alone cannot see a regime nobody has typed a rate for.
    const [igstMode, setIgstMode] = useState(false);
    const igstJob = isIgstJob(rows) || igstMode;

    // Every row moves together: the place of supply belongs to the sale, not to one line of
    // it. toIgstRow carries the product's tax across (igst || the CGST+SGST it had), and
    // toGstRow restores what it was, so the rate is never lost in either direction.
    const openIgstColumn = () => {
        setIgstMode(true);
        setRows((prev) => prev.map(toIgstRow));
    };
    const closeIgstColumn = () => {
        setIgstMode(false);
        setRows((prev) => prev.map(toGstRow));
    };

    // A job is IGST or GST, never both - the place of supply belongs to the sale, not to one
    // line of it. So setting IGST on any row converts every row, and clearing the last IGST
    // converts them all back. The rate itself carries across either way: it is the same tax
    // at the same percentage, only recorded on the other side.
    const onIgstChange = (index, value) => {
        setRows((prev) => applyIgstEdit(prev, index, value));
    };

    const addRow = (withDimensions) => setRows((prev) => [...prev, emptyJobRow(withDimensions)]);
    const removeRow = (index) => setRows((prev) => (prev[index].entry_id ? prev : prev.filter((_, i) => i !== index)));

    // A row is only "real" once a product is picked. Before that its blank measurements are
    // just an untouched spare row; after it they are missing values, because the amount is
    // qty x length x width x rate (jobMath.rowAmount) and any zero silently makes the row
    // worth nothing. Locked rows are already converted to Entries, so they are exempt.
    const anyRowNeeds = (field) => rows.some((row) => rowNeedsField(row, field));

    const canSubmit =
        header.challanNumber.trim() && header.clientId && rows.length > 0 && missingRowFields(rows).length === 0;

    const onSubmit = () => {
        if (!canSubmit) return;
        const formData = new FormData();
        if (editingJob) formData.set("job_id", editingJob._id);
        formData.set("client_id", header.clientId);
        formData.set("challanNumber", header.challanNumber);
        formData.set("receivedDate", header.receivedDate);
        formData.set("advance", Number(header.advance) || 0);
        formData.set("rows", JSON.stringify(rows));
        setSubmitError("");
        const action = editingJob ? onUpdate(formData) : onCreate(formData);
        action
            .then(() => {
                setHeader(initialHeader);
                setRows([emptyJobRow()]);
                toggle();
            })
            .catch((err) => setSubmitError(err.message || "Failed to save job."));
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} size="xl" style={{ maxWidth: "95vw" }}>
            <ModalHeader toggle={toggle}>{editingJob ? "Edit Job" : "Create Job"}</ModalHeader>
            <ModalBody>
                {/* The lock lives here, where the editing actually happens - it was only on
                    the Job detail modal before, which meant unlocking and editing were two
                    different screens and the form gave no hint why its rows were dead. */}
                {lock?.invoiced && (
                    <div className="job-lock-bar" style={{ marginBottom: 16 }}>
                        {lock.unlocked ? <Unlock size={14} /> : <Lock size={14} />}
                        <span>
                            {lock.unlocked
                                ? "Unlocked - saving rewrites the invoice this job is billed on."
                                : "Billed on an invoice, so its values are locked. Unlock to correct them."}
                        </span>
                        <button
                            type="button"
                            className="shell-btn shell-btn-secondary job-lock-btn"
                            onClick={onToggleLock}
                            disabled={lockBusy}
                        >
                            {lockBusy ? "…" : lock.unlocked ? "Lock" : "Unlock"}
                        </button>
                    </div>
                )}
                <Row>
                    <Col md="3">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Received Date</label>
                            <DateField name="receivedDate" value={header.receivedDate} onChange={onHeaderChange} />
                        </FormGroup>
                    </Col>
                    <Col md="3">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Job Number<span className="required-star">*</span></label>
                            <Input name="challanNumber" placeholder="MG/26-27/00001" value={header.challanNumber} onChange={onHeaderChange} />
                        </FormGroup>
                    </Col>
                    <Col md="3">
                        <FormGroup>
                            <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                                Client<span className="required-star">*</span>
                            </label>
                            {fixedClient ? (
                                <Input value={header.clientName} disabled />
                            ) : (
                                <AssigneeDropdown
                                    value={header.clientName}
                                    placeholder="Select client"
                                    options={clientOptions}
                                    onSelect={onClientSelect}
                                    fullWidth
                                    variant="dashed"
                                    menuWidth={320}
                                    onCreateNew={(query) => setQuickCreateClient(query)}
                                />
                            )}
                        </FormGroup>
                    </Col>
                    <Col md="3">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Advance</label>
                            <Input type="number" name="advance" value={header.advance} onChange={onHeaderChange} />
                        </FormGroup>
                    </Col>
                </Row>

                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col" style={{ width: 32 }} />
                            <th scope="col" style={{ minWidth: 160 }}>
                                Material
                            </th>
                            <th scope="col" style={{ minWidth: 160 }}>
                                Description
                            </th>
                            <th scope="col" style={{ minWidth: SIZE_COL_PX, width: SIZE_COL_PX }}>
                                Length
                                {anyRowNeeds("length") && <span className="required-star">*</span>}
                            </th>
                            <th scope="col" style={{ minWidth: SIZE_COL_PX, width: SIZE_COL_PX }}>
                                Width
                                {anyRowNeeds("width") && <span className="required-star">*</span>}
                            </th>
                            <th scope="col" style={{ minWidth: QTY_COL_PX, width: QTY_COL_PX }}>
                                Qty
                                {anyRowNeeds("qty") && <span className="required-star">*</span>}
                            </th>
                            <th scope="col" style={{ minWidth: QTY_COL_PX, width: QTY_COL_PX }}>
                                Rate
                                {anyRowNeeds("rate") && <span className="required-star">*</span>}
                            </th>
                            {/* One tax column, showing whichever side this job is on. The two
                                can never both apply, so a second column would only ever be
                                dead width on a table already wide enough to scroll. */}
                            <th scope="col" style={{ minWidth: QTY_COL_PX, width: QTY_COL_PX }} title={igstJob ? "Interstate sale - one IGST rate per row" : "Split evenly into CGST + SGST"}>
                                {igstJob ? (
                                    <span className="d-inline-flex align-items-center" style={{ gap: 6 }}>
                                        IGST%
                                        <button
                                            type="button"
                                            className="igst-col-close"
                                            aria-label="Switch back to GST"
                                            title="Back to GST — the rate moves with it"
                                            disabled={rows.some((r) => r.entry_id)}
                                            onClick={closeIgstColumn}
                                        >
                                            <X size={13} strokeWidth={2.5} />
                                        </button>
                                    </span>
                                ) : (
                                    "GST%"
                                )}
                            </th>
                            <th scope="col" style={{ minWidth: 220 }}>
                                Extra
                            </th>
                            <th scope="col" style={{ minWidth: 100 }}>
                                Amount
                            </th>
                            <th scope="col" style={{ minWidth: 100 }}>
                                Total
                            </th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row, index) => {
                            // Billed AND the job is still frozen. Unlocking the job opens
                            // these; a job with no invoice behind it any more is just a job.
                            const locked = !!row.entry_id && rowsFrozen;
                            return (
                                <tr key={row._id || index} style={locked ? { opacity: 0.6 } : undefined}>
                                    <td>
                                        {locked ? (
                                            <span title="Already converted to an Entry - locked" style={{ color: "var(--text-tertiary)" }}>
                                                <Lock size={14} />
                                            </span>
                                        ) : (
                                            <button
                                                type="button"
                                                className="shell-icon-btn"
                                                aria-label="Remove row"
                                                disabled={rows.length <= 1}
                                                onClick={() => removeRow(index)}
                                                style={{ color: "var(--xan-rose)" }}
                                            >
                                                <Trash2 size={14} />
                                            </button>
                                        )}
                                    </td>
                                    <td>
                                        {locked ? (
                                            row.material || "—"
                                        ) : (
                                            <AssigneeDropdown
                                                value={row.material}
                                                placeholder="Select material"
                                                options={materialOptions}
                                                onSelect={(id, name) => onMaterialSelect(index, id, name)}
                                                onCreateNew={(query) => setQuickCreateProduct({ query, rowIndex: index })}
                                            />
                                        )}
                                    </td>
                                    <td>
                                        {locked ? (
                                            row.description || "—"
                                        ) : (
                                            <Input
                                                className="cell-input"
                                                value={row.description}
                                                placeholder="Description"
                                                onChange={(e) => updateRow(index, { description: e.target.value })}
                                            />
                                        )}
                                    </td>
                                    <RowSizeCells
                                        row={row}
                                        disabled={locked}
                                        needsLength={rowNeedsField(row, "length")}
                                        needsWidth={rowNeedsField(row, "width")}
                                        onChange={(patch) => updateRow(index, patch)}
                                    />
                                    <td>
                                        <Input
                                            className={`cell-input${rowNeedsField(row, "qty") ? " is-required-missing" : ""}`}
                                            type="number"
                                            step="1"
                                            value={row.qty}
                                            disabled={locked}
                                            onChange={(e) => updateRow(index, { qty: e.target.value })}
                                        />
                                    </td>
                                    <td>
                                        <Input
                                            className={`cell-input${rowNeedsField(row, "rate") ? " is-required-missing" : ""}`}
                                            type="number"
                                            step="1"
                                            value={row.rate}
                                            disabled={locked}
                                            onChange={(e) => updateRow(index, { rate: e.target.value })}
                                        />
                                    </td>
                                    {/* The one tax cell, on whichever side the job is. */}
                                    <td>
                                        {igstJob ? (
                                            <Input
                                                className="cell-input"
                                                type="number"
                                                step="1"
                                                value={row.igst}
                                                disabled={locked}
                                                // Editing one row's IGST still converts the
                                                // whole job - a sale has one place of supply.
                                                onChange={(e) => onIgstChange(index, e.target.value)}
                                            />
                                        ) : (
                                            <Input
                                                className="cell-input"
                                                type="number"
                                                step="1"
                                                // The column is "GST%", so it shows the FULL rate (18), not the stored half (9). cgst and
                                                // sgst each hold half of it - an intrastate split - so typing 18
                                                // writes 9 to each. Previously this rendered row.cgst directly,
                                                // so an 18% product displayed as 9%.
                                                value={gstPercent(row)}
                                                disabled={locked}
                                                // There's no separate SGST input in this form - the single
                                                // "CGST%" field is what a user thinks of as "the GST rate", so
                                                // typing here has to mirror into sgst too (same convention as
                                                // ClientComponents.js's Add Entry form), or a manual edit here
                                                // would silently leave sgst stuck at whatever it was before.
                                                onChange={(e) => updateRow(index, splitGst(e.target.value))}
                                            />
                                        )}
                                    </td>
                                    <td>
                                        <div style={{ display: "flex", flexWrap: "wrap", gap: 5 }}>
                                            {/* Only offered while the job is on GST. Pressing it
                                                promotes IGST out of Extra into its own column,
                                                carrying the product's tax with it - so it is a
                                                one-way door here, and the column's X is the way
                                                back. Leaving it in Extra as well would be two
                                                controls for one regime. */}
                                            {!igstJob && !locked && (
                                                <button
                                                    type="button"
                                                    className="igst-promote"
                                                    onClick={openIgstColumn}
                                                    title="Interstate sale - moves this job's tax to IGST"
                                                >
                                                    <Plus size={10} />
                                                    IGST%
                                                </button>
                                            )}
                                            <OptionalRowField
                                                label="Discount"
                                                value={row.discount}
                                                disabled={locked}
                                                onChange={(v) => updateRow(index, { discount: v })}
                                            />
                                            <OptionalRowField
                                                label="Charges"
                                                value={row.charges}
                                                disabled={locked}
                                                onChange={(v) => updateRow(index, { charges: v })}
                                            />
                                        </div>
                                    </td>
                                    <td className="cell-mono">₹{RoundOff(rowAmount(row))}</td>
                                    <td className="cell-mono">₹{RoundOff(rowTotal(row))}</td>
                                </tr>
                            );
                        })}
                    </tbody>
                </DataTable>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 12 }}>
                    <AddRowButton onAdd={addRow} />
                    <div className="text-body-medium">
                        Grand Total: <strong>₹{RoundOff(jobGrandTotal(rows))}</strong>
                    </div>
                </div>

                <p className="text-body-small" style={{ color: "var(--text-tertiary)", marginTop: 8 }}>
                    {/* No longer "rows converted to an Entry are locked" - that stopped being
                        true, and stopped being the user's vocabulary. What locks a job now is
                        an invoice, and the bar above says so when there is one. */}
                    Progress starts as Unassigned - assign an employee from the Lifecycle board afterward.
                    {rowsFrozen ? " Values are locked while this job-id is billed on an invoice." : ""}
                </p>

                {submitError && (
                    <div className="text-body-small" style={{ color: "var(--status-red-text)" }}>
                        {submitError}
                    </div>
                )}
            </ModalBody>
            <ModalFooter style={{ justifyContent: "space-between" }}>
                <MissingFieldsHint missing={[
                        !header.clientId && "Client",
                        !header.challanNumber.trim() && "Job Number",
                        rows.length === 0 && "at least one row",
                        ...missingRowFields(rows),
                    ].filter(Boolean)} />
                <span style={{ display: "flex", gap: 10 }}>
                <Button className="shell-btn shell-btn-secondary" onClick={toggle}>
                    Cancel
                </Button>
                <Button className="shell-btn shell-btn-primary" onClick={onSubmit} disabled={!canSubmit}>
                    {editingJob ? "Save Changes" : "Create Job"}
                </Button>
                </span>
            </ModalFooter>
            <QuickCreateProductModal
                isOpen={quickCreateProduct !== null}
                toggle={() => setQuickCreateProduct(null)}
                initialName={quickCreateProduct ? quickCreateProduct.query : ""}
                onCreated={(material) => {
                    setMaterials((prev) => [...prev, material]);
                    if (quickCreateProduct) {
                        // Select it into the row that opened the dialog. onMaterialSelect
                        // reads from `materials`, which has not re-rendered yet, so apply the
                        // same patch directly from the created document.
                        // Through taxFromMaterial, like onMaterialSelect. This used to call
                        // splitGst directly and set cgst/sgst only - which ignored the regime
                        // outright and left igst untouched, so creating a product inline on an
                        // IGST job wrote the tax to the wrong side AND left the old one
                        // standing. The comment above it claimed the two were kept in step;
                        // they were not, which is what a second copy of a rule does.
                        updateRow(quickCreateProduct.rowIndex, {
                            material: material.material_name || "",
                            rate: Number(material.material_rate) || 0,
                            ...taxFromMaterial(material, igstJob),
                        });
                    }
                    setQuickCreateProduct(null);
                }}
            />

            <QuickCreateClientModal
                isOpen={quickCreateClient !== null}
                toggle={() => setQuickCreateClient(null)}
                initialName={quickCreateClient}
                onCreated={onClientCreated}
            />
        </Modal>
    );
};

export default CreateJobModal;
