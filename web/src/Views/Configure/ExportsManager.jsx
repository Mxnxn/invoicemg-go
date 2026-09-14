import { useEffect, useMemo, useState } from "react";
import { Image as ImageIcon, Briefcase, MapPin, Phone, Hash, Calendar, Check } from "react-feather";

import { companyBackend } from "../../Common/company_backend";
import { notifySuccess, notifyError } from "../../global/toast";
import SaveBar from "./SaveBar";
import "./exportsManager.css";

// Only company identity lives here - what the letterhead says. Which columns a report
// exports stays in the report's own code: a settings matrix of every report against every
// column would sit far from the Download button, where the question actually arises.
// An icon each, because these are six things that appear on a piece of paper rather than six
// settings - the icon is what makes the row scannable at a glance.
const FIELDS = [
    { key: "logo", label: "Logo", Icon: ImageIcon },
    { key: "firm", label: "Firm name", Icon: Briefcase },
    { key: "address", label: "Address", Icon: MapPin },
    { key: "phone", label: "Phone", Icon: Phone },
    { key: "gst", label: "GST number", Icon: Hash },
    { key: "showPeriod", label: "Report period", Icon: Calendar },
];

const DEFAULTS = { logo: true, firm: true, address: true, phone: true, gst: true, showPeriod: true };

export default function ExportsManager() {
    const [companyId, setCompanyId] = useState(null);
    const [company, setCompany] = useState(null);
    const [saved, setSaved] = useState(DEFAULTS);
    const [template, setTemplate] = useState(DEFAULTS);
    const [hasLogo, setHasLogo] = useState(true);
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        companyBackend
            .activeCompany()
            .then((res) => {
                setCompanyId(res.data?._id || null);
                setCompany(res.data || null);
                const stored = { ...DEFAULTS, ...(res.data?.exportTemplate || {}) };
                setSaved(stored);
                setTemplate(stored);
                setHasLogo(Boolean(res.data?.url));
            })
            .catch(() => {})
            .finally(() => setLoading(false));
    }, []);

    const dirty = useMemo(() => FIELDS.some(({ key }) => Boolean(saved[key]) !== Boolean(template[key])), [saved, template]);

    const save = async () => {
        // activeCompany() failed silently above (its .catch swallows the error) and left
        // companyId null - posting "null" as company_id would pass the API's truthy check
        // and fail as a cast error instead of a clean refusal.
        if (!companyId) return;
        setSaving(true);
        try {
            const formData = new FormData();
            // /company/update requires company_id (422 without it) - the active company's own
            // id, not the session's default, since a tab can be acting as a non-default company.
            formData.set("company_id", companyId);
            // All six keys, every time: the API's exportTemplate guard is all-or-nothing per
            // request, so posting only the changed key would reset every other key to default.
            FIELDS.forEach(({ key }) => formData.set(`exportTemplate[${key}]`, String(Boolean(template[key]))));
            const res = await companyBackend.updateCompany(formData);
            if (res.code === 200) {
                setSaved(template);
                notifySuccess("Export settings saved.");
            }
        } catch (error) {
            notifyError(error.message || "Could not save.");
        } finally {
            setSaving(false);
        }
    };

    if (loading) return <p className="dev-muted">Loading…</p>;

    // What the chosen switches actually produce, in the order the spreadsheet writes them.
    // The toggles are a cause; this is the effect, and the effect is what the customer sees.
    const headerLines = [
        template.firm && (company?.firm || "Your firm"),
        template.address && company?.address,
        template.phone && company?.phone,
        template.gst && company?.gst && `GST ${company.gst}`,
    ].filter(Boolean);

    return (
        <div className="design-layout">
            <div className="design-list">
                {/* Buttons, not checkboxes. Each one is a thing that either appears on the
                    letterhead or does not.

                    aria-pressed, not role="checkbox": these are toggle buttons, and a screen
                    reader should announce them as pressed or not rather than as a form field
                    that never gets submitted. */}
                <div className="exports-fields" role="group" aria-label="What appears on the letterhead">
                    {FIELDS.map(({ key, label, Icon }) => {
                        const on = Boolean(template[key]);
                        const disabled = key === "logo" && !hasLogo;
                        return (
                            <button
                                key={key}
                                type="button"
                                className={`exports-chip${on ? " is-on" : ""}`}
                                aria-pressed={on}
                                disabled={disabled}
                                title={disabled ? "Upload a logo first" : undefined}
                                onClick={() => setTemplate((prev) => ({ ...prev, [key]: !prev[key] }))}
                            >
                                <span className="exports-chip-icon">
                                    <Icon size={15} aria-hidden="true" />
                                </span>
                                <span className="exports-chip-label">{label}</span>
                                {/* The tick is the state, so it only exists when the state is
                                    on - a permanently visible empty box is what made six of
                                    these read as a form rather than as a preview. */}
                                {on && <Check size={13} className="exports-chip-tick" aria-hidden="true" />}
                            </button>
                        );
                    })}
                </div>
            </div>

            <div className="design-preview">
                <div className="shell-card">
                    <div className="shell-card-header">
                        <span className="text-heading-brand">Spreadsheet header</span>
                    </div>
                    <div className="settings-pane">
                        <p className="text-body-small settings-intro">
                            What appears at the top of every spreadsheet (XLSX) export this company downloads. PDF exports
                            keep their own fixed letterhead and are not affected by these settings.
                        </p>

                        {!hasLogo && (
                            <p className="text-body-small settings-intro">
                                No logo uploaded for this company — the header will show text only.
                            </p>
                        )}

                        <div className="settings-preview-block exports-sheet">
                            {template.logo && hasLogo && <span className="exports-sheet-logo">Logo</span>}
                            {headerLines.length > 0 ? (
                                headerLines.map((line, i) => (
                                    <span key={i} className={i === 0 ? "exports-sheet-firm" : "exports-sheet-line"}>
                                        {line}
                                    </span>
                                ))
                            ) : (
                                <span className="exports-sheet-line">No header — the sheet starts at the data.</span>
                            )}
                            {template.showPeriod && <span className="exports-sheet-line">Period: 1 Apr 2026 – 30 Apr 2026</span>}
                        </div>
                    </div>
                </div>

                <SaveBar
                    dirty={dirty}
                    saving={saving}
                    disabled={!companyId}
                    onSave={save}
                    onDiscard={() => setTemplate(saved)}
                    note="Letterhead saved."
                />
            </div>
        </div>
    );
}
