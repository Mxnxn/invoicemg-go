import { useCallback, useEffect, useState } from "react";

import { FileText } from "react-feather";

import { createPortal } from "react-dom";
import useFlipPanel from "../../../Common/useFlipPanel";
import PoSendSection from "./PoSendSection";
import ConvertToInvoiceModal from "./ConvertToInvoiceModal";
import PoSendPrompt from "./PoSendPrompt";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { poState } from "../poState";
import { purchaseOrderBackend } from "../purchaseOrder_backend";
import "./poDetail.css";
import { portalHost } from "../../../Common/portalHost";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;
const when = (d) => (d ? new Date(d).toLocaleString("en-IN") : "");

const EDIT_WINDOW_MS = 24 * 60 * 60 * 1000;

// The same rule the server enforces in Helpers/NoteEditWindow.js. Duplicated here on
// purpose and kept deliberately trivial: the client decides only whether to OFFER the
// button, the server decides whether the edit is allowed. Offering an edit that will be
// refused is a worse experience than not offering one.
const canEdit = (note, uid) =>
    String(note.authorId) === String(uid) && Date.now() - new Date(note.createdAt).getTime() < EDIT_WINDOW_MS;

// Field paths are how the record is stored; they are not how anyone reads. "rows[1].qty"
// says nothing to the person deciding whether to re-approve an order.
const FIELD_LABEL = {
    total: "Total",
    date: "Date",
    supplier_id: "Supplier",
    material: "Product",
    qty: "Qty",
    rate: "Rate",
    discount: "Discount",
    charges: "Charges",
    description: "Description",
    unit: "Unit",
    hsn: "HSN",
    gst: "GST",
};

// Money reads as money. The record stores plain numbers, so the formatting is decided here
// by which field it is.
const MONEY_FIELDS = new Set(["total", "rate", "discount", "charges"]);

const parseField = (field) => {
    const row = String(field || "").match(/^rows\[(\d+)\](?:\.(.+))?$/);
    if (!row) return { rowIndex: null, key: field };
    return { rowIndex: Number(row[1]), key: row[2] || null };
};

/**
 * One change, in words.
 *
 * `rows` is the order's CURRENT rows, used only to name the product a change belongs to -
 * "Vinyl · Qty" reads, "rows[1].qty" does not. Rows are addressed 1-based in the record, and
 * a removed row simply has no name to find, so it falls back to its number.
 */
const describe = (change, rows = []) => {
    const { rowIndex, key } = parseField(change.field);
    const fmt = (v) => {
        if (v === "" || v === null || v === undefined) return "—";
        return MONEY_FIELDS.has(key || change.field) ? `₹${Number(v).toLocaleString("en-IN")}` : v;
    };

    let label = FIELD_LABEL[change.field] || change.field;
    if (rowIndex !== null) {
        const product = rows[rowIndex - 1]?.material;
        const where = product || `Row ${rowIndex}`;
        label = key ? `${where} · ${FIELD_LABEL[key] || key}` : where;
    }
    return `${label}: ${fmt(change.from)} → ${fmt(change.to)}`;
};

// Total and rate first: they are what someone re-approving actually needs to see, and a
// long edit otherwise buries them under description and unit changes.
const PRIORITY = ["total", "rate", "qty", "material"];
const byImportance = (a, b) => {
    const rank = (c) => {
        const { key } = parseField(c.field);
        const i = PRIORITY.indexOf(key || c.field);
        return i === -1 ? PRIORITY.length : i;
    };
    return rank(a) - rank(b);
};

// One big edit must not flood the panel.
const MAX_VISIBLE_CHANGES = 3;

export default function PoDetailSidebar({ poId, uid, rect, onClose, onChanged, onEdit }) {
    const stoken = window.localStorage.getItem("session_token");
    const [state, setState] = useState({ loading: true, error: "", po: null, history: [], notes: [] });
    const [draft, setDraft] = useState("");
    // Which note is open for editing, and the text being edited. null = none.
    const [editingNote, setEditingNote] = useState(null);
    const [busy, setBusy] = useState(false);
    const [converting, setConverting] = useState(false);
    // Raised the moment an order becomes sendable, so nobody has to remember to come back
    // and send it. See PoSendPrompt for why approval rather than creation.
    const [justApproved, setJustApproved] = useState(false);
    const [sendBusy, setSendBusy] = useState("");

    const load = useCallback(() => {
        const form = new FormData();
        form.set("po_id", poId);
        purchaseOrderBackend
            .detail(form, stoken)
            .then((res) => setState({ loading: false, error: "", ...res.data }))
            .catch(() =>
                setState({ loading: false, error: "Could not load this purchase order.", po: null, history: [], notes: [] })
            );
    }, [poId, stoken]);

    useEffect(() => {
        load();
    }, [load]);

    const act = (fn, { prompt = false } = {}) => {
        const form = new FormData();
        form.set("po_id", poId);
        fn(form, stoken)
            .then(() => {
                load();
                onChanged && onChanged();
                if (prompt) setJustApproved(true);
            })
            .catch(() => {});
    };

    const send = (kind, fn) => {
        setSendBusy(kind);
        const form = new FormData();
        form.set("po_id", poId);
        fn(form, stoken)
            .then(() => {
                load();
                onChanged && onChanged();
            })
            // The global interceptor raises the toast; the panel just stops spinning.
            .catch(() => {})
            .finally(() => setSendBusy(""));
    };

    const addNote = () => {
        const text = draft.trim();
        if (!text || busy) return;
        setBusy(true);
        const form = new FormData();
        form.set("po_id", poId);
        form.set("text", text);
        purchaseOrderBackend
            .addNote(form, stoken)
            .then(() => {
                setDraft("");
                load();
            })
            .catch(() => {})
            .finally(() => setBusy(false));
    };

    const saveNote = () => {
        const text = (editingNote?.text || "").trim();
        if (!text || busy) return;
        setBusy(true);
        const form = new FormData();
        form.set("note_id", editingNote.id);
        form.set("text", text);
        purchaseOrderBackend
            .editNote(form, stoken)
            .then(() => {
                setEditingNote(null);
                load();
            })
            // The server is the authority on the 24h window - if it refuses, the editor stays
            // open with the text intact rather than silently discarding what was typed.
            .catch(() => {})
            .finally(() => setBusy(false));
    };

    const { po, history, notes } = state;

    // Converted outranks approved: once an order has become a purchase invoice, whether it
    // was approved is history. Same rule, same words as the list card - poState() is shared
    // so the two cannot drift.
    const headState = po ? poState(po) : null;

    // The most recent revocation explains why an order that was approved no longer is.
    const lapse = po && po.approval?.state !== "approved" && history.find((h) => h.action === "Approval revoked");

    // Absolute, because it is pasted into a message and opened outside the app.
    // Grown out of the card that was pressed, rather than slid in from the edge of the
    // screen. A slide-over arrives from somewhere the order was not, and covers the deck it
    // came from; expanding out of the card keeps the connection between the thing you
    // pressed and the thing you are now reading. Same motion as the job board's card panel,
    // which is the point - one idea, not two.
    const { panelRef, transform, phase, close } = useFlipPanel({ rect, onClose });

    const supplierLink = po ? `${window.location.origin}/PO/${po.supplier_id?._id || ""}/${po._id}` : "";

    return createPortal(
        <>
            <div className={`po-scrim${phase === "opening" ? " is-open" : ""}`} onClick={close} aria-hidden="true" />
            <div
                ref={panelRef}
                className={`po-flip-panel is-${phase}`}
                style={{ transform }}
                role="dialog"
                aria-label="Purchase order"
            >
            {state.error ? (
                <p className="po-state text-body-regular">{state.error}</p>
            ) : state.loading || !po ? (
                <p className="po-state text-body-regular">Loading…</p>
            ) : (
                <div className="po-detail">
                    <header className="po-detail-head">
                        <span className="po-detail-mark" aria-hidden="true">
                            <FileText size={18} />
                        </span>
                        <span className="po-detail-ident">
                            <span className="po-detail-eyebrow">Purchase order</span>
                            <span className="po-detail-no">{po.poNumber}</span>
                            <span className="po-detail-meta text-body-small">
                                {po.supplier_id?.firm || po.supplier_id?.name} · {po.date}
                            </span>
                        </span>
                        {/* Same three states, and the same words, as the card this panel grew
                            out of - the card said "Approved" and the panel said nothing, so
                            opening one looked like losing the information. */}
                        <StatusBadge status={headState.status}>{headState.label}</StatusBadge>
                    </header>

                    {lapse && (
                        <div className="po-lapse text-body-regular">
                            <strong>Approval lapsed.</strong> {lapse.detail}
                            <ul>
                                {[...(lapse.changes || [])]
                                    .sort(byImportance)
                                    .slice(0, MAX_VISIBLE_CHANGES)
                                    .map((c, i) => (
                                        <li key={i}>{describe(c, po.rows)}</li>
                                    ))}
                            </ul>
                            {(lapse.changes || []).length > MAX_VISIBLE_CHANGES && (
                                <p className="text-body-small po-lapse-more">
                                    +{lapse.changes.length - MAX_VISIBLE_CHANGES} more below
                                </p>
                            )}
                        </div>
                    )}

                    <section className="po-section">
                        <h3 className="text-body-medium">Rows</h3>
                        <table className="po-table">
                            <thead>
                                <tr>
                                    <th scope="col">Product</th>
                                    <th scope="col">Description</th>
                                    <th scope="col">Qty</th>
                                    <th scope="col">Rate</th>
                                </tr>
                            </thead>
                            <tbody>
                                {po.rows.map((r) => (
                                    <tr key={r._id}>
                                        <td>{r.material}</td>
                                        <td>{r.description}</td>
                                        <td>
                                            {r.qty} {r.unit}
                                        </td>
                                        <td>{money(r.rate)}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                        <p className="po-total text-body-medium">Total {money(po.total)}</p>
                    </section>

                    <section className="po-section">
                        <h3 className="text-body-medium">Notes</h3>
                        <ul className="po-notes">
                            {notes.map((n) => (
                                <li key={n._id}>
                                    {editingNote?.id === n._id ? (
                                        <div className="po-note-edit">
                                            <textarea
                                                className="po-note-input text-body-regular"
                                                rows={2}
                                                value={editingNote.text}
                                                aria-label="Edit note"
                                                onChange={(e) => setEditingNote({ ...editingNote, text: e.target.value })}
                                            />
                                            <div className="po-note-actions">
                                                <button
                                                    type="button"
                                                    className="shell-btn shell-btn-secondary"
                                                    onClick={() => setEditingNote(null)}
                                                >
                                                    Cancel
                                                </button>
                                                <button
                                                    type="button"
                                                    className="shell-btn shell-btn-primary"
                                                    onClick={saveNote}
                                                    disabled={busy || !editingNote.text.trim()}
                                                >
                                                    Save
                                                </button>
                                            </div>
                                        </div>
                                    ) : (
                                        <div className="po-note-row">
                                            <span className="text-body-regular po-note-text">{n.text}</span>
                                            <span className="text-body-small po-note-by">{n.authorName}</span>
                                            {canEdit(n, uid) && (
                                                <button
                                                    type="button"
                                                    className="shell-btn shell-btn-secondary"
                                                    aria-label={`Edit note by ${n.authorName}`}
                                                    onClick={() => setEditingNote({ id: n._id, text: n.text })}
                                                >
                                                    Edit
                                                </button>
                                            )}
                                        </div>
                                    )}
                                </li>
                            ))}
                        </ul>

                        {/* The endpoints existed from the start with nothing calling them. A note
                            can be edited by its author within 24h (Helpers/NoteEditWindow.js),
                            which is why the Edit button comes and goes on its own. */}
                        <div className="po-note-compose">
                            <textarea
                                className="po-note-input text-body-regular"
                                rows={2}
                                placeholder="Add a note"
                                aria-label="Add a note"
                                value={draft}
                                onChange={(e) => setDraft(e.target.value)}
                            />
                            <button
                                type="button"
                                className="shell-btn shell-btn-primary"
                                onClick={addNote}
                                disabled={busy || !draft.trim()}
                            >
                                Add note
                            </button>
                        </div>
                    </section>

                    {/* Only once approved, and only until converted - which is exactly when the
                        public endpoint serves it. Offering a link that answers "closed" would
                        be worse than offering none. */}
                    {po.approval?.state === "approved" && !po.purchaseInvoice_id && (
                        <section className="po-section">
                            <h3 className="text-body-medium">Supplier link</h3>
                            <p className="text-body-small po-link-hint">
                                Anyone with this link can read this order and download it. It stops working once the
                                order becomes a purchase invoice.
                            </p>
                            <div className="po-link-row">
                                <code className="po-link text-body-small">{supplierLink}</code>
                                <button
                                    type="button"
                                    className="shell-btn shell-btn-secondary"
                                    onClick={() => navigator.clipboard?.writeText(supplierLink)}
                                >
                                    Copy
                                </button>
                            </div>
                        </section>
                    )}

                    <PoSendSection
                        send={state.send}
                        busy={sendBusy}
                        onShare={() => send("share", purchaseOrderBackend.share)}
                        onConfirm={() => send("confirm", purchaseOrderBackend.confirm)}
                    />

                    {!po.purchaseInvoice_id && (
                        <footer className="po-detail-foot">
                            {/* Editing is what the approval control reacts to, so it has to be
                                reachable - but never after conversion, which the server also
                                refuses. */}
                            {onEdit && (
                                <button
                                    type="button"
                                    className="shell-btn shell-btn-secondary po-foot-edit"
                                    onClick={() => onEdit(po)}
                                >
                                    Edit
                                </button>
                            )}
                            {po.approval?.state === "approved" ? (
                                <>
                                    <button
                                        type="button"
                                        className="shell-btn shell-btn-secondary"
                                        onClick={() => act(purchaseOrderBackend.revoke)}
                                    >
                                        Withdraw approval
                                    </button>
                                    {/* The end of the order's life, and until now unreachable:
                                        the convert route has existed since the order work
                                        shipped with nothing in the UI calling it, so an order
                                        could never actually become a bill. Offered only once
                                        approved, which is the same condition the route
                                        enforces. */}
                                    <button
                                        type="button"
                                        className="shell-btn shell-btn-primary"
                                        onClick={() => setConverting(true)}
                                    >
                                        <FileText size={14} /> Generate purchase invoice
                                    </button>
                                </>
                            ) : (
                                <button
                                    type="button"
                                    className="shell-btn shell-btn-primary"
                                    onClick={() => act(purchaseOrderBackend.approve, { prompt: true })}
                                >
                                    Approve
                                </button>
                            )}
                        </footer>
                    )}

                    {/* History last, deliberately.
                        It used to sit second, between the rows and the notes, so an order
                        with a long history buried every control that acts on it - you
                        scrolled past a month of edits to reach Approve. Nothing in history
                        is actionable; everything below it was. Now the things you came to
                        do are reachable without scrolling and the record sits under them. */}
                    <section className="po-section po-history-section">
                        <h3 className="text-body-medium">History</h3>
                        <ul className="po-history">
                            {history.map((h) => (
                                <li key={h._id}>
                                    <div className="po-history-head">
                                        <span className="text-body-medium">{h.action}</span>
                                        <span className="text-body-small po-history-meta">
                                            {h.actorName} · {when(h.createdAt)}
                                        </span>
                                    </div>
                                    {h.detail && <p className="text-body-small po-history-detail">{h.detail}</p>}
                                    {(h.changes || []).length > 0 && (
                                        <ul className="po-changes">
                                            {[...h.changes]
                                                .sort(byImportance)
                                                .slice(0, MAX_VISIBLE_CHANGES)
                                                .map((c, i) => (
                                                    <li key={i} className="text-body-small po-change">
                                                        {describe(c, po.rows)}
                                                    </li>
                                                ))}
                                            {h.changes.length > MAX_VISIBLE_CHANGES && (
                                                <li className="text-body-small po-change">
                                                    +{h.changes.length - MAX_VISIBLE_CHANGES} more
                                                </li>
                                            )}
                                        </ul>
                                    )}
                                </li>
                            ))}
                        </ul>
                    </section>
                </div>
            )}

            {justApproved && (
                <PoSendPrompt
                    po={state.po}
                    onClose={() => setJustApproved(false)}
                    onSent={() => {
                        load();
                        onChanged && onChanged();
                    }}
                />
            )}

            <ConvertToInvoiceModal
                po={po}
                isOpen={converting}
                toggle={() => setConverting(false)}
                onConverted={() => {
                    // Closes the panel as well as refreshing: a converted order can no longer
                    // be approved, edited or sent, so leaving the panel open would leave every
                    // control in it offering something the server now refuses.
                    onChanged?.();
                    onClose?.();
                }}
            />
            </div>
        </>,
        portalHost()
    );
}
