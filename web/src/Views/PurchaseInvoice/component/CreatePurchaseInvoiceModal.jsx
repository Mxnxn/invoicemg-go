import React, { useEffect, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup, Input, Button, Row, Col } from "reactstrap";
import { Plus, Trash2 } from "react-feather";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import OptionalRowField from "../../../Common/DataTable/OptionalRowField";
import DataTable from "../../../Common/DataTable/DataTable";
import { personBackend } from "../../../Common/person_backend";
import { materialsBackend } from "../../Material/material_backend";
import { unitBackend } from "../../../Common/unit_backend";
import { rowAmount, rowTotal, purchaseInvoiceGrandTotal, emptyPurchaseInvoiceRow } from "../purchaseInvoiceMath";
import AddRowButton from "../../../Common/DataTable/AddRowButton";
import RowSizeCells, { QTY_COL_PX, SIZE_COL_PX } from "../../../Common/DataTable/RowSizeCells";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import QuickCreateSupplierModal from "./QuickCreateSupplierModal";
import MissingFieldsHint from "../../../Common/MissingFieldsHint";
import DateField from "../../../Common/DateField";

const initialHeader = {
    supplierId: "",
    supplierName: "",
    date: "",
    invoiceNumber: "",
};

const today = () => new Date().toISOString().slice(0, 10);

// A purchase invoice records what a supplier billed us for products bought - same row-table
// shape as CreateJobModal, but rows are qty*rate (no length/width) and HSN/GST are carried
// from the selected product and locked (can't be hand-edited once a product is chosen).
const CreatePurchaseInvoiceModal = ({ isOpen, toggle, onCreate, onUpdate, editingInvoice }) => {
    const [header, setHeader] = useState(initialHeader);
    const [rows, setRows] = useState([emptyPurchaseInvoiceRow()]);
    const [submitError, setSubmitError] = useState("");
    const [suppliers, setSuppliers] = useState([]);
    const [materials, setMaterials] = useState([]);
    const [units, setUnits] = useState([]);
    const [quickCreateSupplier, setQuickCreateSupplier] = useState(null);

    useEffect(() => {
        if (!isOpen) return;
        personBackend.list("Supplier").then((res) => setSuppliers(res.data));
        materialsBackend.getAllMaterials(new FormData(), window.localStorage.getItem("session_token")).then((res) => setMaterials(res.data));
        unitBackend.list().then((res) => setUnits(res.data));

        if (editingInvoice) {
            setHeader({
                supplierId: editingInvoice.supplier_id?._id || "",
                supplierName: editingInvoice.supplier_id?.firm || editingInvoice.supplier_id?.name || "",
                date: editingInvoice.date || today(),
                invoiceNumber: editingInvoice.invoiceNumber || "",
            });
            setRows(
                editingInvoice.rows && editingInvoice.rows.length
                    ? editingInvoice.rows.map((row) => ({
                          _id: row._id,
                          description: row.description || "",
                          material: row.material || "",
                          hsn: row.hsn || "",
                          gst: row.gst ?? 0,
                          hasDimensions: row.hasDimensions === true,
                          length: row.length || "1",
                          width: row.width || "1",
                          rate: row.rate ?? 0,
                          qty: row.qty ?? 1,
                          unit: row.unit || "",
                          discount: row.discount ?? 0,
                          charges: row.charges ?? 0,
                      }))
                    : [emptyPurchaseInvoiceRow()]
            );
            return;
        }
        setHeader({ ...initialHeader, date: today() });
        setRows([emptyPurchaseInvoiceRow()]);
    }, [isOpen, editingInvoice]);

    const supplierOptions = suppliers.map((s) => ({
        id: s._id,
        name: s.firm || s.name,
        search: [s.firm, s.name, s.phone].filter(Boolean).join(" ").toLowerCase(),
    }));
    const materialOptions = materials.map((m) => ({ id: m._id, name: m.material_name }));
    const unitOptions = units.map((u) => u.name);

    const onHeaderChange = (e) => setHeader({ ...header, [e.target.name]: e.target.value });

    const onSupplierSelect = (id, name) => setHeader({ ...header, supplierId: id || "", supplierName: name || "" });

    const onSupplierCreated = (supplier) => {
        setSuppliers((prev) => [supplier, ...prev]);
        setHeader((h) => ({ ...h, supplierId: supplier._id, supplierName: supplier.firm || supplier.name }));
    };

    const updateRow = (index, patch) => {
        setRows((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
    };

    const onMaterialSelect = (index, id, name) => {
        const material = materials.find((m) => m._id === id);
        updateRow(index, {
            material: name || "",
            hsn: material ? material.hsn || "" : rows[index].hsn,
            gst: material ? Number(material.tax) || 0 : rows[index].gst,
            rate: material ? Number(material.purchase_rate) || 0 : rows[index].rate,
        });
    };

    const addRow = (withDimensions) => setRows((prev) => [...prev, emptyPurchaseInvoiceRow(withDimensions)]);
    const removeRow = (index) => setRows((prev) => prev.filter((_, i) => i !== index));

    const canSubmit = header.supplierId && header.date && header.invoiceNumber.trim() && rows.length > 0;

    const onSubmit = () => {
        if (!canSubmit) return;
        const formData = new FormData();
        if (editingInvoice) formData.set("purchase_invoice_id", editingInvoice._id);
        formData.set("supplier_id", header.supplierId);
        formData.set("date", header.date);
        formData.set("invoiceNumber", header.invoiceNumber.trim());
        formData.set("rows", JSON.stringify(rows));
        setSubmitError("");
        const action = editingInvoice ? onUpdate(formData) : onCreate(formData);
        action
            .then(() => {
                setHeader(initialHeader);
                setRows([emptyPurchaseInvoiceRow()]);
                toggle();
            })
            .catch((err) => setSubmitError(err.message || "Failed to save purchase invoice."));
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} size="xl" style={{ maxWidth: "95vw" }}>
            <ModalHeader toggle={toggle}>{editingInvoice ? "Edit Purchase Invoice" : "Create Purchase Invoice"}</ModalHeader>
            <ModalBody>
                <Row>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                                Supplier<span className="required-star">*</span>
                            </label>
                            <AssigneeDropdown
                                value={header.supplierName}
                                placeholder="Select supplier"
                                options={supplierOptions}
                                onSelect={onSupplierSelect}
                                fullWidth
                                menuWidth={320}
                                onCreateNew={(query) => setQuickCreateSupplier(query)}
                            />
                        </FormGroup>
                    </Col>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Date<span className="required-star">*</span></label>
                            <DateField name="date" value={header.date} onChange={onHeaderChange} />
                        </FormGroup>
                    </Col>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Invoice Number<span className="required-star">*</span></label>
                            <Input
                                name="invoiceNumber"
                                placeholder="Supplier's invoice number"
                                value={header.invoiceNumber}
                                onChange={onHeaderChange}
                            />
                        </FormGroup>
                    </Col>
                </Row>

                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col" style={{ width: 32 }} />
                            <th scope="col" style={{ minWidth: 160 }}>
                                Description
                            </th>
                            <th scope="col" style={{ minWidth: 160 }}>
                                Product
                            </th>
                            <th scope="col" style={{ minWidth: SIZE_COL_PX, width: SIZE_COL_PX }}>
                                Length
                            </th>
                            <th scope="col" style={{ minWidth: SIZE_COL_PX, width: SIZE_COL_PX }}>
                                Width
                            </th>
                            <th scope="col" style={{ minWidth: QTY_COL_PX, width: QTY_COL_PX }}>
                                Qty
                            </th>
                            <th scope="col" style={{ minWidth: 90 }}>
                                HSN
                            </th>
                            <th scope="col" style={{ minWidth: QTY_COL_PX, width: QTY_COL_PX }}>
                                GST%
                            </th>
                            <th scope="col" style={{ minWidth: QTY_COL_PX, width: QTY_COL_PX }}>
                                Purchase Rate
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
                        {rows.map((row, index) => (
                            <tr key={row._id || index}>
                                <td>
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
                                </td>
                                <td>
                                    <Input
                                        className="cell-input"
                                        value={row.description}
                                        placeholder="Description"
                                        onChange={(e) => updateRow(index, { description: e.target.value })}
                                    />
                                </td>
                                <td>
                                    <AssigneeDropdown
                                        value={row.material}
                                        placeholder="Select product"
                                        options={materialOptions}
                                        onSelect={(id, name) => onMaterialSelect(index, id, name)}
                                    />
                                </td>
                                {/* A purchase line is by-quantity unless switched, so "+ Size" is
                                    the usual state here rather than the exception. */}
                                <RowSizeCells row={row} onChange={(patch) => updateRow(index, patch)} />
                                <td>
                                    <Input
                                        className="cell-input"
                                        type="number"
                                        step="1"
                                        value={row.qty}
                                        onChange={(e) => updateRow(index, { qty: e.target.value })}
                                    />
                                </td>
                                <td>
                                    <Input className="cell-input" value={row.hsn} disabled placeholder="HSN" />
                                </td>
                                <td>
                                    {/* Editable once a product is picked. The product's tax is
                                        what it usually is, not what this supplier actually
                                        charged: slabs change, and a supplier billing 12% on a
                                        line the catalogue calls 18% left no way to record the
                                        invoice as issued. Locked until a product is selected so
                                        there is something for the figure to be a correction to. */}
                                    <Input
                                        className="cell-input"
                                        type="number"
                                        step="0.25"
                                        min="0"
                                        max="100"
                                        value={row.gst}
                                        disabled={!row.material}
                                        title={row.material ? "GST % on this line" : "Pick a product first"}
                                        onChange={(e) => updateRow(index, { gst: e.target.value })}
                                    />
                                </td>
                                <td>
                                    <Input
                                        className="cell-input"
                                        type="number"
                                        step="1"
                                        value={row.rate}
                                        onChange={(e) => updateRow(index, { rate: e.target.value })}
                                    />
                                </td>
                                <td>
                                    <div style={{ display: "flex", flexWrap: "wrap", gap: 5 }}>
                                        <OptionalRowField
                                            label="Unit"
                                            type="select"
                                            options={unitOptions}
                                            value={row.unit}
                                            onChange={(v) => updateRow(index, { unit: v })}
                                        />
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
                        Grand Total: <strong>₹{RoundOff(purchaseInvoiceGrandTotal(rows))}</strong>
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
                        !header.supplierId && "Supplier",
                        !header.date && "Date",
                        !header.invoiceNumber.trim() && "Invoice Number",
                        rows.length === 0 && "at least one row",
                    ].filter(Boolean)} />
                <span style={{ display: "flex", gap: 10 }}>
                <Button className="shell-btn shell-btn-secondary" onClick={toggle}>
                    Cancel
                </Button>
                <Button className="shell-btn shell-btn-primary" onClick={onSubmit} disabled={!canSubmit}>
                    {editingInvoice ? "Save Changes" : "Create Purchase Invoice"}
                </Button>
                </span>
            </ModalFooter>
            <QuickCreateSupplierModal
                isOpen={quickCreateSupplier !== null}
                toggle={() => setQuickCreateSupplier(null)}
                initialName={quickCreateSupplier}
                onCreated={onSupplierCreated}
            />
        </Modal>
    );
};

export default CreatePurchaseInvoiceModal;
