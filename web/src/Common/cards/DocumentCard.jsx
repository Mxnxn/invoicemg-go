import React from "react";

import RowCheckbox from "../DataTable/RowCheckbox";
import { cardSize } from "../../Views/Lifecycle/jobBoard";
import "./cards.css";

const PRODUCTS_SHOWN = 3;

/**
 * A document, as a card.
 *
 * An invoice, a quotation, a purchase invoice and a purchase order are the same object wearing
 * four names: a number, a party, a date, a total, a state, and some rows. They had four tables
 * between them - nine columns, six columns, five and five - agreeing on none of it, so the same
 * question ("what is this and has it been dealt with") was asked four different ways.
 *
 * What a document IS lives here. What it MEANS stays with the caller: `badges` and `figures`
 * are slots, because "Due" on a sales invoice and "Approved" on a purchase order are not the
 * same kind of claim and this component has no business flattening them.
 *
 * `rows` is optional. When a document carries line items their products are named, in the same
 * name-plus-size pairing the job card uses - it is the difference between knowing a document
 * exists and knowing what is on it.
 */
export default function DocumentCard({
    id,
    number,
    party,
    partySub,
    date,
    figures = [],
    badges,
    rows,
    onOpen,
    selectable = false,
    selected = false,
    onToggleSelect,
    ariaLabel,
    actions,
}) {
    const products = (rows || [])
        .map((r) => ({ name: r.material || r.description || "", size: cardSize(r) }))
        .filter((p) => p.name);
    const shown = products.slice(0, PRODUCTS_SHOWN);
    const more = products.length - shown.length;

    return (
        <div id={id} className={`doc-card-wrap${selected ? " is-selected" : ""}`}>
            {/* A sibling of the card, never a child - the card is a <button>, and a checkbox
                inside one is neither valid nor operable. */}
            {/* One cluster in the top-right: what the document IS, then what you can do with
                it. Both sit OUTSIDE the card button - the card is a <button>, and a button or
                a checkbox inside one is neither valid nor operable. The selection box moves to
                the opposite corner rather than queue behind them. */}
            {(badges || actions) && (
                <span className="doc-card-corner">
                    {badges}
                    {actions}
                </span>
            )}

            {selectable && (
                <span className="job-card-check">
                    <RowCheckbox
                        checked={selected}
                        onChange={() => onToggleSelect?.()}
                        ariaLabel={ariaLabel || `Select ${number}`}
                    />
                </span>
            )}

            <button type="button" className="doc-card" onClick={() => onOpen?.()}>
                {/* The number owns its line. It is what the document is called, and sharing the
                    line with a status badge is what truncates it. */}
                <span className="doc-card-no">{number}</span>

                {/* No monogram. It earns its place on the dashboard, where you are hunting one
                    customer among many and the initials are a shape to match before the name is
                    read. Here the deck is of DOCUMENTS - you arrive knowing the number or the
                    date, not the face - so thirty circles of initials were thirty repetitions
                    of something nobody was looking for, taking the width the name needed. */}
                <span className="doc-card-names">
                    <span className="doc-card-firm">{party || "—"}</span>
                    <span className="doc-card-sub">
                        {partySub && partySub !== party ? `${partySub} · ${date || ""}` : date || ""}
                    </span>
                </span>

                {shown.length > 0 && (
                    <span className="job-card-products">
                        {shown.map((p, i) => (
                            <span key={`${p.name}-${i}`} className="job-card-product">
                                {p.name}
                                {p.size && <span className="job-card-product-size">{p.size}</span>}
                            </span>
                        ))}
                        {more > 0 && <span className="job-card-product is-more">+{more} more</span>}
                    </span>
                )}

                {/* Last on the card, and pinned to its foot by margin-top:auto.
                    The money is the conclusion of a document, and putting it above a variable
                    number of product chips meant it sat at a different height on every card -
                    so a deck of them could not be read down a column. */}
                {figures.length > 0 && (
                    <span className="doc-card-figures">
                        {figures.map((f) => (
                            <span key={f.label} className={`doc-card-figure${f.tone ? ` is-${f.tone}` : ""}`}>
                                <span className="doc-card-figure-label">{f.label}</span>
                                <span className="doc-card-figure-value">{f.value}</span>
                            </span>
                        ))}
                    </span>
                )}

            </button>
        </div>
    );
}
