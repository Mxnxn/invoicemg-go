import React, { useEffect, useState } from "react";
import useRequiredFields from "../Common/useRequiredFields";
import { Modal, ModalHeader, ModalBody, ModalFooter, Button, FormGroup, Input, Row, Col } from "reactstrap";

const EMPTY = { name: "", firm: "", gst: "", phone: "", address: "", account_no: "", ifsc: "", bank_name: "" };

// Create or edit one company profile. `company` null means create.
const CompanyFormModal = ({ isOpen, toggle, company, onSubmit }) => {
    const [form, setForm] = useState(EMPTY);
    const [error, setError] = useState("");

    // Only `name` was ever enforced here (the old inline check below), so that stays the
    // required set - the asterisk marks what the form actually rejects, nothing more.
    const required = useRequiredFields(form, { name: "Company name", phone: "Phone number" });

    useEffect(() => {
        if (!isOpen) return;
        setError("");
        setForm(
            company
                ? {
                      name: company.name || "",
                      firm: company.firm || "",
                      gst: company.gst || "",
                      phone: company.phone || "",
                      address: company.address || "",
                      account_no: company.account_no || "",
                      ifsc: company.ifsc || "",
                      bank_name: company.bank_name || "",
                  }
                : EMPTY
        );
    }, [isOpen, company]);

    const onChange = (e) => setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }));

    const submit = () => {
        if (!required.isComplete) {
            required.showAll();
            return;
        }
        const formData = new FormData();
        if (company) formData.set("company_id", company._id);
        Object.entries(form).forEach(([key, value]) => formData.set(key, value));
        onSubmit(formData)
            .then(() => toggle())
            .catch((err) => setError(err.message || "Couldn't save the company."));
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} size="lg">
            <ModalHeader toggle={toggle}>{company ? "Edit Company" : "Add Company"}</ModalHeader>
            <ModalBody>
                <Row>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">
                                Company Name
                                {required.errors.name !== undefined && <span className="required-star">*</span>}
                            </label>
                            <Input
                                className={required.errorFor("name") ? "is-required-missing" : undefined}
                                name="name"
                                value={form.name}
                                onChange={onChange}
                                onBlur={() => required.markTouched("name")}
                                placeholder="Shown in the switcher"
                            />
                            {required.errorFor("name") && <span className="field-error">{required.errorFor("name")}</span>}
                        </FormGroup>
                    </Col>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Firm</label>
                            <Input name="firm" value={form.firm} onChange={onChange} placeholder="Printed on invoices" />
                        </FormGroup>
                    </Col>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">GST</label>
                            <Input className="input-identifier" name="gst" value={form.gst} onChange={onChange} />
                        </FormGroup>
                    </Col>
                    <Col md="6">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">
                                Phone
                                {required.errors.phone !== undefined && <span className="required-star">*</span>}
                            </label>
                            <Input
                                className={required.errorFor("phone") ? "is-required-missing" : undefined}
                                name="phone"
                                value={form.phone}
                                onChange={onChange}
                                onBlur={() => required.markTouched("phone")}
                            />
                            {required.errorFor("phone") && <span className="field-error">{required.errorFor("phone")}</span>}
                        </FormGroup>
                    </Col>
                    <Col md="12">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Address</label>
                            <Input name="address" value={form.address} onChange={onChange} />
                        </FormGroup>
                    </Col>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Account No.</label>
                            <Input name="account_no" value={form.account_no} onChange={onChange} />
                        </FormGroup>
                    </Col>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">IFSC</label>
                            <Input name="ifsc" value={form.ifsc} onChange={onChange} />
                        </FormGroup>
                    </Col>
                    <Col md="4">
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Bank Name</label>
                            <Input name="bank_name" value={form.bank_name} onChange={onChange} />
                        </FormGroup>
                    </Col>
                </Row>
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
                <Button className="shell-btn shell-btn-primary" onClick={submit} disabled={!required.isComplete}>
                    {company ? "Save" : "Create Company"}
                </Button>
            </ModalFooter>
        </Modal>
    );
};

export default CompanyFormModal;
