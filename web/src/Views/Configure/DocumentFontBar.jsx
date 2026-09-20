import React from "react";
import { Type, AlertTriangle } from "react-feather";

import { FONTS } from "../../Common/pdf/fonts";
import { DOCUMENT_SIZES, DOCUMENT_SIZE_MIN, DOCUMENT_SIZE_MAX } from "../../Common/pdf/designs";

// One typeface for every document this company issues - invoices, quotations and ledgers
// alike. It sits above the tab strip rather than inside any one tab because it is not a
// property of invoices; a firm's paperwork reads as one firm's paperwork.
//
// The rupee warning is not decoration. react-pdf 1.x has no font-fallback chain, so a family
// without U+20B9 cannot borrow the glyph from another font - amounts are written "Rs 1,200"
// instead of "₹1,200". Open Sans, the default every document has always used, is one of
// those families, so saying so here is the difference between a considered choice and a
// surprise on a customer-facing page.
const DocumentFontBar = ({ value, onChange, saving, scale, onScaleChange, savingScale }) => {
    const active = FONTS.find((f) => f.key === value) || FONTS[0];
    // The size is now an absolute point value (8-18). A legacy label or an absent value has no
    // number to show, so the select falls to its "Default" option, which keeps each design's
    // own base size until a point size is picked.
    const sizeNum = Number(scale);
    const activeSize =
        Number.isFinite(sizeNum) && sizeNum >= DOCUMENT_SIZE_MIN && sizeNum <= DOCUMENT_SIZE_MAX ? String(sizeNum) : "normal";

    return (
        <div className="doc-font-bar">
            <span className="doc-font-icon" aria-hidden="true">
                <Type size={15} />
            </span>

            <label className="doc-font-label text-body-small" htmlFor="document-font">
                Document font
            </label>

            <select
                id="document-font"
                className="form-control doc-font-select"
                value={active.key}
                disabled={saving}
                onChange={(event) => onChange(event.target.value)}
            >
                {FONTS.map((font) => (
                    <option key={font.key} value={font.key}>
                        {font.label}
                    </option>
                ))}
            </select>

            <span className="doc-font-hint text-body-small">
                {saving ? "Saving…" : active.blurb}
            </span>

            {!active.rupee && (
                <span className="doc-font-warn text-body-small">
                    <AlertTriangle size={13} aria-hidden="true" />
                    No ₹ glyph — amounts print as &ldquo;Rs&rdquo;
                </span>
            )}

            {/* Beside the font, because it answers the same question about the same thing and
                is company-wide for the same reason. Each design sets its own base size - 7.5pt
                on one, 11 on another - so changing design used to resize the whole document
                without anyone asking for that. This multiplies, so a design keeps its own
                proportions and every template can be brought to one comfortable size. */}
            {onScaleChange && (
                <>
                    <label className="doc-font-label text-body-small" htmlFor="document-scale">
                        Text size
                    </label>
                    <select
                        id="document-scale"
                        className="form-control doc-font-select"
                        value={activeSize}
                        disabled={savingScale}
                        onChange={(event) => onScaleChange(event.target.value)}
                    >
                        <option value="normal">Default</option>
                        {DOCUMENT_SIZES.map((size) => (
                            <option key={size} value={size}>
                                {size} pt
                            </option>
                        ))}
                    </select>
                    <span className="doc-font-hint text-body-small">
                        {savingScale ? "Saving…" : "Body text size in points; every design is brought to it."}
                    </span>
                </>
            )}
        </div>
    );
};

export default DocumentFontBar;
