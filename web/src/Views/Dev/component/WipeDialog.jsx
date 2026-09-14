import React, { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { AlertTriangle } from "react-feather";
import { portalHost } from "../../../Common/portalHost";

// Wipe is the most dangerous action in the app, so the dialog is deliberately obstructive:
// it states exactly what will move, and the button stays dead until the operator types the
// target's email. ConfirmDialog is a plain message bar with no input, hence a separate
// component rather than bending that one out of shape.
//
// The same email check runs server-side - this is a speed bump for the operator, not the
// security boundary.
const WipeDialog = ({ open, target, footprint, busy, onConfirm, onCancel }) => {
    const [typed, setTyped] = useState("");

    useEffect(() => {
        if (open) setTyped("");
    }, [open, target]);

    useEffect(() => {
        if (!open) return undefined;
        const onKey = (e) => {
            if (e.key === "Escape" && !busy) onCancel();
        };
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    }, [open, busy, onCancel]);

    if (!open || !target) return null;

    const matches = typed.trim().toLowerCase() === String(target.email).toLowerCase();

    return createPortal(
        <div className="dev-modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="wipe-title">
            <div className="dev-modal dev-modal--danger">
                <div className="dev-modal-head">
                    <AlertTriangle size={18} />
                    <span id="wipe-title">Archive all data for this customer</span>
                </div>

                <div className="dev-modal-body">
                    <p className="dev-modal-lead">
                        <strong>{target.name || target.email}</strong> ({target.email})
                    </p>

                    {footprint ? (
                        <div className="dev-footprint">
                            <div className="dev-footprint-total">
                                <span className="dev-footprint-num">{footprint.total.toLocaleString()}</span>
                                <span>documents across {footprint.collections} collections</span>
                            </div>
                            <ul className="dev-footprint-list">
                                {footprint.breakdown.map((b) => (
                                    <li key={b.collection}>
                                        <span>{b.collection}</span>
                                        <span>{b.count.toLocaleString()}</span>
                                    </li>
                                ))}
                            </ul>
                        </div>
                    ) : (
                        <p className="dev-muted">Counting…</p>
                    )}

                    <p className="dev-reassure">
                        This is reversible. Everything moves into <code>&lt;collection&gt;_deleted</code> and can be put
                        back with Restore. The customer loses access immediately.
                    </p>

                    <label className="dev-label" htmlFor="wipe-confirm">
                        Type <strong>{target.email}</strong> to confirm
                    </label>
                    <input
                        id="wipe-confirm"
                        className="dev-input"
                        value={typed}
                        onChange={(e) => setTyped(e.target.value)}
                        placeholder={target.email}
                        autoComplete="off"
                        disabled={busy}
                    />
                </div>

                <div className="dev-modal-foot">
                    <button type="button" className="dev-btn" onClick={onCancel} disabled={busy}>
                        Cancel
                    </button>
                    <button
                        type="button"
                        className="dev-btn dev-btn--danger"
                        disabled={!matches || busy}
                        onClick={() => onConfirm(typed.trim())}
                    >
                        {busy ? "Archiving…" : "Archive everything"}
                    </button>
                </div>
            </div>
        </div>,
        portalHost()
    );
};

export default WipeDialog;
