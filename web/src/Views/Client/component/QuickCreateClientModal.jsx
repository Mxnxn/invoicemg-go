import React, { useEffect, useState } from "react";
import { Repeat } from "react-feather";
import useRequiredFields from "../../../Common/useRequiredFields";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup, Input, Button, Row, Col } from "reactstrap";
import { clientsBackend } from "../client_backend";
import OpeningBalanceField from "../../../Common/OpeningBalanceField";

const initialForm = { clientName: "", clientFirm: "", clientPhone: "", clientGST: "", clientAddress: "", openingBalance: 0, alsoSupplier: false };

// One customer form, in a dialog.
//
// Used three ways: the Client picker's "+ Create new" row in CreateJobModal and
// CreateQuotationModal, and both Create and Edit in the full Customers manager
// (Configure > Customers). Editing reuses this rather than a second form because two forms
// over one record drift - one grows a field, gains a validation rule, or starts trimming
// whitespace, and which one you got depended on where you opened it from.
//
// Pass `client` to edit; omit it to create. The only thing that differs between the two is
// the "they supply us too" toggle, which is create-only: editing a customer must never
// silently rewrite a separate Supplier record.
const QuickCreateClientModal = ({ isOpen, toggle, initialName, client, onCreated, onUpdated }) => {
    const editing = Boolean(client && client._id);
    const uid = window.localStorage.getItem("uid");
    const [form, setForm] = useState(initialForm);
    const [fieldErrors, setFieldErrors] = useState({});
    const [error, setError] = useState("");
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        if (!isOpen) return;
        setForm(
            editing
                ? {
                      ...initialForm,
                      clientName: client.clientName || "",
                      clientFirm: client.clientFirm || "",
                      clientPhone: client.clientPhone || "",
                      clientGST: client.clientGST || "",
                      clientAddress: client.clientAddress || "",
                      openingBalance: Number(client.openingBalance) || 0,
                  }
                : { ...initialForm, clientName: initialName || "" }
        );
        setFieldErrors({});
        setError("");
    }, [isOpen, initialName, editing, client]);

    const onChange = (e) => setForm({ ...form, [e.target.name]: e.target.value });

    // Was an inline five-way && - same required set, but routed through the shared hook so
    // the asterisk, the red border and the per-field message come with it.
    const required = useRequiredFields(form, {
        clientName: "Contact name",
        clientFirm: "Firm",
        clientPhone: "Phone",
    });
    const canSubmit = required.isComplete;

    const onSubmit = async () => {
        if (!canSubmit) return;
        setSaving(true);
        setError("");
        setFieldErrors({});
        try {
            const formData = new FormData();
            formData.set("uid", uid);
            formData.set("client_name", form.clientName.trim());
            formData.set("client_firm", form.clientFirm.trim());
            formData.set("client_phone", form.clientPhone.trim());
            formData.set("client_gst", form.clientGST.trim());
            formData.set("client_address", form.clientAddress.trim());
            // Same key the full form posts - routes/Client.js reads opening_balance.
            formData.set("opening_balance", String(form.openingBalance || 0));
            if (editing) {
                formData.set("client_id", client._id);
                const res = await clientsBackend.editClientDetail(formData);
                (onUpdated || onCreated)?.(res.data);
            } else {
                // Create only - see the note on the component. routes/Client.js creates the
                // Supplier alongside when this is set.
                if (form.alsoSupplier) formData.set("also_supplier", "true");
                const res = await clientsBackend.addNewClient(formData);
                onCreated?.(res.data);
            }
            toggle();
        } catch (err) {
            if (err.error) setFieldErrors(err.error);
            setError(err.message || `Couldn't ${editing ? "save" : "create"} the client.`);
        } finally {
            setSaving(false);
        }
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle}>
            <ModalHeader toggle={toggle}>{editing ? `Edit ${client.clientName || "client"}` : "New Client"}</ModalHeader>
            <ModalBody>
                <Row>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Name{required.errors.clientName !== undefined && <span className="required-star">*</span>}</label>
                            <Input
                            	className={required.errorFor("clientName") ? "is-required-missing" : undefined} name="clientName" value={form.clientName} onChange={onChange} placeholder="Contact name"
                            	onBlur={() => required.markTouched("clientName")}
                            />
                            {required.errorFor("clientName") && <span className="field-error">{required.errorFor("clientName")}</span>}
                        </FormGroup>
                    </Col>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Firm{required.errors.clientFirm !== undefined && <span className="required-star">*</span>}</label>
                            <Input
                            	className={required.errorFor("clientFirm") ? "is-required-missing" : undefined} name="clientFirm" value={form.clientFirm} onChange={onChange} placeholder="Business name"
                            	onBlur={() => required.markTouched("clientFirm")}
                            />
                            {required.errorFor("clientFirm") && <span className="field-error">{required.errorFor("clientFirm")}</span>}
                        </FormGroup>
                    </Col>
                </Row>
                <Row>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Phone{required.errors.clientPhone !== undefined && <span className="required-star">*</span>}</label>
                            <Input
                            	className={required.errorFor("clientPhone") ? "is-required-missing" : undefined} name="clientPhone" value={form.clientPhone} onChange={onChange} placeholder="10-digit number"
                            	onBlur={() => required.markTouched("clientPhone")}
                            />
                            {required.errorFor("clientPhone") && <span className="field-error">{required.errorFor("clientPhone")}</span>}
                            {fieldErrors.client_phone && (
                                <div className="text-body-small" style={{ color: "var(--status-red-text)", marginTop: 4 }}>
                                    {fieldErrors.client_phone === "duplicate" ? "This phone number is already registered." : fieldErrors.client_phone}
                                </div>
                            )}
                        </FormGroup>
                    </Col>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">GST{required.errors.clientGST !== undefined && <span className="required-star">*</span>}</label>
                            <Input
                            	className={["input-identifier", required.errorFor("clientGST") ? "is-required-missing" : ""].filter(Boolean).join(" ")} name="clientGST" value={form.clientGST} onChange={onChange} placeholder="15-character GSTIN"
                            	onBlur={() => required.markTouched("clientGST")}
                            />
                            {required.errorFor("clientGST") && <span className="field-error">{required.errorFor("clientGST")}</span>}
                            {fieldErrors.client_gst && (
                                <div className="text-body-small" style={{ color: "var(--status-red-text)", marginTop: 4 }}>
                                    {fieldErrors.client_gst === "duplicate" ? "This GST number is already registered." : fieldErrors.client_gst}
                                </div>
                            )}
                        </FormGroup>
                    </Col>
                </Row>
                <Row>
                    <Col md="12">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Address{required.errors.clientAddress !== undefined && <span className="required-star">*</span>}</label>
                            <Input
                            	className={required.errorFor("clientAddress") ? "is-required-missing" : undefined} name="clientAddress" value={form.clientAddress} onChange={onChange} placeholder="Address"
                            	onBlur={() => required.markTouched("clientAddress")}
                            />
                            {required.errorFor("clientAddress") && <span className="field-error">{required.errorFor("clientAddress")}</span>}
                        </FormGroup>
                    </Col>
                </Row>
                {/* Same field, same component and same sign convention as the full Customers
                    form (Views/Client/component/AddClient.js) - a customer created mid-job
                    carries the debt they walked in with, and having to reopen them in
                    Configure afterwards just to record it is how it gets forgotten. */}
                <Row>
                    <Col md="12">
                        <FormGroup>
                            <OpeningBalanceField
                                side="customer"
                                value={form.openingBalance}
                                onChange={(v) => setForm((f) => ({ ...f, openingBalance: v }))}
                            />
                        </FormGroup>
                    </Col>
                </Row>
                {/* Create-only, and not a plain checkbox: this creates a SECOND record, and a
                    card that says so is harder to tick by accident. Hidden when editing -
                    the two records are independent once both exist, and rewriting one from
                    the other would be surprising. */}
                {!editing && (
                <button
                    type="button"
                    className={["relation-toggle", form.alsoSupplier ? "is-on" : ""].filter(Boolean).join(" ")}
                    aria-pressed={form.alsoSupplier}
                    onClick={() => setForm((f) => ({ ...f, alsoSupplier: !f.alsoSupplier }))}
                >
                    <span className="relation-toggle-icon">
                        <Repeat size={16} />
                    </span>
                    <span className="relation-toggle-body">
                        <span className="relation-toggle-title">
                            They supply us too
                            <span className="relation-toggle-state">{form.alsoSupplier ? "On" : "Off"}</span>
                        </span>
                        <span className="relation-toggle-desc">
                            Also creates a supplier record, so you can raise purchase invoices against this firm. The two
                            ledgers stay separate — what they owe you and what you owe them are never combined.
                        </span>
                    </span>
                </button>
                )}
                {error && (
                    <div className="text-body-small" style={{ color: "var(--status-red-text)" }}>
                        {error}
                    </div>
                )}
            </ModalBody>
            <ModalFooter>
                <Button className="shell-btn shell-btn-secondary" onClick={toggle}>
                    Cancel
                </Button>
                <Button className="shell-btn shell-btn-primary" onClick={onSubmit} disabled={!canSubmit || saving}>
                    Create Client
                </Button>
            </ModalFooter>
        </Modal>
    );
};

export default QuickCreateClientModal;
