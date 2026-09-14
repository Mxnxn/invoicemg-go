import React, { useEffect, useRef, useState } from "react";
import { Maximize2, Hash } from "react-feather";
import AddRowIcon from "./AddRowIcon";
import "./dataTable.css";

// "Add row", with the kind of row chosen up front.
//
// A line is priced one of two ways and the choice is structural, not a field you fill in
// later: a by-dimension row multiplies qty x length x width x rate, a by-quantity row is just
// qty x rate. Asking at the moment the row is created is what keeps the table honest - the
// alternative, two dimension inputs that some rows are silently allowed to leave blank, gives
// no clue which rows are finished and which are abandoned.
//
// The choice here is only the row's STARTING kind. Every row can be switched afterwards from
// its own Length/Width cells (see RowSizeCells), so picking the wrong one costs a click, not a
// deleted row. Asking up front still earns its place: it sets the row up right for the common
// case, and it is where the two kinds are named and explained.
const AddRowButton = ({ onAdd, disabled, label = "Add Row" }) => {
    const [open, setOpen] = useState(false);
    const wrapRef = useRef(null);

    // Close on an outside click or Escape - the same two exits every other menu in the app
    // offers, so the popup never becomes something you have to hunt for a way out of.
    useEffect(() => {
        if (!open) return undefined;
        const onDocClick = (e) => {
            if (wrapRef.current && !wrapRef.current.contains(e.target)) setOpen(false);
        };
        const onKey = (e) => {
            if (e.key === "Escape") setOpen(false);
        };
        document.addEventListener("mousedown", onDocClick);
        document.addEventListener("keydown", onKey);
        return () => {
            document.removeEventListener("mousedown", onDocClick);
            document.removeEventListener("keydown", onKey);
        };
    }, [open]);

    const choose = (hasDimensions) => {
        setOpen(false);
        onAdd(hasDimensions);
    };

    return (
        <div className="xan-addrow" ref={wrapRef}>
            <button
                type="button"
                className="shell-btn shell-btn-primary xan-addrow-trigger"
                onClick={() => setOpen((v) => !v)}
                disabled={disabled}
                aria-haspopup="menu"
                aria-expanded={open}
            >
                <AddRowIcon open={open} size={13} />
                {label}
            </button>

            {open && (
                <div className="xan-addrow-menu" role="menu">
                    <button type="button" role="menuitem" className="xan-addrow-option" onClick={() => choose(true)}>
                        <Maximize2 size={14} className="xan-addrow-icon" />
                        <span>
                            <span className="xan-addrow-option-title">By dimension</span>
                            <span className="xan-addrow-option-sub">qty × length × width × rate</span>
                        </span>
                    </button>
                    <button type="button" role="menuitem" className="xan-addrow-option" onClick={() => choose(false)}>
                        <Hash size={14} className="xan-addrow-icon" />
                        <span>
                            <span className="xan-addrow-option-title">By quantity</span>
                            <span className="xan-addrow-option-sub">qty × rate, no size</span>
                        </span>
                    </button>
                </div>
            )}
        </div>
    );
};

export default AddRowButton;
