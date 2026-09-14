import React from "react";

import RowCheckbox from "../DataTable/RowCheckbox";
import { RoundOff } from "../DateAndTime/RoundOff";
import { cardSize } from "../../Views/Lifecycle/jobBoard";
import "./cards.css";

// How many products a card names before it stops. A fixed cap rather than a measurement, the
// same reason the board's grid uses one: measuring real overflow makes equal-sized cards
// render differently depending on how long somebody's product names happen to be.
const PRODUCTS_SHOWN = 3;

const money = (n) => `₹${RoundOff(Number(n) || 0)}`;

/**
 * One job-id, as a card.
 *
 * The same drawing on the sheet, on the customer page and on the board, because it is the same
 * object. Three drawings of one thing is three things to learn, and they drift: the sheet had a
 * thirteen-column table row, the customer page an eleven-column one, and the board a chip -
 * all of the same job-id, none of them agreeing on what mattered about it.
 *
 * What the card asserts is fixed: the number, what it is worth, how much work is on it, and
 * what that work is. What it MEANS is the caller's - `badge` is a slot, because "Due" on a
 * sheet and "Awaiting payment" on a customer page are different readings of the same job and
 * this component has no business choosing between them.
 */
export default function JobCard({ job, onOpen, badge, selectable = false, selected = false, onToggleSelect }) {
    const rows = job.rows || [];
    const products = rows
        .map((r) => ({ name: r.material || r.description || "", size: cardSize(r) }))
        .filter((p) => p.name);
    const shown = products.slice(0, PRODUCTS_SHOWN);
    const more = products.length - shown.length;

    return (
        <div className={`job-card-wrap${selected ? " is-selected" : ""}`}>
            {/* A sibling of the card, never a child: the card is a <button>, and a checkbox
                inside one is neither valid nor operable - the button swallows the click. */}
            {selectable && (
                <span className="job-card-check">
                    <RowCheckbox
                        checked={selected}
                        onChange={() => onToggleSelect?.(job)}
                        ariaLabel={`Select ${job.challanNumber}`}
                    />
                </span>
            )}

            <button type="button" className="job-card" onClick={() => onOpen?.(job)}>
                {/* The number owns its line. Sharing it with the badge left roughly ninety
                    pixels for "JOB/26-27/000012", which ellipsised to "JOB/26-27/00..." - and
                    an identifier truncated past the part that identifies it is not an
                    identifier. The badge reads just as well beside the money. */}
                <span className="job-card-top">
                    <span className="job-card-no">{job.challanNumber}</span>
                </span>

                <span className="job-card-meta">
                    <span>
                        {money(job.total)} · {rows.length} job card{rows.length === 1 ? "" : "s"}
                    </span>
                    {badge}
                </span>

                {/* Name and size together - three badges reading "Vinyl, Vinyl, Vinyl" say less
                    than three carrying their sizes. */}
                <span className="job-card-products">
                    {shown.map((p, i) => (
                        <span key={`${p.name}-${i}`} className="job-card-product">
                            {p.name}
                            {p.size && <span className="job-card-product-size">{p.size}</span>}
                        </span>
                    ))}
                    {more > 0 && <span className="job-card-product is-more">+{more} more</span>}
                </span>
            </button>
        </div>
    );
}
