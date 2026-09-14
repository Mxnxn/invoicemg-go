import React, { useState } from "react";
import { Check, AlertTriangle, Loader, CheckCircle } from "react-feather";

import WhatsAppIcon from "../../../Common/WhatsAppIcon";
import { lifecycleBackend } from "../lifecycle_backend";
import { notifySuccess, notifyError } from "../../../global/toast";
import "./jobAlertActions.css";

// What each delivery state says to the operator.
//
// WhatsApp's own vocabulary, because every user already reads it fluently on their own phone
// - one tick sent, two ticks delivered, coloured ticks read. Inventing a private wording for
// something people see fifty times a day would only make it slower to parse.
//
// "accepted" is the honest gap: Meta took the message but no delivery callback has arrived,
// either because the webhook is not configured or because it is simply still in flight. It
// reads as "Sent" rather than claiming a delivery nobody has confirmed.
export const ALERT_STATUS_COPY = {
    accepted: { label: "Sent", ticks: 1, tone: "muted", hint: "Handed to WhatsApp. No delivery confirmation yet." },
    sent: { label: "Sent", ticks: 1, tone: "muted", hint: "Sent to the customer's number." },
    delivered: { label: "Delivered", ticks: 2, tone: "muted", hint: "Reached the customer's phone." },
    read: { label: "Read", ticks: 2, tone: "read", hint: "The customer opened the message." },
    failed: { label: "Not delivered", ticks: 0, tone: "failed", hint: "WhatsApp could not deliver this message." },
};

// Two overlapping ticks for the delivered/read states - react-feather has no double-check
// glyph, and pulling in a second icon set for one mark is not worth it.
const Ticks = ({ count }) =>
    count === 0 ? (
        <AlertTriangle size={12} aria-hidden="true" />
    ) : (
        <span className={`alert-ticks${count === 2 ? " is-double" : ""}`} aria-hidden="true">
            <Check size={12} />
            {count === 2 && <Check size={12} />}
        </span>
    );

export const AlertDeliveryStatus = ({ status, at, error }) => {
    const copy = ALERT_STATUS_COPY[status] || ALERT_STATUS_COPY.accepted;
    const when = at ? new Date(at) : null;
    return (
        <span className={`alert-status alert-status-${copy.tone}`} title={error || copy.hint}>
            <Ticks count={copy.ticks} />
            <span className="alert-status-label">{copy.label}</span>
            {when && (
                <span className="alert-status-time">
                    {when.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}
                </span>
            )}
        </span>
    );
};

/**
 * Everything a job-id can be told to do about itself: finish its work, and tell the customer.
 *
 * Extracted from JobDetailModal so the board can offer the same things. It was the one part of
 * the table view the board had no answer for - a job could be dragged to Done there and then
 * had to be reopened in the table to notify anybody, which is the sort of gap that teaches
 * people not to trust the new view.
 *
 * Two groups, not five siblings. They used to sit in one wrapping flex row - heading, mark-all,
 * two status chips, two send buttons - so at any width below "all of it fits" they scattered:
 * a status chip could wrap below a button that reported on it, and which control landed where
 * depended on how long the customer's last message had been. Grouping them means they wrap as
 * blocks, and the reading order holds at every width:
 *
 *   what has already happened   |   what you can do about it
 *   (quiet, passive, reporting) |   (send, then finish)
 *
 * `sendingKind` is one piece of state rather than two booleans: only one message can be going
 * out at a time, and the busy button is the one that was pressed.
 */
export default function JobAlertActions({ job, onChange, showMarkAllDone = true, className = "" }) {
    const rows = job?.rows || [];
    // Every row Done means the work is finished - worth saying, and nothing more.
    const allRowsDone = rows.length > 0 && rows.every((r) => r.queue === "Done");
    // The moment the job stops being "just raised" and starts being work in progress. Once any
    // card is finished, the message the customer wants is the one about collection - so the
    // "job raised" send steps aside for it rather than both sitting there competing. Two sends
    // offering to tell the same customer two different things about the same job is how the
    // wrong one gets pressed.
    const anyRowDone = rows.some((r) => r.queue === "Done");

    // Per-channel state from Helpers/JobAlertState, so the buttons and the route cannot
    // disagree about what is allowed. job.alert is the pre-channel shape and still stands in
    // for `done` on a payload from an older API.
    const channels = job?.alerts || {};
    const alert = channels.done || job?.alert || {};
    const createdAlert = channels.created || {};

    const [sendingKind, setSendingKind] = useState(null);
    const [completingAll, setCompletingAll] = useState(false);
    // The server's status once it has one. A send just made in this session has no callback
    // yet, so it starts at "accepted" - the same state the server records on a fresh send.
    const [alertStatus, setAlertStatus] = useState(alert.status || job?.alertStatus || null);
    const [createdStatus, setCreatedStatus] = useState(createdAlert.status || null);

    const phone = job?.client_id?.clientPhone;

    // Every row to Done in one action, instead of opening each card's queue dialog in turn.
    // The server skips rows already Done, so pressing it twice is harmless.
    const onCompleteAll = async () => {
        if (completingAll) return;
        setCompletingAll(true);
        try {
            const formData = new FormData();
            formData.set("job_id", job._id);
            const res = await lifecycleBackend.completeAllRows(formData);
            onChange?.(res.data);
            notifySuccess(res.message || "Rows marked done.");
        } catch (error) {
            notifyError(error?.message || "Could not update the rows.");
        } finally {
            setCompletingAll(false);
        }
    };

    // The whole-job template send. The server is idempotent - a duplicate resolves with
    // alreadySent rather than messaging twice - so the worst a double-click can do is show the
    // same confirmation again.
    const onSendJobAlert = async (kind) => {
        const state = kind === "created" ? createdAlert : alert;
        if (sendingKind || !state.canSend) return;
        setSendingKind(kind);
        try {
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("kind", kind);
            const res = await lifecycleBackend.sendJobAlert(formData);
            // Optimistic only as far as the server itself goes on a fresh send: "accepted"
            // means Meta took it, and everything after that arrives by webhook.
            if (kind === "created") setCreatedStatus("accepted");
            else setAlertStatus("accepted");
            if (res.data?.job) onChange?.(res.data.job);
            notifySuccess(res.data?.alreadySent ? "This customer was already notified." : "Customer notified on WhatsApp.");
        } catch (error) {
            notifyError(error?.message || "Could not send the WhatsApp message.");
        } finally {
            setSendingKind(null);
        }
    };

    const canMarkAllDone = showMarkAllDone && !allRowsDone && job?.lock?.canEditQueue !== false;
    // The first announcement is offered by the post-create prompt, but that prompt appears once
    // and is gone. Adding or removing cards reopens canSend and marks it an update, sending the
    // updated template rather than announcing the job as new twice.
    const canSendCreated = createdAlert.canSend && !anyRowDone;
    const showStatus = createdAlert.sentBefore || alert.sentBefore;
    const showActions = canSendCreated || alert.canSend || canMarkAllDone;

    if (!showStatus && !showActions) return null;

    return (
        <div className={`job-action-cluster ${className}`.trim()}>
            {/* What has already happened. Quiet by design - it reports on something already
                done, so it must not compete with the actions beside it. */}
            {showStatus && (
                <div className="job-action-status">
                    {createdAlert.sentBefore && (
                        <AlertDeliveryStatus
                            status={createdStatus}
                            at={createdAlert.statusAt}
                            error={createdAlert.error}
                        />
                    )}
                    {alert.sentBefore && (
                        <AlertDeliveryStatus
                            status={alertStatus}
                            at={alert.statusAt || job?.alertStatusAt}
                            error={alert.error || job?.alertError}
                        />
                    )}
                </div>
            )}

            {showStatus && showActions && <span className="job-action-divider" aria-hidden="true" />}

            {/* What you can do about it: tell the customer, then finish the work. Mark all done
                sits last because it is the one that changes the job rather than reports it. */}
            {showActions && (
                <div className="job-action-buttons">
                    {canSendCreated && (
                        <button
                            type="button"
                            className="whatsapp-send-btn"
                            title={
                                !phone
                                    ? "No phone number on file for this client"
                                    : createdAlert.changed
                                    ? "Tell the client this job-id has changed"
                                    : "Tell the client this job-id has been raised"
                            }
                            disabled={!phone || Boolean(sendingKind)}
                            onClick={() => onSendJobAlert("created")}
                        >
                            {sendingKind === "created" ? (
                                <Loader size={13} className="whatsapp-send-btn-spin" />
                            ) : (
                                <WhatsAppIcon size={13} />
                            )}
                            {sendingKind === "created" ? "Sending" : createdAlert.isUpdate ? "Send job update" : "Notify created"}
                        </button>
                    )}

                    {/* Once every card is done the job is finished as a whole, and the customer
                        wants one message about the job - not one per line item. */}
                    {alert.canSend && (
                        <button
                            type="button"
                            className="whatsapp-send-btn"
                            title={
                                phone
                                    ? "Tell the customer on WhatsApp that the job is ready to pick up"
                                    : "No phone number on file for this client"
                            }
                            disabled={!phone || Boolean(sendingKind)}
                            onClick={() => onSendJobAlert("done")}
                        >
                            {sendingKind === "done" ? (
                                <Loader size={13} className="whatsapp-send-btn-spin" />
                            ) : (
                                <WhatsAppIcon size={13} />
                            )}
                            {sendingKind === "done"
                                ? "Sending"
                                : alert.isUpdate
                                ? `Send update (${alert.pendingRows} new)`
                                : "Notify Ready to Pick"}
                        </button>
                    )}

                    {canMarkAllDone && (
                        <button
                            type="button"
                            className="shell-btn shell-btn-secondary shell-btn-sm"
                            onClick={onCompleteAll}
                            disabled={completingAll}
                            title="Move every row on this job to Done"
                        >
                            <CheckCircle size={14} aria-hidden="true" />
                            {completingAll ? "Marking…" : "Mark all done"}
                        </button>
                    )}
                </div>
            )}
        </div>
    );
}
