import React, { Children, isValidElement } from "react";
import "./dataTable.css";

const DEFAULT_SHIMMER_ROWS = 5;

// How many columns the placeholder should be. Told is better than guessed, but a caller that
// does not say gets it counted off its own <thead> - which is right far more often than a
// hardcoded number, and means most call sites need only `loading`.
function columnCount(children, given) {
    if (given) return given;
    let count = 0;
    Children.forEach(children, (child) => {
        if (!isValidElement(child) || child.type !== "thead") return;
        Children.forEach(child.props.children, (row) => {
            if (!isValidElement(row)) return;
            const cells = Children.toArray(row.props.children).filter(isValidElement);
            count = Math.max(count, cells.length);
        });
    });
    return count || 1;
}

// Everything except the body. Keeping it by exclusion rather than looking for "thead" means
// a <caption> or <colgroup> survives too, and a header wrapped in a component still renders -
// it just cannot be counted, which is what the `columns` prop is for.
const withoutBody = (children) =>
    Children.toArray(children).filter((child) => !(isValidElement(child) && child.type === "tbody"));

/**
 * The frame every table sits in.
 *
 * `loading` swaps the body for a shimmer of the same shape while keeping the header. The
 * header is kept deliberately: the columns are known before the data is, and a table that
 * reflows when the rows land reads as a second, different table rather than the same one
 * filling in.
 *
 * Placeholder rows are sized from the real column count so the swap does not jump.
 */
const DataTable = ({ children, className = "", loading = false, columns, rows = DEFAULT_SHIMMER_ROWS }) => {
    const cols = loading ? columnCount(children, columns) : 0;

    return (
        <div
            className={["xan-panel", className].filter(Boolean).join(" ")}
            style={{ overflowX: "auto" }}
            aria-busy={loading ? "true" : undefined}
        >
            <table className="xan-table">
                {loading ? (
                    <>
                        {withoutBody(children)}
                        <tbody data-testid="table-shimmer">
                            {Array.from({ length: rows }).map((_, r) => (
                                <tr key={r} className="xan-shimmer-row">
                                    {Array.from({ length: cols }).map((__, c) => (
                                        <td key={c}>
                                            {/* aria-hidden: a screen reader gets aria-busy on the
                                                panel, which says "loading" far better than a row
                                                of empty cells does. */}
                                            <span className="xan-shimmer-cell" aria-hidden="true" />
                                        </td>
                                    ))}
                                </tr>
                            ))}
                        </tbody>
                    </>
                ) : (
                    children
                )}
            </table>
        </div>
    );
};

export default DataTable;
