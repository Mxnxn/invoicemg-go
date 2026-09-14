import React, { useState } from "react";
import { createPortal } from "react-dom";
import { X, Trash2 } from "react-feather";

import useFlipPanel from "../../../Common/useFlipPanel";
import { lifecycleBackend } from "../lifecycle_backend";
import { notifySuccess } from "../../../global/toast";
import { rowDimensions } from "../jobMath";
import { splitGst, igstOnly } from "../../../Common/gst";
import { isIgstJob, totalTaxPercent } from "../jobTax";
import AssigneeDropdown from "./AssigneeDropdown";
import "./jobBoard.css";
import { portalHost } from "../../../Common/portalHost";

// One card, opened out of itself.
//
// It replaces a centred modal. A modal arrives from nowhere and covers the board, so the thing
// you were looking at is gone and the thing you are editing has no evident relationship to it.
// Growing out of the card keeps the connection, and is the same motion the board itself makes -
// one idea rather than two.
//
// Portalled to <body> deliberately. The board is a transformed element while it opens and
// closes, and a position:fixed descendant of a transformed ancestor is positioned against that
// ancestor rather than the viewport - so nested inside, this panel would be dragged around by
// the board's own animation.
//
// Who it is for, then what it is. The assignee comes FIRST because it is the field that decides
// whether the card can move at all - a card with nobody on it is refused every working stage,
// so asking for it after eight commercial fields is asking for it last and finding out first.
//
// It does NOT travel with Save. Save goes through /jobs/update, whose normalizeRow carries only
// the commercial fields - an assignee put in that payload would be silently dropped. So the
// picker writes immediately on its own route, the same one the board's chips use, which is also
// why the two can never disagree: they are the same write.
//
// The stage is still absent. Dragging owns that, and a select here would be a second way to do
// the thing the board exists to do.
//
// Size and quantity are no longer both always on screen. A row records which way it is
// measured (`hasDimensions`), and a by-quantity row still showing length and width invites
// someone to fill them in - which measures the row two ways and totals it a third. The toggle
// is the field; the inputs follow it, the same as the new-card panel.
const ALWAYS = [
    { key: "material", label: "Product", type: "text", wide: true },
    { key: "description", label: "Description", type: "text", wide: true },
];
const SIZE_FIELDS = [
    { key: "length", label: "Length", type: "text" },
    { key: "width", label: "Width", type: "text" },
];
const COMMERCIAL = [
    { key: "qty", label: "Quantity", type: "number" },
    { key: "rate", label: "Rate", type: "number" },
    { key: "discount", label: "Discount", type: "number" },
    { key: "charges", label: "Charges", type: "number" },
];
const FIELDS = [...ALWAYS, ...SIZE_FIELDS, ...COMMERCIAL];

const Field = ({ field, form, setForm }) => (
    <div className={field.wide ? "is-wide" : undefined}>
        <label className="form-control-label pp fs-12" htmlFor={`card-${field.key}`}>
            {field.label}
        </label>
        <input
            id={`card-${field.key}`}
            className="form-control"
            type={field.type}
            value={form[field.key]}
            onChange={(e) => setForm((prev) => ({ ...prev, [field.key]: e.target.value }))}
        />
    </div>
);

export default function JobCardPanel({ job, card, rect, onClose, onSaved, employees = [], onAssign, onCreateEmployee }) {
    const [form, setForm] = useState(() => {
        const seed = {};
        FIELDS.forEach((f) => (seed[f.key] = card?.[f.key] ?? ""));
        // One number in, split on the way out - the same convention every other row in this
        // app is stored under. totalTaxPercent reads whichever side the row actually used.
        seed.tax = String(totalTaxPercent(card || {}) || "");
        seed.hasDimensions = card?.hasDimensions !== false;
        return seed;
    });
    // The job's regime, not the card's. A job is interstate or it is not, and one card
    // disagreeing with its own job is a filing that does not add up.
    const igst = isIgstJob(job?.rows || []);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");
    const { panelRef, transform, phase, close } = useFlipPanel({ rect, onClose });

    // Whether this card may be taken off the job at all. The server decides (JobLock:
    // canDeleteRow - not invoiced, or invoiced and deliberately unlocked) and the update route
    // enforces the same rule, so this control cannot offer what the route would refuse.
    const canDeleteRow = job?.lock?.canDeleteRow !== false;
    const [confirming, setConfirming] = useState(false);

    // Removal is the same write as a save: every row travels, this one left out. The server
    // takes it from there - a removed row that was already billed has its Entry detached from
    // the invoice (Helpers/SyncInvoiceFromJob), and a job that would be left with no rows at
    // all is refused, which surfaces in the error line below.
    const remove = async () => {
        if (busy) return;
        if (!confirming) return setConfirming(true);

        setError("");
        setBusy("removing");
        try {
            const rows = (job.rows || []).filter((r) => String(r._id) !== String(card._id));
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("rows", JSON.stringify(rows));
            const res = await lifecycleBackend.updateJob(formData);
            notifySuccess("Card removed.");
            onSaved?.(res.data);
            close();
        } catch (err) {
            setError(err?.message || "Couldn't remove that card.");
            setConfirming(false);
            setBusy(false);
        }
    };

    const save = async (e) => {
        e.preventDefault();
        setError("");
        if (!String(form.material || "").trim()) return setError("A card needs a product.");

        // Named rather than boolean: the footer has two busy buttons now and each must show
        // its own progress, not both at once.
        setBusy("saving");
        try {
            // Every row travels, with this one's fields replaced. Spreading the existing row
            // first means the card's stage and assignee go back exactly as they came - the
            // server's normalizeRow ignores them either way, and sending something different
            // would be a lie in the payload waiting for someone to act on it.
            const tax = Number(form.tax) || 0;
            const { tax: _entered, ...fields } = form;
            const patch = {
                ...fields,
                // By quantity the two factors that are not asked for go back as the identity
                // rowAmount multiplies by - 1, not 0, or the line would total nothing.
                length: form.hasDimensions ? form.length : "1",
                width: form.hasDimensions ? form.width : "1",
                // splitGst returns only cgst/sgst deliberately, so igst is cleared explicitly
                // rather than left behind from whatever the row used to be.
                ...(igst ? igstOnly(tax) : { ...splitGst(tax), igst: 0 }),
            };
            const rows = (job.rows || []).map((r) =>
                String(r._id) === String(card._id) ? { ...r, ...patch } : r
            );
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("rows", JSON.stringify(rows));
            const res = await lifecycleBackend.updateJob(formData);
            notifySuccess("Card updated.");
            onSaved?.(res.data);
            close();
        } catch (err) {
            setError(err?.message || "Couldn't save that card.");
            setBusy(false);
        }
    };

    if (!card) return null;

    const size = rowDimensions(card);

    return createPortal(
        <div
            ref={panelRef}
            className={`job-card-panel is-${phase}`}
            style={{ transform }}
            role="dialog"
            aria-label={`Card ${card.rowId || ""}`}
        >
            <div className="job-board-overlay-head">
                <span className="job-board-overlay-meta">
                    <span className="text-heading-brand">{card.material || card.rowId}</span>
                    <span className="text-body-small">{card.rowId}</span>
                    {size && size !== "1 x 1" && <span className="text-body-small">{size}</span>}
                </span>
                <button type="button" className="shell-icon-btn" aria-label="Close card" onClick={close}>
                    <X size={16} />
                </button>
            </div>

            <form onSubmit={save} className="job-card-panel-body">
                {onAssign && (
                    <div className="job-card-panel-assign">
                        <label className="form-control-label pp fs-12" htmlFor="card-assignee">
                            Assigned to
                        </label>
                        <AssigneeDropdown
                            value={card.employee_id?.name || ""}
                            placeholder="Unassigned"
                            options={employees}
                            onSelect={(id) => onAssign(card, id)}
                            onCreateNew={onCreateEmployee ? (name) => onCreateEmployee(name, card) : undefined}
                            fullWidth
                        />
                        {/* Said here rather than left to be discovered at the moment a drag is
                            refused - this is where someone is already looking at the card. */}
                        <span className="text-body-small job-card-panel-hint">
                            A card needs someone on it before it can move into a working stage. Saved as soon
                            as you pick, not with the rest.
                        </span>
                    </div>
                )}

                <div className="job-card-panel-fields">
                    {ALWAYS.map((f) => (
                        <Field key={f.key} field={f} form={form} setForm={setForm} />
                    ))}
                </div>

                {/* How this card is measured. The same control the new-card panel uses, so a
                    card can be changed from one to the other after the fact - which was the
                    only way to fix a row entered the wrong way round. */}
                <div className="new-card-measure" role="radiogroup" aria-label="How this card is measured">
                    <button
                        type="button"
                        role="radio"
                        aria-checked={form.hasDimensions}
                        className={`new-card-measure-btn${form.hasDimensions ? " is-on" : ""}`}
                        onClick={() => setForm((prev) => ({ ...prev, hasDimensions: true }))}
                    >
                        By size
                    </button>
                    <button
                        type="button"
                        role="radio"
                        aria-checked={!form.hasDimensions}
                        className={`new-card-measure-btn${!form.hasDimensions ? " is-on" : ""}`}
                        onClick={() => setForm((prev) => ({ ...prev, hasDimensions: false }))}
                    >
                        By quantity
                    </button>
                </div>

                <div className="job-card-panel-fields">
                    {form.hasDimensions && SIZE_FIELDS.map((f) => (
                        <Field key={f.key} field={f} form={form} setForm={setForm} />
                    ))}
                    {COMMERCIAL.map((f) => (
                        <Field key={f.key} field={f} form={form} setForm={setForm} />
                    ))}
                    {/* Either-or, never both. Which one is the job's answer, not this card's -
                        so it is shown, not asked. */}
                    <div>
                        <label className="form-control-label pp fs-12" htmlFor="card-tax">
                            {igst ? "IGST %" : "GST %"}
                        </label>
                        <input
                            id="card-tax"
                            className="form-control"
                            type="number"
                            value={form.tax}
                            onChange={(e) => setForm((prev) => ({ ...prev, tax: e.target.value }))}
                        />
                    </div>
                </div>

                {error && (
                    <span className="text-body-small" style={{ color: "var(--text-danger)" }}>
                        {error}
                    </span>
                )}

                <div className="job-card-panel-foot">
                    {/* Removing sits apart from Cancel and Save, on the other end of the row:
                        it is the one control here that destroys something, and a destructive
                        button beside the button you press every time is how it gets pressed by
                        accident.

                        Two presses, not a window.confirm - a native dialog blocks the page and
                        says nothing this label cannot say itself. The second press is a
                        different word in a different colour, so it cannot be completed by
                        double-clicking the first. */}
                    {canDeleteRow && (
                        <button
                            type="button"
                            className={`shell-btn job-card-panel-remove${confirming ? " is-confirming" : ""}`}
                            onClick={remove}
                            disabled={busy}
                        >
                            <Trash2 size={13} />
                            {busy === "removing" ? "Removing…" : confirming ? "Remove for good?" : "Remove"}
                        </button>
                    )}
                    <span className="job-card-panel-foot-spacer" />
                    <button type="button" className="shell-btn shell-btn-secondary" onClick={close} disabled={busy}>
                        Cancel
                    </button>
                    <button type="submit" className="shell-btn shell-btn-primary" disabled={busy}>
                        {busy === "saving" ? "Saving…" : "Save"}
                    </button>
                </div>
            </form>
        </div>,
        portalHost()
    );
}
