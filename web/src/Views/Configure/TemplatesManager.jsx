import React, { useEffect, useState } from "react";
import { Check, X, Copy } from "react-feather";

import { userBackend } from "../UserProfile/user_backend";
import { notifyError, notifySuccess } from "../../global/toast";
import { INVOICE_TEMPLATES } from "../Invoice/Template/registry";
import { QUOTATION_TEMPLATES } from "../Quotation/registry";
import { LEDGER_TEMPLATES } from "../Ledger/registry";
import { resolveDesignKey } from "../../Common/pdf/designs";
import { DEMO_INVOICE, DEMO_QUOTATION, DEMO_USER, DEMO_LEDGER } from "./templatePreviewDemoData";
import TemplatePreviewPanel from "./TemplatePreviewPanel";
import DocumentFontBar from "./DocumentFontBar";
import NumberingManager from "./NumberingManager";
import ExportsManager from "./ExportsManager";
import "./templatesManager.css";

const SECTIONS = [
    {
        docType: "invoice",
        label: "Invoice",
        templates: INVOICE_TEMPLATES,
        field: "invoiceTemplate",
        componentProps: { invoice: DEMO_INVOICE },
    },
    {
        docType: "quotation",
        label: "Quotation",
        templates: QUOTATION_TEMPLATES,
        field: "quotationTemplate",
        componentProps: { quotation: DEMO_QUOTATION, user: DEMO_USER },
    },
    {
        docType: "ledger",
        label: "Client Ledger",
        templates: LEDGER_TEMPLATES,
        field: "ledgerTemplate",
        componentProps: { ledger: DEMO_LEDGER, user: DEMO_USER, from: null, to: null },
    },
];

// One design choice per document type, applied to every invoice/quotation/ledger this Company
// generates from here on (not a per-send override) - see Company.invoiceTemplate and friends,
// and Common/pdf/designs.js for the designs themselves.
//
// The three tabs list the SAME six names, because a design is data now rather than a
// component per document type. That is what makes "Use for all three" meaningful: it is one
// design being applied, not three lookalikes being kept in step by hand.
//
// Selecting a design doesn't touch anything already generated. Hovering (or selecting) an
// option previews it live against demo data - see templatePreviewDemoData.js.
const TemplatesManager = () => {
    const [profile, setProfile] = useState(null);
    const [saving, setSaving] = useState("");
    const [activeDocType, setActiveDocType] = useState("invoice");
    const [previewKeys, setPreviewKeys] = useState({});
    // Picking a card used to save instantly, so a stray click silently changed the design
    // for every future document. A pick is now staged until it is confirmed.
    const [pending, setPending] = useState(null);
    // "Use for all three" overwrites two choices the user cannot see from here, so it asks.
    const [confirmAll, setConfirmAll] = useState(false);

    const load = () => {
        const formData = new FormData();
        formData.set("uid", window.localStorage.getItem("uid"));
        userBackend.getUserInfo(formData, window.localStorage.getItem("session_token")).then((res) => setProfile(res.data));
    };

    useEffect(() => {
        load();
    }, []);

    const select = async (docType, key) => {
        setSaving(`${docType}:${key}`);
        if (docType !== "all") setPreviewKeys((prev) => ({ ...prev, [docType]: key }));
        try {
            const res = await userBackend.setTemplate(docType, key, window.localStorage.getItem("session_token"));
            setProfile(res.data);
            if (docType === "all") notifySuccess("Applied to invoices, quotations and ledgers.");
        } catch (err) {
            notifyError(err.message || "Couldn't update the template.");
        } finally {
            setSaving("");
        }
    };

    const selectScale = async (scaleId) => {
        setSaving("scale");
        // Optimistic like the font, and for the same reason: the live preview re-renders at the
        // new size immediately rather than after the round trip.
        setProfile((prev) => ({ ...prev, documentScale: scaleId }));
        try {
            const res = await userBackend.setTemplate(
                null,
                null,
                window.localStorage.getItem("session_token"),
                null,
                scaleId
            );
            setProfile(res.data);
            notifySuccess("Document text size updated.");
        } catch (err) {
            notifyError(err.message || "Couldn't update the text size.");
        } finally {
            setSaving(null);
        }
    };

    const selectFont = async (fontKey) => {
        setSaving("font");
        // Optimistic so the live preview re-renders in the new face as soon as it is picked,
        // rather than after the round trip. The response overwrites it either way.
        setProfile((prev) => ({ ...prev, documentFont: fontKey }));
        try {
            const res = await userBackend.setTemplate(null, null, window.localStorage.getItem("session_token"), fontKey);
            setProfile(res.data);
            notifySuccess("Document font updated.");
        } catch (err) {
            notifyError(err.message || "Couldn't update the font.");
            load();
        } finally {
            setSaving("");
        }
    };

    if (!profile) return null;

    // Falls back rather than returning undefined: the Numbering and Export configs tabs are
    // not in SECTIONS, and the lines below dereference this before the render reaches the
    // conditional. Their values are unused on those tabs.
    const section = SECTIONS.find((s) => s.docType === activeDocType) || SECTIONS[0];
    // Resolved, not read raw: a company configured before the rename still has "gst-detailed"
    // stored. The document renders correctly either way, but comparing the raw value against
    // the card keys highlighted nothing, so the tab claimed no design was chosen at all.
    const savedKey = resolveDesignKey(profile[section.field]);
    const previewKey = (pending && pending.docType === section.docType && pending.key) || previewKeys[section.docType] || savedKey;
    const previewTemplate = section.templates.find((t) => t.key === previewKey) || section.templates[0];
    const fontKey = profile.documentFont || "open-sans";
    const scaleId = profile.documentScale || "normal";

    // True when all three document types are already on the same design, which is the only
    // time "Use for all three" has nothing to do.
    const allMatch = SECTIONS.every((s) => resolveDesignKey(profile[s.field]) === savedKey);

    return (
        <div>
            {/* Above the tab strip because it is not a property of any one document type -
                one font covers invoices, quotations and ledgers. */}
            <DocumentFontBar
                value={fontKey}
                onChange={selectFont}
                saving={saving === "font"}
                scale={scaleId}
                onScaleChange={selectScale}
                savingScale={saving === "scale"}
            />

            <div className="shell-segmented" role="tablist" style={{ marginBottom: 16 }}>
                {/* Numbering is a peer of the three design pickers, not a card stacked under
                    them: it answers the same question about a different part of the document,
                    and stacking it meant scrolling past a PDF preview to reach it. */}
                {/* Export configs joins Numbering as a peer of the design pickers: a
                    letterhead on a downloaded report is the same "how does our paperwork look"
                    question the three design tabs answer, so it belongs beside them rather
                    than in a card of its own on the Configure grid. */}
                {[...SECTIONS, { docType: "numbering", label: "Numbering" }, { docType: "exports", label: "Export configs" }].map(
                    (s) => (
                        <button
                            key={s.docType}
                            type="button"
                            role="tab"
                            aria-selected={s.docType === activeDocType}
                            className="shell-segmented-btn"
                            style={s.docType === activeDocType ? { background: "var(--xan-blue-bg)", color: "var(--xan-blue)" } : undefined}
                            onClick={() => {
                                setActiveDocType(s.docType);
                                setPending(null);
                                setConfirmAll(false);
                            }}
                        >
                            {s.label}
                        </button>
                    )
                )}
            </div>

            {activeDocType === "exports" ? (
                <ExportsManager />
            ) : activeDocType === "numbering" ? (
                <NumberingManager />
            ) : (
                <div className="design-layout">
                    <div className="design-list">
                        {pending && pending.docType === section.docType && pending.key !== savedKey && (
                            <div className="template-confirm">
                                <span className="template-confirm-label">
                                    Apply <strong>{(section.templates.find((t) => t.key === pending.key) || {}).label}</strong>?
                                </span>
                                <span className="template-confirm-actions">
                                    <button
                                        type="button"
                                        className="template-confirm-btn is-yes"
                                        aria-label="Confirm design"
                                        disabled={!!saving}
                                        onClick={() => {
                                            select(pending.docType, pending.key);
                                            setPending(null);
                                        }}
                                    >
                                        <Check size={16} />
                                    </button>
                                    <button
                                        type="button"
                                        className="template-confirm-btn is-no"
                                        aria-label="Cancel design change"
                                        disabled={!!saving}
                                        onClick={() => {
                                            setPending(null);
                                            setPreviewKeys((prev) => ({ ...prev, [section.docType]: savedKey }));
                                        }}
                                    >
                                        <X size={16} />
                                    </button>
                                </span>
                            </div>
                        )}

                        {section.templates.map((tpl) => {
                            const active = savedKey === tpl.key;
                            const isSaving = saving === `${section.docType}:${tpl.key}`;
                            return (
                                <button
                                    key={tpl.key}
                                    type="button"
                                    onClick={() => setPending({ docType: section.docType, key: tpl.key })}
                                    onMouseEnter={() => setPreviewKeys((prev) => ({ ...prev, [section.docType]: tpl.key }))}
                                    onFocus={() => setPreviewKeys((prev) => ({ ...prev, [section.docType]: tpl.key }))}
                                    disabled={isSaving}
                                    aria-pressed={active}
                                    className={`design-card${active ? " is-active" : ""}${isSaving ? " is-saving" : ""}`}
                                >
                                    <span className="design-card-head">
                                        <span className="design-card-name text-body-regular">{tpl.label}</span>
                                        {/* currentColor so the tick follows the label - white
                                            on an active dark-theme card. */}
                                        {active && <Check size={15} color="currentColor" aria-hidden="true" />}
                                    </span>
                                    <span className="design-card-blurb text-body-small">{tpl.blurb}</span>
                                </button>
                            );
                        })}
                    </div>

                    <div className="design-preview">
                        {/* Keyed so switching designs or fonts fully remounts the PDFViewer
                            instead of swapping its Document tree in place - react-pdf's
                            internal layout engine crashes (blank preview) if a live viewer's
                            document is hot-swapped, same issue fixed in Invoice/Template/Main.js. */}
                        <TemplatePreviewPanel
                            key={`${section.docType}-${previewTemplate.key}-${fontKey}-${scaleId}`}
                            Component={previewTemplate.Component}
                            componentProps={{ ...section.componentProps, fontKey, scaleId }}
                            label={previewTemplate.label}
                        />

                        <div className="design-apply-all">
                            {confirmAll ? (
                                <>
                                    <span className="text-body-small design-apply-all-text">
                                        Set <strong>{(section.templates.find((t) => t.key === savedKey) || {}).label}</strong> on
                                        invoices, quotations and ledgers?
                                    </span>
                                    <button
                                        type="button"
                                        className="shell-btn shell-btn-sm"
                                        disabled={!!saving}
                                        onClick={() => {
                                            select("all", savedKey);
                                            setConfirmAll(false);
                                        }}
                                    >
                                        {saving === `all:${savedKey}` ? "Applying…" : "Yes, apply"}
                                    </button>
                                    <button
                                        type="button"
                                        className="shell-btn shell-btn-sm shell-btn-secondary"
                                        disabled={!!saving}
                                        onClick={() => setConfirmAll(false)}
                                    >
                                        Cancel
                                    </button>
                                </>
                            ) : (
                                <>
                                    <span className="text-body-small design-apply-all-text">
                                        {allMatch
                                            ? "All three documents already use this design."
                                            : "Invoices, quotations and ledgers can share one design."}
                                    </span>
                                    <button
                                        type="button"
                                        className="shell-btn shell-btn-sm shell-btn-secondary design-apply-all-btn"
                                        disabled={!!saving || allMatch}
                                        onClick={() => setConfirmAll(true)}
                                    >
                                        <Copy size={13} aria-hidden="true" />
                                        Use for all three
                                    </button>
                                </>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};

export default TemplatesManager;
