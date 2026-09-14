import React, { useEffect } from "react";
import { createPortal } from "react-dom";
import { Check, Trash2, X } from "react-feather";
import { portalHost } from "./portalHost";

// Centered top-of-screen confirmation bar - used instead of the browser's native
// confirm() so it can be styled/themed and positioned deliberately (top-center).
const ConfirmDialog = ({
    open,
    message,
    confirmLabel = "Confirm",
    cancelLabel = "Cancel",
    onConfirm,
    onCancel,
    position = "top",
    // The confirm button carries the destructive styling, not Cancel. This was the other
    // way round: Cancel was painted red and Confirm blue, so on a "Delete this supplier?"
    // bar the red button - the one you reach for to destroy something - was the one that
    // abandoned the delete. Red marks the action that does the damage.
    danger = false,
}) => {
    useEffect(() => {
        if (!open) return;
        const onKeyDown = (e) => {
            if (e.key === "Escape") onCancel();
        };
        document.addEventListener("keydown", onKeyDown);
        return () => document.removeEventListener("keydown", onKeyDown);
    }, [open, onCancel]);

    if (!open) return null;

    return createPortal(
        <>
            <style>{`
                @keyframes confirm-dialog-shake {
                    0% { opacity: 0; transform: translateX(-50%) translateX(0); }
                    15% { opacity: 1; transform: translateX(-50%) translateX(-10px); }
                    30% { transform: translateX(-50%) translateX(9px); }
                    45% { transform: translateX(-50%) translateX(-7px); }
                    60% { transform: translateX(-50%) translateX(5px); }
                    75% { transform: translateX(-50%) translateX(-3px); }
                    90% { transform: translateX(-50%) translateX(1px); }
                    100% { opacity: 1; transform: translateX(-50%) translateX(0); }
                }
            `}</style>
            <div
                role="alertdialog"
                aria-modal="true"
                style={{
                    position: "fixed",
                    ...(position === "bottom" ? { bottom: 20 } : { top: 20 }),
                    left: "50%",
                    zIndex: 2000,
                    display: "flex",
                    alignItems: "center",
                    gap: 16,
                    padding: "12px 16px",
                    borderRadius: "var(--radius-control)",
                    background: "var(--bg-raised)",
                    border: "1px solid var(--border-strong, var(--border-default))",
                    boxShadow: "var(--shadow-floating)",
                    fontFamily: "var(--font-family)",
                    maxWidth: "90vw",
                    animation: "confirm-dialog-shake 0.5s ease-out forwards",
                }}
            >
                <span className="text-body-regular" style={{ color: "var(--text-primary)", fontWeight: 600, whiteSpace: "nowrap" }}>
                    {message}
                </span>
                <div className="d-flex" style={{ gap: 8, flexShrink: 0 }}>
                    {/* Icons so the two buttons read at a glance rather than as two words of
                        the same length - the destructive one carries the bin. */}
                    <button type="button" className="shell-btn shell-btn-sm shell-btn-secondary" onClick={onCancel}>
                        <X size={14} />
                        {cancelLabel}
                    </button>
                    <button
                        type="button"
                        className={["shell-btn shell-btn-sm", danger ? "shell-btn-danger" : "shell-btn-accent"].join(" ")}
                        onClick={onConfirm}
                    >
                        {danger ? <Trash2 size={14} /> : <Check size={14} />}
                        {confirmLabel}
                    </button>
                </div>
            </div>
        </>,
        portalHost()
    );
};

export default ConfirmDialog;
