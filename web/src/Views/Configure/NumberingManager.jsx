import React, { useEffect, useMemo, useState } from "react";
import { Input } from "reactstrap";

import { companyBackend } from "../../Common/company_backend";
import { notifySuccess, notifyError } from "../../global/toast";
import SaveBar from "./SaveBar";

const KINDS = [
    { id: "invoice", label: "Invoices", hint: "Raised against your customers" },
    { id: "job", label: "Job-ids", hint: "Work taken in" },
    { id: "quotation", label: "Quotations", hint: "Estimates sent out" },
    { id: "purchase", label: "Purchase invoices", hint: "Suggested — the field stays editable" },
];

const YEARS = [
    { id: "fy", label: "Financial year (26-27)" },
    { id: "yy", label: "Short year (26)" },
    { id: "yyyy", label: "Full year (2026)" },
    { id: "none", label: "No year" },
];

// Built here as well as on the server so the sample updates as you type, rather than only
// after saving. The server is still what numbers a document.
const preview = (f) => {
    if (!f) return "";
    const year = f.year === "fy" ? "26-27" : f.year === "yy" ? "26" : f.year === "yyyy" ? "2026" : "";
    const parts = [f.prefix, year].filter(Boolean);
    const seq = "1".padStart(Math.min(8, Math.max(1, Number(f.pad) || 1)), "0");
    return parts.length ? `${parts.join(f.separator)}${f.separator}${seq}` : seq;
};

// How documents are numbered, per company.
//
// The scheme was hardcoded as MG/<FY>/<code><00001> - "MG" being one firm's initials. A
// business arriving from other software usually has a numbering habit its customers and its
// accountant already recognise, and asking them to abandon it is not reasonable.
//
// Changing a format never renumbers anything already issued; the next document simply
// continues from the highest number so far, under the new prefix.
//
// Laid out as list-and-panel to match the three design tabs beside it, rather than as four
// stacked rows each with its own Save button: the four kinds are one setting seen four ways,
// and four save buttons made it possible to edit three of them and save one.
const NumberingManager = () => {
    const [saved, setSaved] = useState(null);
    const [formats, setFormats] = useState(null);
    const [activeKind, setActiveKind] = useState("invoice");
    const [saving, setSaving] = useState(false);

    const load = () =>
        companyBackend
            .getNumbering()
            .then((res) => {
                setSaved(res.data.numbering);
                setFormats(res.data.numbering);
            })
            .catch(() => {});

    useEffect(() => {
        load();
    }, []);

    const patch = (kind, key, value) => setFormats((prev) => ({ ...prev, [kind]: { ...prev[kind], [key]: value } }));

    // Which kinds actually differ from what is stored - the set that a save has to write, and
    // the reason the bar is showing.
    const changedKinds = useMemo(() => {
        if (!saved || !formats) return [];
        return KINDS.map((k) => k.id).filter((id) => JSON.stringify(saved[id]) !== JSON.stringify(formats[id]));
    }, [saved, formats]);

    // A prefix is the one field the server rejects empty, so an empty one blocks the save
    // rather than failing after the round trip.
    const invalidKinds = changedKinds.filter((id) => !String(formats[id]?.prefix ?? "").trim());

    const save = async () => {
        setSaving(true);
        try {
            // One request per changed kind, because that is the endpoint's shape; awaited
            // together so a slow one does not serialise the rest.
            const results = await Promise.all(changedKinds.map((kind) => companyBackend.updateNumbering(kind, formats[kind])));
            setSaved(formats);
            const count = results.length;
            notifySuccess(`Numbering updated for ${count} document type${count === 1 ? "" : "s"}.`);
        } catch (err) {
            notifyError(err.message || "Could not save the numbering.");
            // The stored values are now unknown - one write may have landed - so re-read
            // rather than leaving the form claiming to match the server.
            load();
        } finally {
            setSaving(false);
        }
    };

    if (!formats) return null;

    const active = formats[activeKind];
    const activeKindMeta = KINDS.find((k) => k.id === activeKind);

    return (
        <div className="design-layout">
            <div className="design-list">
                {KINDS.map(({ id, label, hint }) => {
                    const isActive = id === activeKind;
                    const isChanged = changedKinds.includes(id);
                    return (
                        <button
                            key={id}
                            type="button"
                            onClick={() => setActiveKind(id)}
                            aria-pressed={isActive}
                            className={`design-card${isActive ? " is-active" : ""}`}
                        >
                            <span className="design-card-head">
                                <span className="design-card-name text-body-regular">{label}</span>
                                {/* A dot rather than the word "edited": four of these in a
                                    column need to be countable at a glance, not read. */}
                                {isChanged && <span className="settings-dot" aria-label="Edited" />}
                            </span>
                            <span className="design-card-blurb text-body-small">{hint}</span>
                            <span className="cell-mono numbering-preview settings-sample">{preview(formats[id])}</span>
                        </button>
                    );
                })}
            </div>

            <div className="design-preview">
                <div className="shell-card">
                    <div className="shell-card-header">
                        <span className="text-heading-brand">{activeKindMeta.label}</span>
                    </div>
                    <div className="settings-pane">
                        <p className="text-body-small settings-intro">
                            Numbers already issued keep what they have. A new format takes effect on the next document,
                            carrying the count forward so nothing collides.
                        </p>

                        <div className="numbering-fields">
                            <div>
                                <label className="form-control-label pp fs-12" htmlFor="numbering-prefix">
                                    Prefix
                                    {!String(active.prefix ?? "").trim() && <span className="required-star">*</span>}
                                </label>
                                <Input
                                    id="numbering-prefix"
                                    value={active.prefix}
                                    maxLength={12}
                                    className={!String(active.prefix ?? "").trim() ? "is-required-missing" : undefined}
                                    onChange={(e) => patch(activeKind, "prefix", e.target.value)}
                                />
                                {!String(active.prefix ?? "").trim() && (
                                    <span className="field-error">Prefix is required</span>
                                )}
                            </div>
                            <div>
                                <label className="form-control-label pp fs-12" htmlFor="numbering-year">
                                    Year
                                </label>
                                <Input
                                    id="numbering-year"
                                    type="select"
                                    value={active.year}
                                    onChange={(e) => patch(activeKind, "year", e.target.value)}
                                >
                                    {YEARS.map((y) => (
                                        <option key={y.id} value={y.id}>
                                            {y.label}
                                        </option>
                                    ))}
                                </Input>
                            </div>
                            <div>
                                <label className="form-control-label pp fs-12" htmlFor="numbering-pad">
                                    Digits
                                </label>
                                <Input
                                    id="numbering-pad"
                                    type="number"
                                    min={1}
                                    max={8}
                                    value={active.pad}
                                    onChange={(e) => patch(activeKind, "pad", e.target.value)}
                                />
                            </div>
                            <div>
                                <label className="form-control-label pp fs-12" htmlFor="numbering-separator">
                                    Separator
                                </label>
                                <Input
                                    id="numbering-separator"
                                    value={active.separator}
                                    maxLength={2}
                                    onChange={(e) => patch(activeKind, "separator", e.target.value)}
                                />
                            </div>
                        </div>

                        {/* The sample is the point of the whole screen - it is what tells you
                            the format is right, so it is the largest thing on the panel. */}
                        <div className="settings-preview-block">
                            <span className="text-body-small settings-preview-label">Next number will look like</span>
                            <span className="cell-mono settings-preview-value">{preview(active)}</span>
                        </div>
                    </div>
                </div>

                <SaveBar
                    dirty={changedKinds.length > 0}
                    saving={saving}
                    disabled={invalidKinds.length > 0}
                    onSave={save}
                    onDiscard={() => setFormats(saved)}
                    note="Every document type is saved."
                />
            </div>
        </div>
    );
};

export default NumberingManager;
