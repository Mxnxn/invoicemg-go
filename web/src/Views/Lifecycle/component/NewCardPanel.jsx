import React, { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "react-feather";

import useFlipPanel from "../../../Common/useFlipPanel";
import AssigneeDropdown from "./AssigneeDropdown";
import QuickCreateProductModal from "../../Material/component/QuickCreateProductModal";
import { lifecycleBackend } from "../lifecycle_backend";
import { notifySuccess } from "../../../global/toast";
import { emptyJobRow } from "../jobMath";
import { splitGst, igstOnly } from "../../../Common/gst";
import { isIgstJob } from "../jobTax";
import "./jobBoard.css";
import { portalHost } from "../../../Common/portalHost";

// A new card, grown out of the button that asks for it - the same motion as the card panel it
// will become once it exists.
//
// It replaces an append. The + used to push a row called "New card" straight onto the job with
// no product, no rate and no tax, which is a real line on a real job that contributes nothing
// to its total and has to be found and fixed later. A card is a thing someone is charged for;
// the moment to say what it is is the moment it is made.
//
// Size OR quantity, never both on screen. A row carries `hasDimensions`, and a by-quantity row
// that still shows length and width invites someone to fill them in - which measures the row
// two ways and produces a total that agrees with neither. The toggle is the field; the inputs
// follow it.
//
// Tax is entered as ONE number and stored split, because that is how every other row in this
// app stores it: cgst + sgst for an intrastate sale, igst whole for an interstate one.
// GstReport reads the split per row, so putting a combined rate on cgst alone would charge the
// customer correctly and misreport the filing.
//
// Which side it lands on is not asked twice. A job is interstate or it is not, and its existing
// rows already answer; only a job with no rows yet has to be asked.
export default function NewCardPanel({ job, rect, onClose, onCreated }) {
    const [materials, setMaterials] = useState([]);
    const [productOpen, setProductOpen] = useState(false);
    const [productSeed, setProductSeed] = useState("");
    const [form, setForm] = useState(() => ({
        material: "",
        description: "",
        hasDimensions: true,
        length: "",
        width: "",
        qty: "1",
        rate: "",
        tax: "",
        discount: "",
        charges: "",
    }));
    const [igst, setIgst] = useState(() => isIgstJob(job?.rows || []));
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");
    const { panelRef, transform, phase, close } = useFlipPanel({ rect, onClose });

    // Whether the job's regime is already settled. With rows on it the answer is not a
    // question, and offering the choice would let one card disagree with its own job.
    const regimeSettled = (job?.rows || []).length > 0;

    useEffect(() => {
        let live = true;
        lifecycleBackend
            .lookupMaterials({})
            .then((res) => live && setMaterials(res.data || []))
            .catch(() => {});
        return () => {
            live = false;
        };
    }, []);

    const set = (patch) => setForm((prev) => ({ ...prev, ...patch }));

    // Picking a product fills in what the product knows - its rate and its tax. A typed name is
    // allowed too: not everything made has been set up as a material first.
    const pickMaterial = (id, name) => {
        const material = materials.find((m) => m._id === id);
        if (!material) return set({ material: name || "" });
        set({
            material: name || material.material_name || "",
            rate: String(Number(material.material_rate) || 0),
            tax: String(Number(material.tax) || 0),
        });
    };

    const submit = async (e) => {
        e.preventDefault();
        setError("");
        if (!String(form.material).trim()) return setError("A card needs a product.");
        if (!Number(form.qty)) return setError("A card needs a quantity.");
        if (form.hasDimensions && (!Number(form.length) || !Number(form.width))) {
            return setError("A card measured by size needs both a length and a width.");
        }
        if (!Number(form.rate)) return setError("A card needs a rate.");

        setBusy(true);
        try {
            const tax = Number(form.tax) || 0;
            const row = {
                ...emptyJobRow(form.hasDimensions),
                material: String(form.material).trim(),
                description: String(form.description).trim(),
                hasDimensions: form.hasDimensions,
                // By quantity, the two factors that are not asked for stay at the identity
                // value rowAmount multiplies by - 1, not 0, or the line would total nothing.
                length: form.hasDimensions ? form.length : "1",
                width: form.hasDimensions ? form.width : "1",
                qty: Number(form.qty) || 0,
                rate: Number(form.rate) || 0,
                discount: Number(form.discount) || 0,
                charges: Number(form.charges) || 0,
                // splitGst deliberately returns only cgst/sgst, so the igst: 0 it does not
                // clear comes from emptyJobRow above.
                ...(igst ? igstOnly(tax) : splitGst(tax)),
            };
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("rows", JSON.stringify([...(job.rows || []), row]));
            const res = await lifecycleBackend.updateJob(formData);
            notifySuccess("Card added.");
            onCreated?.(res.data);
            close();
        } catch (err) {
            setError(err?.message || "Couldn't add that card.");
            setBusy(false);
        }
    };

    const materialOptions = materials.map((m) => ({ id: m._id, name: m.material_name }));

    return createPortal(
        <>
            <div
                ref={panelRef}
                className={`job-card-panel is-${phase}`}
                style={{ transform }}
                role="dialog"
                aria-label="New card"
            >
                <div className="job-board-overlay-head">
                    <span className="job-board-overlay-meta">
                        <span className="text-heading-brand">New card</span>
                        <span className="text-body-small">{job?.challanNumber || ""}</span>
                    </span>
                    <button type="button" className="shell-icon-btn" aria-label="Close" onClick={close}>
                        <X size={16} />
                    </button>
                </div>

                <form onSubmit={submit} className="job-card-panel-body">
                    <div className="job-card-panel-assign">
                        <label className="form-control-label pp fs-12">Product</label>
                        <AssigneeDropdown
                            value={form.material}
                            placeholder="Product"
                            options={materialOptions}
                            onSelect={(id, name) => pickMaterial(id, name)}
                            onCreateNew={(typed) => {
                                setProductSeed(typed);
                                setProductOpen(true);
                            }}
                            fullWidth
                        />
                    </div>

                    {/* The toggle IS the measurement. Showing length and width on a card
                        counted by the piece invites someone to fill them in. */}
                    <div className="new-card-measure" role="radiogroup" aria-label="How this card is measured">
                        <button
                            type="button"
                            role="radio"
                            aria-checked={form.hasDimensions}
                            className={`new-card-measure-btn${form.hasDimensions ? " is-on" : ""}`}
                            onClick={() => set({ hasDimensions: true })}
                        >
                            By size
                        </button>
                        <button
                            type="button"
                            role="radio"
                            aria-checked={!form.hasDimensions}
                            className={`new-card-measure-btn${!form.hasDimensions ? " is-on" : ""}`}
                            onClick={() => set({ hasDimensions: false })}
                        >
                            By quantity
                        </button>
                    </div>

                    <div className="job-card-panel-fields">
                        <div className="is-wide">
                            <label className="form-control-label pp fs-12" htmlFor="new-card-description">
                                Description
                            </label>
                            <input
                                id="new-card-description"
                                className="form-control"
                                value={form.description}
                                onChange={(e) => set({ description: e.target.value })}
                            />
                        </div>

                        {form.hasDimensions && (
                            <>
                                <div>
                                    <label className="form-control-label pp fs-12" htmlFor="new-card-length">
                                        Length
                                    </label>
                                    <input
                                        id="new-card-length"
                                        className="form-control"
                                        value={form.length}
                                        onChange={(e) => set({ length: e.target.value })}
                                    />
                                </div>
                                <div>
                                    <label className="form-control-label pp fs-12" htmlFor="new-card-width">
                                        Width
                                    </label>
                                    <input
                                        id="new-card-width"
                                        className="form-control"
                                        value={form.width}
                                        onChange={(e) => set({ width: e.target.value })}
                                    />
                                </div>
                            </>
                        )}

                        <div>
                            <label className="form-control-label pp fs-12" htmlFor="new-card-qty">
                                Quantity
                            </label>
                            <input
                                id="new-card-qty"
                                className="form-control"
                                type="number"
                                value={form.qty}
                                onChange={(e) => set({ qty: e.target.value })}
                            />
                        </div>

                        <div>
                            <label className="form-control-label pp fs-12" htmlFor="new-card-rate">
                                Rate
                            </label>
                            <input
                                id="new-card-rate"
                                className="form-control"
                                type="number"
                                value={form.rate}
                                onChange={(e) => set({ rate: e.target.value })}
                            />
                        </div>

                        <div>
                            <label className="form-control-label pp fs-12" htmlFor="new-card-tax">
                                {igst ? "IGST %" : "GST %"}
                            </label>
                            <input
                                id="new-card-tax"
                                className="form-control"
                                type="number"
                                value={form.tax}
                                onChange={(e) => set({ tax: e.target.value })}
                            />
                        </div>

                        <div>
                            <label className="form-control-label pp fs-12" htmlFor="new-card-discount">
                                Discount
                            </label>
                            <input
                                id="new-card-discount"
                                className="form-control"
                                type="number"
                                value={form.discount}
                                onChange={(e) => set({ discount: e.target.value })}
                            />
                        </div>

                        <div>
                            <label className="form-control-label pp fs-12" htmlFor="new-card-charges">
                                Charges
                            </label>
                            <input
                                id="new-card-charges"
                                className="form-control"
                                type="number"
                                value={form.charges}
                                onChange={(e) => set({ charges: e.target.value })}
                            />
                        </div>
                    </div>

                    {/* Asked only by a job with no rows to answer it. */}
                    {!regimeSettled && (
                        <label className="new-card-igst text-body-small">
                            <input type="checkbox" checked={igst} onChange={(e) => setIgst(e.target.checked)} />
                            Interstate sale — IGST rather than CGST + SGST
                        </label>
                    )}

                    {error && (
                        <span className="text-body-small" style={{ color: "var(--text-danger)" }}>
                            {error}
                        </span>
                    )}

                    <div className="job-card-panel-foot">
                        <button type="button" className="shell-btn shell-btn-secondary" onClick={close} disabled={busy}>
                            Cancel
                        </button>
                        <button type="submit" className="shell-btn shell-btn-primary" disabled={busy}>
                            {busy ? "Adding…" : "Add card"}
                        </button>
                    </div>
                </form>
            </div>

            {productOpen && (
                <QuickCreateProductModal
                    isOpen={productOpen}
                    initialName={productSeed}
                    toggle={() => setProductOpen(false)}
                    onCreated={(material) => {
                        // Selected straight into the form. `materials` has not re-rendered yet,
                        // so the patch is applied here rather than through pickMaterial.
                        setMaterials((prev) => [...prev, material]);
                        set({
                            material: material.material_name || "",
                            rate: String(Number(material.material_rate) || 0),
                            tax: String(Number(material.tax) || 0),
                        });
                        setProductOpen(false);
                    }}
                />
            )}
        </>,
        portalHost()
    );
}
