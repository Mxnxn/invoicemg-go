import React, { useEffect, useMemo, useRef, useState } from "react";
import { Modal, ModalHeader, ModalBody, ModalFooter, Button, Input } from "reactstrap";
import { AlertTriangle } from "react-feather";

// "Type the name to delete it."
//
// A window.confirm() takes one click, reads the same whether it is about one spare product or
// a customer with four years of invoices behind them, and a mis-aimed Enter confirms it. These
// records are referenced by jobs, quotations and invoices; deleting the wrong one is not
// something an Undo bar can walk back, because the rows pointing at it are already gone.
//
// Typing the name is the cheapest control that scales with the consequence: it costs seconds
// on something you meant to do, and it is impossible to do by accident. It also forces you to
// read which record is actually selected, which is the failure this exists to catch - the
// wrong row, not the wrong button.
const ConfirmDeleteModal = ({
    isOpen,
    toggle,
    // What is being deleted: "customer", "product", "unit", "supplier".
    kind = "record",
    // The exact text that must be typed back.
    name = "",
    // Anything worth knowing before they commit - "4 invoices reference this", say.
    warning = "",
    onConfirm,
}) => {
    const [typed, setTyped] = useState("");
    const [busy, setBusy] = useState(false);
    const inputRef = useRef(null);

    useEffect(() => {
        if (!isOpen) return;
        setTyped("");
        setBusy(false);
    }, [isOpen, name]);

    // Case- and space-insensitive. The point is to prove you read the name, not to test
    // whether you can reproduce its capitalisation - and these names carry a titleCase setter
    // server-side, so what is stored may not be what the user remembers typing.
    const matches = useMemo(
        () => typed.trim().toLowerCase() === String(name || "").trim().toLowerCase() && Boolean(name),
        [typed, name]
    );

    const confirm = async () => {
        if (!matches || busy) return;
        setBusy(true);
        try {
            await onConfirm();
            toggle();
        } catch (error) {
            // The global interceptor raises the toast with the server's own reason.
            setBusy(false);
        }
    };

    return (
        <Modal isOpen={isOpen} toggle={toggle} centered onOpened={() => inputRef.current?.focus()}>
            <ModalHeader toggle={toggle}>Delete {kind}</ModalHeader>
            <ModalBody>
                <div className="confirm-delete-lead">
                    <AlertTriangle size={18} className="confirm-delete-icon" aria-hidden="true" />
                    <div>
                        <p className="confirm-delete-text">
                            This permanently deletes the {kind} <strong>{name}</strong>.
                        </p>
                        {warning && <p className="confirm-delete-warning">{warning}</p>}
                    </div>
                </div>

                <label className="form-control-label pp fs-12" htmlFor="confirm-delete-input">
                    Type <strong>{name}</strong> to confirm
                </label>
                <Input
                    id="confirm-delete-input"
                    innerRef={inputRef}
                    value={typed}
                    autoComplete="off"
                    placeholder={name}
                    onChange={(e) => setTyped(e.target.value)}
                    // Enter submits only once the name matches, so it cannot be the thing that
                    // confirms a deletion by accident.
                    onKeyDown={(e) => {
                        if (e.key === "Enter" && matches) confirm();
                    }}
                />
            </ModalBody>
            <ModalFooter>
                <Button className="shell-btn shell-btn-secondary" onClick={toggle} disabled={busy}>
                    Cancel
                </Button>
                <Button className="shell-btn shell-btn-danger" onClick={confirm} disabled={!matches || busy}>
                    {busy ? "Deleting…" : `Delete ${kind}`}
                </Button>
            </ModalFooter>
        </Modal>
    );
};

export default ConfirmDeleteModal;
