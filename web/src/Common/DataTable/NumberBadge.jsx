import React from "react";
import "./dataTable.css";

/**
 * A document's number, as a badge.
 *
 * Invoice numbers, purchase invoice numbers and purchase order numbers are identifiers, not
 * prose - they are what someone scans a column for and what they read back down a phone. As
 * plain monospace text in the first cell they sat at the same weight as the supplier name
 * beside them, so the column you actually navigate by was the least distinct thing in the row.
 *
 * Not StatusBadge: that one carries a pulse dot and a colour that means something about state.
 * A number has no state, so this is a quiet outline - it separates the identifier from the
 * data without claiming anything about it.
 */
export default function NumberBadge({ children, onClick, title, accent = false }) {
    if (children === undefined || children === null || children === "") return null;

    // A button only when it does something. An identifier that looks pressable but is not is
    // worse than one that plainly is not.
    if (!onClick) {
        return <span className={`xan-number-badge${accent ? " is-accent" : ""}`}>{children}</span>;
    }

    return (
        <button
            type="button"
            className={`xan-number-badge is-action${accent ? " is-accent" : ""}`}
            onClick={onClick}
            title={title}
        >
            {children}
        </button>
    );
}
