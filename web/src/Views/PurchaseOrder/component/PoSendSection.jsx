import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { hasAccess } from "../../../Common/access";

const when = (d) => (d ? new Date(d).toLocaleString("en-IN") : "");

// `status` values come from Common/DataTable/StatusBadge - only the ones with a rule in
// dataTable.css are coloured.
const TONE = {
    Created: "neutral",
    Sent: "green",
    Modified: "amber",
    "Sent Update": "green",
};

// Why the share button is off, in words. An inert control with no explanation is the thing
// to avoid - the server already knows the reason, so say it.
const shareBlockedReason = (send) => {
    // count > 0 and still blocked can only mean the fingerprint matches - the supplier has
    // this exact version. Otherwise the order is not in a sendable state at all.
    if (send.count > 0) return "The supplier already has this version of the order.";
    return "This order cannot be sent yet — approve it first.";
};

/**
 * What the supplier has been told, and what can be sent next.
 *
 * Props only - every decision about what is allowed is made server-side in
 * Helpers/PoSend.js, so the button and the route cannot disagree.
 */
export default function PoSendSection({ send, onShare, onConfirm, busy }) {
    if (!send) return null;
    const canSend = hasAccess("purchase_orders_send");

    return (
        <section className="po-section">
            <div className="po-send-head">
                <h3 className="text-body-medium">Supplier</h3>
                <StatusBadge status={TONE[send.status] || "neutral"}>{send.status}</StatusBadge>
            </div>

            {send.sentAt && (
                <p className="text-body-small po-send-meta">
                    Last sent {when(send.sentAt)}
                    {send.count > 1 ? ` · ${send.count} sends` : ""}
                </p>
            )}
            {send.confirmedAt && <p className="text-body-small po-send-meta">Dispatch confirmed {when(send.confirmedAt)}</p>}

            {!send.canShare && <p className="text-body-small po-send-meta">{shareBlockedReason(send)}</p>}

            {canSend && (
                <>
                    {/* The two messages, said plainly. They were labelled "Share order" and
                        "Send PO", which had them the wrong way round to read: the one called
                        "Send PO" was the dispatch confirmation, and the one that actually sends
                        the order was the one called "share". Whichever you pressed, you got the
                        other. The primary is now the one you reach for first - sending the
                        order - and the confirmation is secondary, because it comes second. */}
                    <p className="text-body-small po-send-meta">
                        Two separate messages: the order itself for them to read, and the
                        go-ahead to dispatch it.
                    </p>
                    <div className="po-send-actions">
                        <button
                            type="button"
                            className="shell-btn shell-btn-primary"
                            onClick={onShare}
                            disabled={!send.canShare || busy === "share"}
                            title="Sends the order to the supplier with a link to read it"
                        >
                            {busy === "share"
                                ? "Sending…"
                                : send.isUpdate
                                ? "Send updated order"
                                : "Send order to supplier"}
                        </button>
                        <button
                            type="button"
                            className="shell-btn shell-btn-secondary"
                            onClick={onConfirm}
                            disabled={!send.canConfirm || busy === "confirm"}
                            title="Tells the supplier to dispatch the order"
                        >
                            {busy === "confirm" ? "Sending…" : "Confirm dispatch"}
                        </button>
                    </div>
                </>
            )}
        </section>
    );
}
