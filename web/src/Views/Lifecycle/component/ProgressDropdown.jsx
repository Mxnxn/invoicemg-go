import React, { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Check } from "react-feather";
import { notifyError } from "../../../global/toast";
import { portalHost } from "../../../Common/portalHost";

// Per-row progress (see Refactor.md "# NEW") - simpler 3-state vocabulary than the old
// job-level Progress (no vendor, no Needs Review bridge): a row starts at "Assign", flips to
// "In Progress" once an employee is picked, and can be marked "Complete" from there.
const PROGRESS_OPTIONS = ["Assign", "In Progress", "Complete"];

const PROGRESS_STYLE = {
    Assign: { bg: "var(--bg-field)", color: "var(--text-tertiary)", border: "var(--border-default)" },
    "In Progress": { bg: "var(--xan-blue-bg)", color: "var(--xan-blue)", border: "var(--xan-blue)" },
    Complete: { bg: "var(--xan-emerald-bg)", color: "var(--xan-emerald)", border: "var(--xan-emerald)" },
};

// Same floating/portal pattern as RowActionMenu (the table panel scrolls horizontally,
// which clips any absolutely-positioned dropdown that opens inside a row otherwise).
const ProgressDropdown = ({ value, hasAssignee, onSelect, disabled }) => {
    const [open, setOpen] = useState(false);
    const [position, setPosition] = useState(null);
    const triggerRef = useRef(null);
    const menuRef = useRef(null);
    const style = PROGRESS_STYLE[value] || PROGRESS_STYLE.Assign;

    useEffect(() => {
        if (!open) {
            setPosition(null);
            return;
        }
        const rect = triggerRef.current.getBoundingClientRect();
        setPosition({ top: rect.bottom + 6, left: rect.left });
    }, [open]);

    useEffect(() => {
        if (!open) return;
        const close = () => setOpen(false);
        const onOutside = (e) => {
            if (menuRef.current?.contains(e.target) || triggerRef.current?.contains(e.target)) return;
            close();
        };
        const onKeyDown = (e) => {
            if (e.key === "Escape") close();
        };
        document.addEventListener("mousedown", onOutside);
        document.addEventListener("keydown", onKeyDown);
        window.addEventListener("scroll", close, true);
        window.addEventListener("resize", close);
        return () => {
            document.removeEventListener("mousedown", onOutside);
            document.removeEventListener("keydown", onKeyDown);
            window.removeEventListener("scroll", close, true);
            window.removeEventListener("resize", close);
        };
    }, [open]);

    return (
        <div style={{ display: "inline-block" }}>
            <button
                ref={triggerRef}
                type="button"
                disabled={disabled}
                onClick={(e) => {
                    e.stopPropagation();
                    if (disabled) return;
                    setOpen((v) => !v);
                }}
                aria-haspopup="true"
                aria-expanded={open}
                style={{
                    height: 24,
                    padding: "0 10px",
                    borderRadius: 9999,
                    border: `1px solid ${style.border}`,
                    background: style.bg,
                    color: style.color,
                    fontSize: 10,
                    fontWeight: 700,
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                    whiteSpace: "nowrap",
                    cursor: disabled ? "not-allowed" : "pointer",
                    opacity: disabled ? 0.5 : 1,
                }}
            >
                {value}
            </button>
            {open &&
                position &&
                createPortal(
                    <div
                        ref={menuRef}
                        className="xan-row-menu"
                        role="menu"
                        style={{ top: position.top, left: position.left, transform: "none", minWidth: 180 }}
                    >
                        {PROGRESS_OPTIONS.map((opt) => {
                            const needsAssignee = opt !== "Assign" && !hasAssignee;
                            const optStyle = PROGRESS_STYLE[opt];
                            return (
                                <button
                                    key={opt}
                                    type="button"
                                    role="menuitem"
                                    className="xan-row-menu-item"
                                    aria-disabled={needsAssignee}
                                    style={needsAssignee ? { opacity: 0.45, cursor: "not-allowed" } : undefined}
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        if (needsAssignee) {
                                            notifyError("Assign an employee before changing progress.");
                                            return;
                                        }
                                        setOpen(false);
                                        if (opt !== value) onSelect(opt);
                                    }}
                                >
                                    <Check size={14} style={{ visibility: opt === value ? "visible" : "hidden", color: optStyle.color }} />
                                    <span style={{ color: optStyle.color, fontWeight: 600 }}>{opt}</span>
                                </button>
                            );
                        })}
                    </div>,
                    portalHost()
                )}
        </div>
    );
};

export default ProgressDropdown;
