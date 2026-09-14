import React, { useEffect, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup, Input, Button, Row, Col } from "reactstrap";
import { Plus, Trash2, X } from "react-feather";
import AssigneeDropdown from "../Lifecycle/component/AssigneeDropdown";
import DataTable from "../../Common/DataTable/DataTable";
import { lifecycleBackend } from "../Lifecycle/lifecycle_backend";
import { quotationBackend } from "./quotation_backend";
import { rowAmount, rowTotal, quotationGrandTotal, emptyQuotationRow } from "./quotationMath";
import AddRowButton from "../../Common/DataTable/AddRowButton";
import RowSizeCells, { QTY_COL_PX, SIZE_COL_PX } from "../../Common/DataTable/RowSizeCells";
// A quotation row has the same shape and the same amount formula as a job row, so the
// conditional-required rule is shared rather than restated here.
import { rowNeedsField, missingRowFields } from "../Lifecycle/jobMath";
import { RoundOff } from "../../Common/DateAndTime/RoundOff";
import QuickCreateClientModal from "../Client/component/QuickCreateClientModal";
import OptionalRowField from "../../Common/DataTable/OptionalRowField";
import { splitGst } from "../../Common/gst";
// The interstate/intrastate rule is the job form's, reused rather than restated: a
// quotation row becomes a job row, and the two disagreeing about which side the tax
// sits on is how a quotation gets converted into a job that charges differently.
import { isIgstJob, totalTaxPercent, applyIgstEdit, toIgstRow, toGstRow, taxFromMaterial } from "../Lifecycle/jobTax";
import MissingFieldsHint from "../../Common/MissingFieldsHint";
import DateField from "../../Common/DateField";
// .igst-promote / .igst-col-close live with the job form they were written for.
import "../Lifecycle/lifecycle.css";

// Create-only: editing an existing quotation's rows happens row-by-row from the detail
// modal (per spec point 9), not by reopening this form.
// GST is stored split across cgst/sgst (9 + 9 for an 18% intrastate sale), or whole on igst
// for an interstate sale, but entered and read as one number either way - totalTaxPercent in
// jobTax.js is that number, and it is shared with the job form rather than restated here.

const CreateQuotationModal = ({ isOpen, toggle, onCreate, fixedClient }) => {
    const [clientId, setClientId] = useState(fixedClient?.id || "");
    const [clientName, setClientName] = useState(fixedClient?.name || "");
    const [date, setDate] = useState("");
    const [quotationNumber, setQuotationNumber] = useState("");
    const [rows, setRows] = useState([emptyQuotationRow()]);
    const [clients, setClients] = useState([]);
    const [materials, setMaterials] = useState([]);
    const [submitError, setSubmitError] = useState("");
    const [quickCreateClient, setQuickCreateClient] = useState(null);

    useEffect(() => {
        if (!isOpen) return;
        if (!fixedClient) lifecycleBackend.lookupClients().then((res) => setClients(res.data));
        setClientId(fixedClient?.id || "");
        setClientName(fixedClient?.name || "");
        setDate(new Date().toISOString().slice(0, 10));
        setRows([emptyQuotationRow()]);
        setIgstMode(false);
        setSubmitError("");
        quotationBackend.nextQuotationNumber().then((res) => setQuotationNumber(res.data.quotationNumber));
    }, [isOpen, fixedClient]);

    // Products are company-wide, not per-client - /lifecycle/lookups/materials ignores
    // client_id and filters on company_id alone. Gating on a selected client was a leftover
    // from when materials belonged to a client, and left the picker empty until one was
    // chosen. CreateJobModal already loads them unconditionally; this matches it.
    useEffect(() => {
        lifecycleBackend.lookupMaterials({}).then((res) => setMaterials(res.data));
    }, []);

    const clientOptions = clients.map((c) => ({
        id: c._id,
        name: c.clientFirm || c.clientName,
        search: [c.clientFirm, c.clientName, c.clientPhone].filter(Boolean).join(" ").toLowerCase(),
    }));
    const materialOptions = materials.map((m) => ({ id: m._id, name: m.material_name }));

    const onClientCreated = (client) => {
        setClients((prev) => [client, ...prev]);
        setClientId(client._id);
        setClientName(client.clientFirm || client.clientName);
    };

    // Which side of the tax this quotation is on. Read from the rows so it always matches
    // what they actually hold - and kept in state as well, because isIgstJob cannot see a
    // regime nobody has typed a rate for yet: a fresh quotation switched to IGST has igst 0
    // on every row and would read back as intrastate.
    const [igstMode, setIgstMode] = useState(false);
    const interstate = isIgstJob(rows) || igstMode;

    // Whole-quotation, both ways. The place of supply belongs to the sale, not to one line of
    // it, so a single row cannot be interstate while its neighbours are not.
    const openIgstColumn = () => {
        setIgstMode(true);
        setRows((prev) => prev.map(toIgstRow));
    };
    const closeIgstColumn = () => {
        setIgstMode(false);
        setRows((prev) => prev.map(toGstRow));
    };

    const updateRow = (index, patch) => {
        setRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
    };

    const onMaterialSelect = (index, id, name) => {
        const material = materials.find((m) => m._id === id);
        // Material.tax is one combined rate (e.g. 18). Where it lands depends on the sale:
        // halved across CGST/SGST intrastate, whole on IGST interstate. Dumping the whole
        // rate into cgst alone (leaving sgst at 0) still totals the same amount charged, but
        // misreports the GST filing split 2x too high on CGST and 0 on SGST once this
        // quotation's rows convert to a Job/Entry.
        //
        // taxFromMaterial takes the DECISION rather than the rows to re-derive it from - on a
        // quotation switched to IGST before any rate is typed, every row's igst is 0, so
        // re-deriving reads as intrastate and quietly halves the product's tax into CGST/SGST
        // while the IGST column the user is looking at stays empty.
        const split = material ? taxFromMaterial(material, interstate) : null;
        updateRow(index, {
            material: name || "",
            rate: material ? Number(material.material_rate) : rows[index].rate,
            cgst: split ? split.cgst : rows[index].cgst,
            sgst: split ? split.sgst : rows[index].sgst,
            igst: split ? split.igst : rows[index].igst,
        });
    };

    const addRow = (withDimensions) => setRows((prev) => [...prev, emptyQuotationRow(withDimensions)]);
    const removeRow = (index) => setRows((prev) => prev.filter((_, i) => i !== index));

    // Same rule as the job card (CreateJobModal): a row only counts once a material is
    // chosen, and from then on the four factors of the amount must be non-zero - the total
    // is qty x length x width x rate, so any blank silently zeroes the line.
    const anyRowNeeds = (field) => rows.some((row) => rowNeedsField(row, field));

    const canSubmit = clientId && date && rows.length > 0 && missingRowFields(rows).length === 0;

    const onSubmit = () => {
        if (!canSubmit) return;
        const formData = new FormData();
        formData.set("client_id", clientId);
        formData.set("date", date);
        formData.set("quotationNumber", quotationNumber);
        formData.set("rows", JSON.stringify(rows));
        setSubmitError("");
        onCreate(formData)
            .then(() => toggle())
            .catch((err) => setSubmitError(err.message || "Failed to create quotation."));
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} size="xl" style={{ maxWidth: "95vw" }}>
            <ModalHeader toggle={toggle}>Create Quotation</ModalHeader>
            <ModalBody>
                <Row>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                                Client<span className="required-star">*</span>
                            </label>
                            {fixedClient ? (
                                <Input value={clientName} disabled />
                            ) : (
                                <AssigneeDropdown
                                    value={clientName}
                                    placeholder="Select client"
                                    options={clientOptions}
                                    fullWidth
                                    variant="dashed"
                                    onSelect={(id, name) => {
                                        setClientId(id || "");
                                        setClientName(name || "");
                                    }}
                                    onCreateNew={(query) => setQuickCreateClient(query)}
                                />
                            )}
                        </FormGroup>
                    </Col>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Date<span className="required-star">*</span></label>
                            <DateField value={date} onChange={(e) => setDate(e.target.value)} />
                        </FormGroup>
                    </Col>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Quotation Number</label>
                            <Input value={quotationNumber} disabled />
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
                            {/* One tax column, showing whichever side this quotation is on. The
                                two can never both apply, so a second column would only ever be
                                dead width. Same treatment as the job form's. */}
                            <th
                                scope="col"
                                style={{ minWidth: QTY_COL_PX, width: QTY_COL_PX }}
                                title={interstate ? "Interstate sale - one IGST rate per row" : "Split evenly into CGST + SGST"}
                            >
                                {interstate ? (
                                    <span className="d-inline-flex align-items-center" style={{ gap: 6 }}>
                                        IGST%
                                        <button
                                            type="button"
                                            className="igst-col-close"
                                            aria-label="Switch back to GST"
                                            title="Back to GST - the rate moves with it"
                                            onClick={closeIgstColumn}
                                        >
                                            <X size={13} strokeWidth={2.5} />
                                        </button>
                                    </span>
                                ) : (
                                    "GST%"
                                )}
                            </th>
                            <th scope="col" style={{ minWidth: 160 }}>
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
                        {rows.map((row, index) => (
                            <tr key={index}>
                                <td>
                                    <button
                                        type="button"
                                        className="shell-icon-btn"
                                        aria-label="Remove row"
                                        disabled={index === 0}
                                        onClick={() => removeRow(index)}
                                        style={{ color: "var(--xan-rose)" }}
                                    >
                                        <Trash2 size={14} />
                                    </button>
                                </td>
                                <td>
                                    <AssigneeDropdown
                                        value={row.material}
                                        placeholder={clientId ? "Select material" : "Select a client first"}
                                        options={materialOptions}
                                        onSelect={(id, name) => onMaterialSelect(index, id, name)}
                                    />
                                </td>
                                <td>
                                    <Input
                                        className="cell-input"
                                        value={row.description}
                                        placeholder="Description"
                                        onChange={(e) => updateRow(index, { description: e.target.value })}
                                    />
                                </td>
                                <RowSizeCells
                                    row={row}
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
                                        onChange={(e) => updateRow(index, { qty: e.target.value })}
                                    />
                                </td>
                                <td>
                                    <Input
                                        className={`cell-input${rowNeedsField(row, "rate") ? " is-required-missing" : ""}`}
                                        type="number"
                                        step="1"
                                        value={row.rate}
                                        onChange={(e) => updateRow(index, { rate: e.target.value })}
                                    />
                                </td>
                                <td>
                                    <Input
                                        className="cell-input"
                                        type="number"
                                        step="1"
                                        // One number in, one number out, whichever side it is stored
                                        // on: the column shows the FULL rate (18), never the stored
                                        // half. Rendering row.cgst directly showed an 18% product as 9%.
                                        value={totalTaxPercent(row)}
                                        // There's no separate SGST input in this form - the single
                                        // field is what a user thinks of as "the GST rate", so typing
                                        // here has to mirror into sgst too (same convention as
                                        // ClientComponents.js's Add Entry form), or a manual edit here
                                        // would silently leave sgst stuck at whatever it was before.
                                        // On IGST, applyIgstEdit converts the whole table instead -
                                        // clearing the last rate is what brings CGST/SGST back.
                                        onChange={(e) =>
                                            interstate
                                                ? setRows((prev) => applyIgstEdit(prev, index, e.target.value))
                                                : updateRow(index, splitGst(e.target.value))
                                        }
                                    />
                                </td>
                                <td>
                                    <div style={{ display: "flex", flexWrap: "wrap", gap: 5 }}>
                                        {/* Offered only while the quotation is on GST: pressing it
                                            promotes IGST into its own column, carrying whatever rate
                                            is already there. The column's X is the way back, so
                                            there is one control for the regime, not two. */}
                                        {!interstate && (
                                            <button
                                                type="button"
                                                className="igst-promote"
                                                onClick={openIgstColumn}
                                                title="Interstate sale - moves this quotation's tax to IGST"
                                            >
                                                <Plus size={10} />
                                                IGST%
                                            </button>
                                        )}
                                        <OptionalRowField
                                            label="Discount"
                                            value={row.discount}
                                            onChange={(v) => updateRow(index, { discount: v })}
                                        />
                                        <OptionalRowField
                                            label="Charges"
                                            value={row.charges}
                                            onChange={(v) => updateRow(index, { charges: v })}
                                        />
                                    </div>
                                </td>
                                <td className="cell-mono">₹{RoundOff(rowAmount(row))}</td>
                                <td className="cell-mono">₹{RoundOff(rowTotal(row))}</td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 12 }}>
                    <AddRowButton onAdd={addRow} />
                    <div className="text-body-medium">
                        Grand Total: <strong>₹{RoundOff(quotationGrandTotal(rows))}</strong>
                    </div>
                </div>

                {submitError && (
                    <div className="text-body-small" style={{ color: "var(--status-red-text)", marginTop: 8 }}>
                        {submitError}
                    </div>
                )}
            </ModalBody>
            <ModalFooter style={{ justifyContent: "space-between" }}>
                <MissingFieldsHint missing={[
                        !clientId && "Client",
                        !date && "Date",
                        rows.length === 0 && "at least one row",
                        ...missingRowFields(rows),
                    ].filter(Boolean)} />
                <span style={{ display: "flex", gap: 10 }}>
                <Button className="shell-btn shell-btn-secondary" onClick={toggle}>
                    Cancel
                </Button>
                <Button className="shell-btn shell-btn-primary" onClick={onSubmit} disabled={!canSubmit}>
                    Create Quotation
                </Button>
                </span>
            </ModalFooter>
            <QuickCreateClientModal
                isOpen={quickCreateClient !== null}
                toggle={() => setQuickCreateClient(null)}
                initialName={quickCreateClient}
                onCreated={onClientCreated}
            />
        </Modal>
    );
};

export default CreateQuotationModal;
