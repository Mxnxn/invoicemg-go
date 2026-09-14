import { useEffect, useRef, useState } from "react";
import { Bell, FileText, Check, X } from "react-feather";

import "./notificationBell.css";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

// "0 days" reads as a bug. Age is why an item deserves attention, so it is phrased the way
// someone would say it out loud.
const age = (days) => {
    if (!days || days < 1) return "today";
    if (days === 1) return "1 day";
    return `${days} days`;
};

/**
 * The approval queue, in the navbar.
 *
 * Props only - it fetches nothing. That keeps the suite able to drive every state directly,
 * and leaves the provider (Shell/pendingApprovals.jsx) as the one place that knows about the
 * network.
 *
 * `count`/`items` are what this person has not yet waved away; `totalPending` is everything
 * still awaiting approval. Both, because a bell that said "nothing waiting" while orders sat
 * unapproved would be lying about the one thing it exists to report.
 */
export default function NotificationBell({
    count = 0,
    totalPending = 0,
    items = [],
    onSelect,
    onOpen,
    onDismiss,
}) {
    const [open, setOpen] = useState(false);
    const wrapRef = useRef(null);

    useEffect(() => {
        if (!open) return undefined;
        const onKey = (e) => e.key === "Escape" && setOpen(false);
        const onClick = (e) => {
            if (wrapRef.current && !wrapRef.current.contains(e.target)) setOpen(false);
        };
        document.addEventListener("keydown", onKey);
        document.addEventListener("mousedown", onClick);
        return () => {
            document.removeEventListener("keydown", onKey);
            document.removeEventListener("mousedown", onClick);
        };
    }, [open]);

    const toggle = () => {
        const next = !open;
        setOpen(next);
        // Opening is the moment someone looks, so it is the moment a stale badge should
        // correct itself - cheaper and more honest than polling.
        if (next && onOpen) onOpen();
    };

    return (
        <div className="bell-wrap" ref={wrapRef}>
            <button
                type="button"
                className="shell-icon-btn bell-btn"
                aria-label={`Pending approvals${count ? `, ${count} waiting` : ""}`}
                aria-expanded={open}
                title="Pending approvals"
                onClick={toggle}
            >
                <Bell size={16} />
                {count > 0 && (
                    <span className="bell-badge text-label-caps" data-testid="bell-badge">
                        {count > 99 ? "99+" : String(count)}
                    </span>
                )}
            </button>

            {open && (
                <div className="bell-panel" role="menu" aria-label="Pending approvals">
                    <div className="bell-panel-head text-body-small">Waiting for approval</div>

                    {items.length === 0 ? (
                        // Two different facts, and only one of them means there is no work.
                        // Saying "nothing waiting" while five orders sit unapproved because
                        // they were waved away would be the bell lying about the thing it
                        // exists to report - so when there are dismissed ones, it says so and
                        // points at where they are.
                        totalPending > 0 ? (
                            <p className="bell-empty text-body-regular">
                                Nothing new.
                                <span className="bell-empty-sub text-body-small">
                                    {totalPending === 1
                                        ? "1 order is still waiting for approval."
                                        : `${totalPending} orders are still waiting for approval.`}{" "}
                                    <button
                                        type="button"
                                        className="bell-empty-link"
                                        onClick={() => {
                                            setOpen(false);
                                            onSelect && onSelect(null);
                                        }}
                                    >
                                        Open purchase orders
                                    </button>
                                </span>
                            </p>
                        ) : (
                            <p className="bell-empty text-body-regular">Nothing waiting for approval.</p>
                        )
                    ) : (
                        <ul className="bell-list">
                            {items.map((item) => (
                                <li key={item._id} className="bell-row">
                                    <button
                                        type="button"
                                        className="bell-item"
                                        role="menuitem"
                                        onClick={() => {
                                            setOpen(false);
                                            onSelect && onSelect(item._id);
                                        }}
                                    >
                                        <FileText size={16} className="bell-item-icon" aria-hidden="true" />
                                        <span className="bell-item-text">
                                            <span className="text-body-medium">{item.poNumber}</span>
                                            <span className="text-body-small bell-item-sub">
                                                {item.supplierName} · {age(item.ageDays)}
                                            </span>
                                        </span>
                                        <span className="text-body-medium bell-item-total">{money(item.total)}</span>
                                    </button>
                                    {/* Its own button outside the row's, not nested inside it -
                                        a button within a button is invalid and the click would
                                        open the order as well as dismissing it. */}
                                    {onDismiss && (
                                        <button
                                            type="button"
                                            className="bell-dismiss"
                                            aria-label={`Mark ${item.poNumber} as read`}
                                            title="Mark as read"
                                            onClick={() => onDismiss(item._id)}
                                        >
                                            <X size={14} />
                                        </button>
                                    )}
                                </li>
                            ))}
                        </ul>
                    )}

                    {/* Only when there is something to mark. An always-present button that
                        does nothing on an empty list reads as broken. */}
                    {items.length > 0 && onDismiss && (
                        <div className="bell-panel-foot">
                            <button type="button" className="bell-markall" onClick={() => onDismiss(null)}>
                                <Check size={14} /> Mark all as read
                            </button>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
