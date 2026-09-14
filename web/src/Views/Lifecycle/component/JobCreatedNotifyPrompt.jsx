import React, { useEffect, useRef, useState } from "react";
import { X, Loader, Check } from "react-feather";
import WhatsAppIcon from "../../../Common/WhatsAppIcon";
import { lifecycleBackend } from "../lifecycle_backend";
import { clientsBackend } from "../../Client/client_backend";
import { notifySuccess, notifyError } from "../../../global/toast";
import "./jobCreatedNotify.css";

// The offer to tell a customer their job-id has been raised, made at the one moment the
// answer is obvious: right after raising it.
//
// Asked rather than assumed. This sends a real WhatsApp message to a real customer, and the
// person who just typed the job is the only one who knows whether it is ready to be
// announced. Configure > WhatsApp decides how insistent the offer is (Company.notifyOnCreate):
// on, it sends itself after a few seconds unless stopped; off, it waits to be pressed. Even
// "on" never sends invisibly - the countdown is the point, not a formality.
const AUTO_SEND_SECONDS = 5;

const JobCreatedNotifyPrompt = ({ job, autoSend, onClose }) => {
    // This customer's remembered answer. null - never asked, so ask. true - they always want
    // it, so send after a countdown. The company-wide default only applies until a customer
    // has an answer of their own.
    // Whether this is the first announcement or a follow-up. Read from the server's own alert
    // state rather than guessed here, so the card's wording and the template the sender picks
    // (Helpers/JobCreatedAlert.js) can never disagree - a card that says "created" while the
    // customer receives an update message is worse than either alone.
    const isUpdate = Boolean(job.alerts?.created?.isUpdate);
    // Whichever channel this card is for. The two are remembered separately: a customer who
    // wants to hear that their job was raised does not necessarily want a message every time
    // a line is edited, and the reverse is just as reasonable.
    const remembered = isUpdate ? job.client_id?.notifyOnUpdate : job.client_id?.notifyOnCreate;
    const sendsItself = remembered === true || (remembered !== false && autoSend);

    const [busy, setBusy] = useState(false);
    const [remember, setRemember] = useState(false);
    const [remaining, setRemaining] = useState(sendsItself ? AUTO_SEND_SECONDS : null);
    // Guards against the countdown firing a second send after a manual one, and against a
    // send outliving the prompt.
    const sentRef = useRef(false);

    // Records this customer's answer. Best-effort and never blocking: the message itself is
    // what matters, and a preference that failed to save only means being asked again.
    const rememberChoice = async (value) => {
        try {
            const formData = new FormData();
            formData.set("client_id", job.client_id?._id || job.client_id);
            formData.set("kind", isUpdate ? "updated" : "created");
            formData.set("notifyOnCreate", value);
            await clientsBackend.setNotifyPreference(formData);
        } catch (error) {
            // Asked again next time, which is the safe direction.
        }
    };

    const send = async () => {
        if (sentRef.current || busy) return;
        sentRef.current = true;
        setBusy(true);
        setRemaining(null);
        try {
            if (remember) await rememberChoice("true");
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("kind", "created");
            const res = await lifecycleBackend.sendJobAlert(formData);
            notifySuccess(res.data?.alreadySent ? "This customer was already notified." : "Customer notified on WhatsApp.");
            onClose?.(res.data?.job || null);
        } catch (error) {
            // The interceptor toasts the API's own message; this covers a dead network, which
            // would otherwise leave the card spinning with nothing said.
            notifyError(error?.message || "Could not send the WhatsApp message.");
            sentRef.current = false;
            setBusy(false);
        }
    };

    // The countdown, when this company has opted into sending by default. One interval, and
    // the send happens at zero rather than on a second timer, so stopping it is immediate.
    useEffect(() => {
        if (remaining === null) return undefined;
        if (remaining === 0) {
            send();
            return undefined;
        }
        const timer = setTimeout(() => setRemaining((n) => (n === null ? null : n - 1)), 1000);
        return () => clearTimeout(timer);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [remaining]);

    if (!job) return null;

    const client = job.client_id?.clientFirm || job.client_id?.clientName || "this customer";
    const counting = remaining !== null && remaining > 0;

    return (
        <div className="job-created-notify" role="status" aria-live="polite">
            <span className="job-created-notify-icon" aria-hidden="true">
                <WhatsAppIcon size={16} />
            </span>
            <div className="job-created-notify-body">
                <p className="job-created-notify-title">
                    {job.challanNumber} {isUpdate ? "updated" : "created"}
                </p>
                <p className="job-created-notify-text">
                    {counting
                        ? `Telling ${client} on WhatsApp in ${remaining}s…`
                        : isUpdate
                        ? `Tell ${client} on WhatsApp that this job-id has changed?`
                        : `Tell ${client} on WhatsApp that this job-id is raised?`}
                </p>
                {/* Only offered when there is a choice being made. Once a customer's answer is
                    remembered the card is counting down, and the way to change your mind is
                    Cancel, which forgets it - so a checkbox here would be a second control for
                    the same decision. */}
                {!sendsItself && !busy && (
                    <button
                        type="button"
                        className="job-created-notify-remember"
                        aria-pressed={remember}
                        onClick={() => setRemember((on) => !on)}
                    >
                        <span className={`job-created-notify-box${remember ? " is-on" : ""}`} aria-hidden="true">
                            {remember && <Check size={11} />}
                        </span>
                        Remember choice for this customer
                    </button>
                )}
            </div>
            <div className="job-created-notify-actions">
                {counting ? (
                    // One button while it counts down, because there is only one thing left to
                    // decide. Cancel stops this send AND forgets the remembered answer, so the
                    // next job for this customer asks again - otherwise "cancel" would stop one
                    // message and silently leave every future one on automatic.
                    <button
                        type="button"
                        className="shell-btn"
                        onClick={async () => {
                            setRemaining(null);
                            sentRef.current = true;
                            if (remembered === true) await rememberChoice("clear");
                            onClose?.(null);
                        }}
                    >
                        Cancel
                    </button>
                ) : (
                    <>
                        <button type="button" className="shell-btn shell-btn-primary" onClick={send} disabled={busy}>
                            {busy ? <Loader size={13} className="job-created-notify-spin" aria-hidden="true" /> : null}
                            {busy ? "Sending" : "Send"}
                        </button>
                        <button
                            type="button"
                            className="shell-btn"
                            disabled={busy}
                            onClick={async () => {
                                // "Not now" with Remember ticked is an answer too: never ask
                                // for this customer again.
                                if (remember) await rememberChoice("false");
                                onClose?.(null);
                            }}
                        >
                            Not now
                        </button>
                    </>
                )}
            </div>
            <button type="button" className="job-created-notify-close" onClick={() => onClose?.(null)} aria-label="Dismiss" disabled={busy}>
                <X size={14} aria-hidden="true" />
            </button>
        </div>
    );
};

export default JobCreatedNotifyPrompt;
