import React from "react";
import { Input } from "reactstrap";
import { Plus, X } from "react-feather";
import { hasDimensions } from "../rowPricing";
import "./dataTable.css";

// The Length and Width cells of an editable row, plus the control that switches the row
// between being priced by dimension and by quantity.
//
// One component rather than the same pair of <td>s written out in the job, quotation and
// purchase-invoice modals: three copies of a control that decides how a line is PRICED is
// three chances for two screens to disagree about what a row costs.
//
// Switching is allowed at any time, not only while the row is untouched. A row added the wrong
// way is then a single click to fix instead of a delete and a retype. The switch only flips the
// flag - length and width are left exactly as they were, so turning dimensions off and on again
// brings the same numbers back rather than resetting them to 1.
//
// The width of each of the two columns these fill, in px.
//
// 70% of the 150px Qty/Rate columns. Length and Width hold two or three characters where Rate
// holds a money figure, so matching them exactly wasted the row's width - but at 70px they
// read as a different class of field altogether. Kept as a share of the money columns, and
// exported, so the three modals cannot drift apart and the relationship survives the next time
// either width is revisited.
export const QTY_COL_PX = 150;
export const SIZE_COL_PX = Math.round(QTY_COL_PX * 0.7);

// Returns two <td>s, so the caller's column order stays the caller's business.
const RowSizeCells = ({ row, disabled, onChange, needsLength, needsWidth }) => {
    const on = hasDimensions(row);

    if (!on) {
        return (
            <>
                <td>
                    <button
                        type="button"
                        className="xan-size-add"
                        disabled={disabled}
                        title="Price this row by length x width instead"
                        onClick={() => onChange({ hasDimensions: true })}
                    >
                        <Plus size={10} />
                        Size
                    </button>
                </td>
                <td>
                    <span className="xan-cell-na" title="This row is priced by quantity">
                        &mdash;
                    </span>
                </td>
            </>
        );
    }

    return (
        <>
            {/* Both cells use the same shell, and Length carries an empty gutter the exact
                width of Width's clear button. Without it Length got the whole cell and Width
                got the cell minus the button, so two fields that hold the same kind of value
                were visibly different sizes. */}
            <td>
                <div className="xan-size-cell">
                    <Input
                        className={`cell-input cell-input-narrow${needsLength ? " is-required-missing" : ""}`}
                        type="number"
                        step="1"
                        value={row.length}
                        disabled={disabled}
                        onChange={(e) => onChange({ length: e.target.value })}
                    />
                    {!disabled && <span className="xan-size-gutter" aria-hidden="true" />}
                </div>
            </td>
            <td>
                <div className="xan-size-cell">
                    <Input
                        className={`cell-input cell-input-narrow${needsWidth ? " is-required-missing" : ""}`}
                        type="number"
                        step="1"
                        value={row.width}
                        disabled={disabled}
                        onChange={(e) => onChange({ width: e.target.value })}
                    />
                    {!disabled && (
                        <button
                            type="button"
                            className="xan-size-drop xan-size-gutter"
                            aria-label="Price this row by quantity instead"
                            title="Drop the size - price this row by quantity"
                            onClick={() => onChange({ hasDimensions: false })}
                        >
                            <X size={13} strokeWidth={2.5} />
                        </button>
                    )}
                </div>
            </td>
        </>
    );
};

export default RowSizeCells;
