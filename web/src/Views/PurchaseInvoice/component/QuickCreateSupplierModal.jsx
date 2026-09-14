import React, { useEffect, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup, Input, Row, Col } from "reactstrap";
import { personBackend } from "../../../Common/person_backend";
import useRequiredFields from "../../../Common/useRequiredFields";
import OpeningBalanceField from "../../../Common/OpeningBalanceField";
import { normalizePhone, phoneRule } from "../../../Common/phone";

const initialForm = { name: "", firm: "", phone: "", email: "", gst: "", address: "", openingBalance: 0 };

// The one supplier dialog: "create a supplier without leaving what you're doing" from the
// Supplier picker's "+ Create new" row in CreatePurchaseInvoiceModal, AND the create/edit form
// for Configure > Suppliers, which has no inline form of its own any more.
//
// It carries email and opening balance because the manager's form did. Two forms for one
// record is how they drift - the product dialog had been missing a required field for exactly
// that reason.
//
// `supplier` null (or absent) means create; a supplier means edit.
const QuickCreateSupplierModal = ({ isOpen, toggle, initialName, supplier, onCreated, onUpdated }) => {
    const [form, setForm] = useState(initialForm);
    const [error, setError] = useState("");
    const [saving, setSaving] = useState(false);

    const isEditing = !!supplier;

    useEffect(() => {
        if (!isOpen) return;
        if (supplier) {
            setForm({
                name: supplier.name || "",
                firm: supplier.firm || "",
                phone: supplier.phone || "",
                email: supplier.email || "",
                gst: supplier.gst || "",
                address: supplier.address || "",
                openingBalance: Number(supplier.openingBalance) || 0,
            });
        } else {
            setForm({ ...initialForm, name: initialName || "" });
        }
        setError("");
    }, [isOpen, initialName, supplier]);

    const onChange = (e) => setForm({ ...form, [e.target.name]: e.target.value });

    // Phone was not required, so suppliers could be created with no way to chase a purchase
    // order - the one thing the record exists for.
    const required = useRequiredFields(form, {
        name: "Supplier name",
        phone: { label: "Phone", validate: phoneRule },
    });

    const onSubmit = async () => {
        if (!required.isComplete) {
            required.showAll();
            return;
        }
        setSaving(true);
        setError("");
        try {
            const formData = new FormData();
            formData.set("name", form.name.trim());
            formData.set("type", "Supplier");
            formData.set("firm", form.firm.trim());
            formData.set("phone", normalizePhone(form.phone));
            formData.set("email", form.email.trim());
            formData.set("gst", form.gst.trim());
            formData.set("address", form.address.trim());
            formData.set("openingBalance", String(form.openingBalance || 0));

            if (isEditing) {
                formData.set("person_id", supplier._id);
                const res = await personBackend.update(formData);
                if (onUpdated) onUpdated(res.data);
            } else {
                const res = await personBackend.create(formData);
                if (onCreated) onCreated(res.data);
            }
            toggle();
        } catch (err) {
            setError(err.message || `Couldn't ${isEditing ? "save" : "create"} the supplier.`);
        } finally {
            setSaving(false);
        }
    };

    const field = (name, label, props = {}) => (
        <FormGroup>
            <label className="form-control-label pp fs-12" htmlFor={`supplier-${name}`}>
                {label}
                {required.errors[name] !== undefined && <span className="required-star">*</span>}
            </label>
            <Input
                id={`supplier-${name}`}
                className={required.errorFor(name) ? "is-required-missing" : undefined}
                name={name}
                value={form[name]}
                onChange={onChange}
                onBlur={() => required.markTouched(name)}
                {...props}
            />
            {required.errorFor(name) && <span className="field-error">{required.errorFor(name)}</span>}
        </FormGroup>
    );

    return (
        <Modal isOpen={isOpen} toggle={toggle} centered>
            <ModalHeader toggle={toggle}>
                <span className="confirm-modal-title">{isEditing ? "Edit supplier" : "New supplier"}</span>
            </ModalHeader>
            <ModalBody>
                <Row>
                    <Col md="6">{field("name", "Name", { placeholder: "Contact name" })}</Col>
                    <Col md="6">{field("firm", "Firm", { placeholder: "Business name" })}</Col>
                </Row>
                <Row>
                    <Col md="6">{field("phone", "Phone", { placeholder: "10-digit number" })}</Col>
                    <Col md="6">{field("email", "Email", { type: "email", placeholder: "name@example.com" })}</Col>
                </Row>
                <Row>
                    <Col md="6">{field("gst", "GST", { className: "input-identifier", placeholder: "15-character GSTIN" })}</Col>
                    <Col md="6">{field("address", "Address", { placeholder: "Address" })}</Col>
                </Row>
                <Row>
                    <Col md="12">
                        <FormGroup className="mb-0">
                            <OpeningBalanceField
                                side="supplier"
                                value={form.openingBalance}
                                onChange={(v) => setForm((f) => ({ ...f, openingBalance: v }))}
                            />
                        </FormGroup>
                    </Col>
                </Row>
                {error && (
                    <div className="text-body-small" style={{ color: "var(--status-red-text)", marginTop: 10 }}>
                        {error}
                    </div>
                )}
            </ModalBody>
            <ModalFooter>
                <button type="button" className="shell-btn shell-btn-secondary" onClick={toggle} disabled={saving}>
                    Cancel
                </button>
                <button
                    type="button"
                    className="shell-btn shell-btn-primary"
                    onClick={onSubmit}
                    disabled={saving || !required.isComplete}
                >
                    {saving ? "Saving…" : isEditing ? "Save changes" : "Create Supplier"}
                </button>
            </ModalFooter>
        </Modal>
    );
};

export default QuickCreateSupplierModal;
