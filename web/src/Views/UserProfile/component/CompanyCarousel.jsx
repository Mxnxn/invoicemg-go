import React, { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, Check, Image, Save, Upload } from "react-feather";
import { companyBackend } from "../../../Common/company_backend";
import useRequiredFields from "../../../Common/useRequiredFields";
import DetailFields from "./DetailFields";
import "./companyCarousel.css";

const FIELDS = [
    ["name", "Company Name", "Shown in the switcher"],
    ["firm", "Firm", "Printed on invoices"],
    ["gst", "GST", ""],
    ["phone", "Phone", ""],
    ["address", "Address", ""],
    ["account_no", "Account No", ""],
    ["ifsc", "IFSC", ""],
    ["bank_name", "Bank", ""],
];

const EMPTY = { name: "", firm: "", gst: "", phone: "", address: "", account_no: "", ifsc: "", bank_name: "" };

// Where the API serves uploaded files - the same path the invoice templates build.
const UPLOADS_BASE = `${import.meta.env.VITE_API_URL}/uploads`;

// Every company on one card at a time, edited in place.
//
// Stepped through rather than listed: a list would push Appearance off the side of the screen
// the moment a second company existed, and these are profiles you read one at a time - you
// are checking one firm's bank details, not comparing four.
//
// Edited in place rather than behind a modal, the way the Configure screens work: the form is
// the view, and a border marks it as live once something has been changed. The sidebar
// switcher changes which company you ACT as; this only shows and edits them.
const CompanyCarousel = () => {
    const [companies, setCompanies] = useState([]);
    const [index, setIndex] = useState(0);
    const [activeId, setActiveId] = useState("");
    const [form, setForm] = useState(EMPTY);
    const [dirty, setDirty] = useState(false);
    const [saving, setSaving] = useState(false);
    // Whether this admin may own another company at all. The server decides and still refuses
    // Logo and UPI QR used to live on the Profile form, which always wrote to the ACTIVE
    // company. Here they belong to the card you are looking at, like every other field.
    const [uploading, setUploading] = useState("");
    const logoInput = useRef(null);
    const qrInput = useRef(null);

    const required = useRequiredFields(form, { name: "Company name", phone: "Phone number" });

    // Same shape the sidebar switcher reads: { companies, active_company_id }. The list is
    // not the payload itself, which is what a first pass assumed.
    const load = (keepIndex = true) => {
        companyBackend
            .listCompanies()
            .then((res) => {
                const list = res.data?.companies || [];
                setCompanies(list);
                setActiveId(String(res.data?.active_company_id || ""));
                if (!keepIndex) setIndex(Math.max(0, list.length - 1));
            })
            .catch(() => setCompanies([]));
    };

    useEffect(() => load(), []);

    // A company removed elsewhere would otherwise leave the index past the end.
    const safeIndex = companies.length === 0 ? 0 : Math.min(index, companies.length - 1);
    const company = companies[safeIndex];

    // Reload the form whenever the card changes - but never over an edit in progress, which
    // would silently discard what was typed.
    useEffect(() => {
        if (dirty) return;
        setForm(company ? { ...EMPTY, ...company } : EMPTY);
    }, [company?._id]); // eslint-disable-line react-hooks/exhaustive-deps

    const onChange = (e) => {
        setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }));
        setDirty(true);
    };

    const onSave = async () => {
        if (!required.isComplete) {
            required.showAll();
            return;
        }
        setSaving(true);
        try {
            const formData = new FormData();
            if (company?._id) formData.set("company_id", company._id);
            FIELDS.forEach(([key]) => formData.set(key, form[key] || ""));
            await (company?._id ? companyBackend.updateCompany(formData) : companyBackend.createCompany(formData));
            setDirty(false);
            load(!!company?._id);
        } catch (error) {
            // The global interceptor raises the toast.
        } finally {
            setSaving(false);
        }
    };

    const onPickImage = async (field, file) => {
        if (!file || !company?._id) return;
        setUploading(field);
        try {
            const formData = new FormData();
            formData.set("company_id", company._id);
            formData.append(field, file);
            await companyBackend.updateCompany(formData);
            load();
        } catch (error) {
            // The global interceptor raises the toast.
        } finally {
            setUploading("");
        }
    };

    // `logo` is the form field name; the server stores it as `url`, which is what the invoice
    // templates read.
    const imageField = (field, src, label, inputRef) => (
        <div className="profile-upload">
            <div className="profile-upload-thumb">{src ? <img src={src} alt={label} /> : <Image size={18} />}</div>
            <div className="profile-upload-main">
                <div className="text-label-caps company-carousel-label">{label}</div>
                <button
                    type="button"
                    className="shell-btn shell-btn-secondary"
                    onClick={() => inputRef.current && inputRef.current.click()}
                    disabled={Boolean(uploading) || saving}
                >
                    <Upload size={13} /> {uploading === field ? "Uploading…" : src ? "Replace" : "Upload"}
                </button>
                {/* Hidden, so the button is the whole control - a file input styles badly and
                    says "No file chosen" next to itself forever. */}
                <input
                    ref={inputRef}
                    type="file"
                    accept="image/png,image/jpg,image/jpeg,image/webp,image/gif,image/avif"
                    hidden
                    onChange={(e) => {
                        onPickImage(field, e.target.files && e.target.files[0]);
                        // Reset, or choosing the same file twice fires no change event.
                        e.target.value = "";
                    }}
                />
            </div>
        </div>
    );

    const step = (delta) => {
        if (dirty && !window.confirm("Discard the unsaved changes to this company?")) return;
        setDirty(false);
        setIndex((i) => (companies.length ? (i + delta + companies.length) % companies.length : 0));
    };

    return (
        <div className="shell-card company-carousel">
            <div className="shell-card-header">
                <span className="text-heading-brand">{"Company profiles"}</span>
                {/* Stepping between companies belongs beside the title, not down in the
                    actions row with Save and Remove - it moves you between records rather
                    than doing anything to the one on screen. */}
                <span className="company-carousel-head-nav">
                    <span className="company-carousel-count text-body-small">
                        {companies.length ? `${safeIndex + 1} of ${companies.length}` : "None yet"}
                    </span>
                <span className="company-carousel-nav">
                    <button
                        type="button"
                        className="shell-icon-btn"
                        aria-label="Previous company"
                        onClick={() => step(-1)}
                        disabled={companies.length < 2}
                    >
                        <ChevronLeft size={16} />
                    </button>
                    {/* Dots are a position indicator and a jump target - with three or four
                        companies, stepping past the one you want is annoying. */}
                    <span className="company-carousel-dots">
                        {companies.map((c, i) => (
                            <button
                                key={c._id}
                                type="button"
                                className={["company-carousel-dot", i === safeIndex ? "is-on" : ""].filter(Boolean).join(" ")}
                                aria-label={`Show ${c.name}`}
                                onClick={() => {
                                    if (dirty && !window.confirm("Discard the unsaved changes to this company?")) return;
                                    setDirty(false);
                                    setIndex(i);
                                }}
                            />
                        ))}
                    </span>
                    <button
                        type="button"
                        className="shell-icon-btn"
                        aria-label="Next company"
                        onClick={() => step(1)}
                        disabled={companies.length < 2}
                    >
                        <ChevronRight size={16} />
                    </button>
                </span>
                </span>
            </div>

            {/* The border marks the form as live once something has been typed, so an unsaved
                edit is visible rather than something you have to remember. */}
            <div className={["company-carousel-body", dirty ? "is-dirty" : ""].filter(Boolean).join(" ")}>
                {company && (
                    <div className="company-carousel-title">
                        {String(company._id) === activeId && (
                            <span className="company-carousel-badge">
                                <Check size={11} /> Active
                            </span>
                        )}
                        {company.is_default && <span className="company-carousel-badge is-muted">Default</span>}
                    </div>
                )}

                <DetailFields idPrefix="company" fields={FIELDS} form={form} required={required} onChange={onChange} />

                {/* Only once the company exists - an upload needs an id to attach to. On a
                    new company these appear as soon as it is saved. */}
                {company?._id && (
                    <div className="profile-uploads" style={{ marginTop: 12 }}>
                        {imageField("logo", company.url ? `${UPLOADS_BASE}/${company.url}` : "", "Logo", logoInput)}
                        {imageField("upiQr", company.upiQr ? `${UPLOADS_BASE}/${company.upiQr}` : "", "UPI QR", qrInput)}
                    </div>
                )}

                {/* Add and Remove are gone from here deliberately. This card edits ONE
                    company's details; creating and deleting a company is a different kind of
                    act, and sitting all three in one row made Remove a neighbour of Save on a
                    form people edit often. Save is the only thing this row does now. */}
                <div className="company-carousel-actions">
                    <button
                        type="button"
                        className="shell-btn shell-btn-primary"
                        onClick={onSave}
                        disabled={saving || !dirty || !required.isComplete}
                    >
                        <Save size={14} /> {saving ? "Saving…" : "Save"}
                    </button>
                </div>
            </div>
        </div>
    );
};

export default CompanyCarousel;
