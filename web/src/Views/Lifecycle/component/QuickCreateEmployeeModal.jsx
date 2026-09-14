import React, { useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter } from "reactstrap";

import PasswordInput from "../../../Common/PasswordInput";
import { personBackend } from "../../../Common/person_backend";
import { notifySuccess } from "../../../global/toast";

// Adding someone to the shop, from the board they are needed on.
//
// Name, email, password - the three an employee needs in order to be a person who can sign in.
// Everything else about them (phone, firm, the rest of the permission matrix) belongs on the
// People screen; asking for it here would turn a one-line interruption into a form.
//
// The two job permissions are granted without asking. Somebody created from a job board is
// being created in order to work on jobs, and an employee with no permissions signs in to an
// empty app - which looks like a broken account rather than a deliberate one. Anything beyond
// jobs is a decision, and decisions belong on the People screen.
const JOB_PERMISSIONS = ["lifecycle:view", "lifecycle:create"];

export default function QuickCreateEmployeeModal({ initialName = "", isOpen, toggle, onCreated }) {
    const [name, setName] = useState(initialName);
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");

    const submit = async (e) => {
        e.preventDefault();
        setError("");
        if (!name.trim()) return setError("Enter their name.");
        // Email and password travel together or not at all: the API only sets a password when
        // it has both, so a password typed beside an empty email would be silently discarded
        // and the account would exist with no way in.
        if (password && !email.trim()) return setError("A password needs an email address to sign in with.");
        if (email.trim() && !password) return setError("Set a password so they can sign in, or clear the email.");

        setBusy(true);
        try {
            const formData = new FormData();
            formData.set("name", name.trim());
            formData.set("type", "Employee");
            if (email.trim()) formData.set("email", email.trim());
            if (password) formData.set("password", password);
            formData.set("permissions", JSON.stringify(JOB_PERMISSIONS));
            const res = await personBackend.create(formData);
            notifySuccess(`${name.trim()} can now be assigned to jobs.`);
            onCreated?.(res.data);
            toggle();
        } catch (err) {
            setError(err?.message || "Couldn't create that employee.");
        } finally {
            setBusy(false);
        }
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} centered>
            <ModalHeader toggle={toggle}>New employee</ModalHeader>
            <form onSubmit={submit} autoComplete="off">
                <ModalBody>
                    <div style={{ marginBottom: 14 }}>
                        <label className="form-control-label pp fs-12" htmlFor="qce-name">
                            Name
                        </label>
                        <input
                            id="qce-name"
                            className="form-control"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            autoFocus
                        />
                    </div>

                    <div style={{ marginBottom: 14 }}>
                        <label className="form-control-label pp fs-12" htmlFor="qce-email">
                            Email
                        </label>
                        <input
                            id="qce-email"
                            className="form-control"
                            type="email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            placeholder="what they sign in with"
                            autoComplete="off"
                        />
                    </div>

                    <div style={{ marginBottom: 10 }}>
                        <label className="form-control-label pp fs-12" htmlFor="qce-password">
                            Password
                        </label>
                        <PasswordInput
                            id="qce-password"
                            inputClassName="form-control"
                            autoComplete="new-password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                        />
                    </div>

                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        They can see and raise job-ids straight away. Anything more is set under People.
                    </span>

                    {error && (
                        <div className="text-body-small" style={{ color: "var(--text-danger)", marginTop: 8 }}>
                            {error}
                        </div>
                    )}
                </ModalBody>
                <ModalFooter>
                    <button type="button" className="shell-btn shell-btn-secondary" onClick={toggle} disabled={busy}>
                        Cancel
                    </button>
                    <button type="submit" className="shell-btn shell-btn-primary" disabled={busy}>
                        {busy ? "Creating…" : "Create employee"}
                    </button>
                </ModalFooter>
            </form>
        </Modal>
    );
}
