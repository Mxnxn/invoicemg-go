import React, { useState } from "react";
import { createPortal } from "react-dom";
import { X, Check } from "react-feather";

import WhatsAppIcon from "../../../Common/WhatsAppIcon";
import { purchaseOrderBackend } from "../purchaseOrder_backend";
import { personBackend } from "../../../Common/person_backend";
import { notifySuccess, notifyError } from "../../../global/toast";
import "../../Lifecycle/component/jobCreatedNotify.css";
import { portalHost } from "../../../Common/portalHost";

// The offer to send a purchase order to its supplier, made at the moment it first can be sent.
//
// Deliberately on APPROVAL, not on creation. A new order is a draft, and the send route
// refuses a draft - so a prompt at creation would offer an action that comes back refused,
// which is worse than no prompt. Approval is the first moment the answer is both available and
// obvious, and for an admin who raises and approves in one sitting that is a second later.
//
// "Always" is remembered against the supplier, not the order: it is an answer about who they
// are, and the same table in Configure > WhatsApp > Suppliers can take it back.
export default function PoSendPrompt({ po, onClose, onSent }) {
    const [busy, setBusy] = useState(false);
    const [remember, setRemember] = useState(false);

    if (!po) return null;

    const supplier = po.supplier_id?.firm || po.supplier_id?.name || "this supplier";

    // Best-effort and never blocking, like the job prompt's: the message is what matters, and a
    // preference that failed to save only means being asked again.
    const rememberChoice = async () => {
        try {
            await personBackend.setNotifyPreference(po.supplier_id?._id || po.supplier_id, "notifyPoCreated", "true");
        } catch (error) {
            /* asked again next time, which is the safe direction */
        }
    };

    const send = async () => {
        if (busy) return;
        setBusy(true);
        try {
            if (remember) await rememberChoice();
            const formData = new FormData();
            formData.set("po_id", po._id);
            const res = await purchaseOrderBackend.share(formData, window.localStorage.getItem("session_token"));
            notifySuccess("Order sent to the supplier on WhatsApp.");
            onSent?.(res.data);
            onClose?.();
        } catch (error) {
            // The interceptor toasts the API's own message - including the one that names an
            // unapproved template, which is the failure people actually hit here.
            notifyError(error?.message || "Could not send the order.");
            setBusy(false);
        }
    };

    // Portalled to <body>, and on the LEFT.
    //
    // It used to render inside the detail panel, which is a SlideOverlay - and a SlideOverlay
    // carries a transform. A position:fixed descendant of a transformed ancestor is positioned
    // against that ancestor rather than the viewport, so "fixed to the corner of the screen"
    // put it in the corner of the panel instead.
    //
    // Left rather than right because the panel itself occupies the right: a prompt about the
    // order would otherwise sit on top of the order.
    return createPortal(
        <div className="job-created-notify is-left" role="status" aria-live="polite">
            <span className="job-created-notify-icon" aria-hidden="true">
                <WhatsAppIcon size={16} />
            </span>
            <div className="job-created-notify-body">
                <p className="job-created-notify-title">{po.poNumber} approved</p>
                <p className="job-created-notify-text">Send it to {supplier} on WhatsApp now?</p>

                {!busy && (
                    <button
                        type="button"
                        className="job-created-notify-remember"
                        aria-pressed={remember}
                        onClick={() => setRemember((on) => !on)}
                    >
                        <span className={`job-created-notify-box${remember ? " is-on" : ""}`} aria-hidden="true">
                            {remember && <Check size={11} />}
                        </span>
                        Remember for {supplier}
                    </button>
                )}

                <div className="job-created-notify-actions">
                    <button type="button" className="shell-btn shell-btn-primary" onClick={send} disabled={busy}>
                        {busy ? "Sending…" : "Send now"}
                    </button>
                    {/* Not now closes and nothing is remembered - the order keeps its Send
                        button in the detail panel, which is where anyone who dismisses this
                        will look for it. */}
                    <button type="button" className="shell-btn shell-btn-secondary" onClick={onClose} disabled={busy}>
                        Not now
                    </button>
                </div>
            </div>

            <button type="button" className="job-created-notify-close" aria-label="Dismiss" onClick={onClose}>
                <X size={15} />
            </button>
        </div>,
        portalHost()
    );
}
