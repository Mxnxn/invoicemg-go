import React, { useEffect, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, FormGroup, Input } from "reactstrap";

import { unitBackend } from "./unit_backend";

// Create or edit one unit. `unit` null means create.
//
// A dialog rather than the inline form that used to sit above the units table, for the same
// reason the product and customer forms became dialogs: a form that is always on screen is
// paying rent for the few seconds a month it is used, and the table is what the page is for.
const UnitFormModal = ({ isOpen, toggle, unit, onCreated, onUpdated }) => {
    const [name, setName] = useState("");
    const [error, setError] = useState("");
    const [saving, setSaving] = useState(false);

    const isEditing = !!unit;

    useEffect(() => {
        if (!isOpen) return;
        setName(unit ? unit.name : "");
        setError("");
    }, [isOpen, unit]);

    const onSubmit = async () => {
        if (!name.trim()) {
            // Was a bare return, so pressing Save on an empty field did nothing at all and
            // looked like the button was broken.
            setError("Unit name is required.");
            return;
        }
        setSaving(true);
        try {
            const formData = new FormData();
            formData.set("name", name.trim());
            if (isEditing) {
                formData.set("unit_id", unit._id);
                const res = await unitBackend.update(formData);
                if (onUpdated) onUpdated(res.data);
            } else {
                const res = await unitBackend.create(formData);
                if (onCreated) onCreated(res.data);
            }
            toggle();
        } catch (err) {
            // errorInterceptor already raises the toast; a duplicate name is the likely one.
        } finally {
            setSaving(false);
        }
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} centered>
            <ModalHeader toggle={toggle}>
                <span className="confirm-modal-title">{isEditing ? "Edit unit" : "New unit"}</span>
            </ModalHeader>
            <ModalBody>
                <FormGroup className="mb-0">
                    <label className="form-control-label pp fs-12" htmlFor="unit-name">
                        Unit name<span className="required-star">*</span>
                    </label>
                    <Input
                        id="unit-name"
                        className={`form-control-alternative nn${error ? " is-required-missing" : ""}`}
                        value={name}
                        onChange={(e) => {
                            setName(e.target.value);
                            if (error) setError("");
                        }}
                        placeholder="e.g. SQ. Feet, PCs, NOS"
                        type="text"
                    />
                    {error && <span className="field-error">{error}</span>}
                </FormGroup>
            </ModalBody>
            <ModalFooter>
                <button type="button" className="shell-btn shell-btn-secondary" onClick={toggle} disabled={saving}>
                    Cancel
                </button>
                <button type="button" className="shell-btn shell-btn-primary" onClick={onSubmit} disabled={saving || !name.trim()}>
                    {saving ? "Saving…" : isEditing ? "Save changes" : "Add unit"}
                </button>
            </ModalFooter>
        </Modal>
    );
};

export default UnitFormModal;
