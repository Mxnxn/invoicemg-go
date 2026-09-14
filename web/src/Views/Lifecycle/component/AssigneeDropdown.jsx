import React, { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import "../../../Common/DataTable/dataTable.css";
import { X, Plus, ChevronDown, ChevronUp } from "react-feather";
import { placeMenu } from "../../../Common/menuPlacement";
import { portalHost } from "../../../Common/portalHost";

// Searchable, portal-based assignee picker for a single table cell (Employee or Vendor
// column). Same clip-safe floating pattern as RowActionMenu/ProgressDropdown, since this
// lives inside the horizontally-scrolling table panel too.
// - `menuWidth` overrides the default 200px floor (e.g. for the Client picker, which needs
//   more room to stay readable - see CreateJobModal/CreateQuotationModal).
// - Each option may carry a `search` string (lowercased, e.g. "firm name phone") searched
//   instead of `name` - lets the displayed label (Firm only) differ from what's searchable
//   (Firm + contact name + phone).
// - `onCreateNew(query)` is optional; when set, a "+ Create new ..." row always appears at
//   the bottom of the list so picking a not-yet-existing option is one click away.
// - `variant="dashed"` (fullWidth only) swaps the trigger's Argon box-shadow/border for a
//   white background + dashed border + no shadow - used for the Client picker on the
//   Job/Quotation Create modals, scoped via .assignee-dropdown-dashed so it doesn't affect
//   every other AssigneeDropdown usage.
const AssigneeDropdown = ({ value, placeholder, options, onSelect, fullWidth, menuWidth, onCreateNew, variant, disabled }) => {
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState("");
    const [position, setPosition] = useState(null);
    const triggerRef = useRef(null);
    const menuRef = useRef(null);
    const inputRef = useRef(null);
    // Measured against the VISUAL viewport, which is what the on-screen keyboard shrinks. On
    // iOS the layout viewport does not change at all when the keyboard opens, so
    // window.innerHeight would say there is still room below and the menu would keep rendering
    // behind the keyboard - open, focused, and invisible.
    const reposition = () => {
        if (!triggerRef.current) return;
        const rect = triggerRef.current.getBoundingClientRect();
        const vv = window.visualViewport;
        setPosition(
            placeMenu({
                trigger: rect,
                viewportHeight: vv ? vv.height : window.innerHeight,
                viewportOffsetTop: vv ? vv.offsetTop : 0,
                menuHeight: 260,
                width: Math.max(rect.width, fullWidth ? rect.width : menuWidth || 200),
            })
        );
    };

    useEffect(() => {
        if (!open) {
            setPosition(null);
            setQuery("");
            return;
        }
        reposition();
        // focus the search box once it mounts
        requestAnimationFrame(() => inputRef.current?.focus());
        // eslint-disable-next-line react-hooks/exhaustive-deps
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
        // Capture-phase scroll listener catches scrolling anywhere on the page (so the menu
        // repositions/closes if its trigger scrolls out of view) - but that also fires for
        // scrolling the menu's own option list, which shouldn't close it.
        // Reposition rather than close. Closing was the old behaviour and it is what made
        // the keyboard look like it dismissed the menu: the keyboard scrolls the focused field
        // into view, which fired this. Following the trigger is both correct and less abrupt.
        const onScroll = (e) => {
            if (menuRef.current?.contains(e.target)) return;
            reposition();
        };
        // Same again: the keyboard opening IS a resize, and the right answer is to move the
        // menu somewhere it can still be seen, not to take it away.
        const onResize = () => reposition();
        document.addEventListener("mousedown", onOutside);
        document.addEventListener("keydown", onKeyDown);
        window.addEventListener("scroll", onScroll, true);
        window.addEventListener("resize", onResize);
        // The keyboard's own signal. Android usually resizes the layout viewport too, but iOS
        // reports it here and nowhere else, so without this the flip never happens there.
        const vv = window.visualViewport;
        vv?.addEventListener("resize", onResize);
        vv?.addEventListener("scroll", onResize);
        return () => {
            document.removeEventListener("mousedown", onOutside);
            document.removeEventListener("keydown", onKeyDown);
            window.removeEventListener("scroll", onScroll, true);
            window.removeEventListener("resize", onResize);
            vv?.removeEventListener("resize", onResize);
            vv?.removeEventListener("scroll", onResize);
        };
    }, [open]);

    const filtered = options.filter((o) => (o.search || o.name).toLowerCase().includes(query.toLowerCase()));

    return (
        <div style={{ display: "inline-flex", alignItems: "center", gap: 4, maxWidth: "100%", width: fullWidth ? "100%" : undefined }}>
            <button
                ref={triggerRef}
                type="button"
                // fullWidth is only ever used beside real <Input> fields in a form row (Client/
                // Employee/Vendor pickers) - borrow the exact same classes those Inputs use so
                // the trigger matches their height/padding/border pixel-for-pixel instead of
                // guessing at equivalent inline styles.
                className={
                    fullWidth
                        ? ["form-control", "form-control-alternative", "nn", variant === "dashed" ? "assignee-dropdown-dashed" : ""]
                              .filter(Boolean)
                              .join(" ")
                        : undefined
                }
                disabled={disabled}
                onClick={(e) => {
                    e.stopPropagation();
                    if (disabled) return;
                    setOpen((v) => !v);
                }}
                aria-haspopup="true"
                aria-expanded={open}
                style={{
                    border: fullWidth ? undefined : "1px dashed var(--border-default)",
                    background: fullWidth ? undefined : "transparent",
                    borderRadius: fullWidth ? undefined : "var(--radius-chip)",
                    padding: fullWidth ? undefined : "3px 8px",
                    fontSize: fullWidth ? undefined : 12.5,
                    fontFamily: "inherit",
                    color: value ? "var(--text-primary)" : "var(--text-tertiary)",
                    cursor: disabled ? "not-allowed" : "pointer",
                    opacity: disabled ? 0.5 : 1,
                    width: fullWidth ? "100%" : undefined,
                    maxWidth: fullWidth ? "100%" : 140,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                    textAlign: "left",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    gap: 6,
                }}
            >
                {value || placeholder}
                {/* A dropdown should look like one. Without a caret the trigger is a box of
                    text that gives no sign it opens, which is exactly how people miss it -
                    and the direction says whether pressing it opens or closes. */}
                <span className="assignee-dropdown-caret" aria-hidden="true">
                    {open ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
                </span>
            </button>
            {value && !disabled && (
                <button
                    type="button"
                    onClick={(e) => {
                        e.stopPropagation();
                        onSelect(null, null);
                    }}
                    aria-label={`Clear ${placeholder}`}
                    className="xan-clear-btn"
                >
                    <X size={13} strokeWidth={2.5} />
                </button>
            )}
            {open &&
                position &&
                createPortal(
                    <div
                        ref={menuRef}
                        className="xan-row-menu"
                        role="listbox"
                        style={{
                            top: position.top,
                            left: position.left,
                            transform: "none",
                            width: position.width,
                            maxHeight: position.maxHeight,
                            overflow: "hidden",
                        }}
                    >
                        <div style={{ padding: 6 }}>
                            <input
                                ref={inputRef}
                                className="assignee-search"
                                type="text"
                                value={query}
                                onChange={(e) => setQuery(e.target.value)}
                                placeholder={`Search ${placeholder.toLowerCase()}...`}
                                style={{
                                    width: "100%",
                                    borderRadius: "var(--radius-chip)",
                                    border: "1px solid var(--border-default)",
                                    background: "var(--bg-field)",
                                    color: "var(--text-primary)",
                                    padding: "6px 8px",
                                    fontSize: 12.5,
                                    fontFamily: "inherit",
                                }}
                            />
                        </div>
                        {/* Follows the menu's own height, so a menu shrunk to fit a gap still
                            scrolls its list instead of spilling past the edge. ~60px is the
                            search box above it. */}
                        <div style={{ maxHeight: Math.max(80, (position.maxHeight || 260) - 60), overflowY: "auto" }}>
                            {/* Create sits ABOVE the results, not below them. Below, it moved
                                as you typed - a list of forty customers pushed it off the
                                scroll, so 'add a new one' was only reachable by scrolling past
                                every one that already existed. At the top it is in the same
                                place every time, which is what makes it findable. */}
                            {onCreateNew && (
                                <button
                                    type="button"
                                    className="xan-row-menu-item"
                                    style={{
                                        fontSize: 13.5,
                                        padding: "8px 10px",
                                        display: "flex",
                                        alignItems: "center",
                                        gap: 6,
                                        color: "var(--xan-blue)",
                                    }}
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        // Opens the form whether or not anything was typed.
                                        // It used to refuse on an empty box and pulse it red,
                                        // which made "create a client" a two-step ritual:
                                        // type a name you were about to type again in the
                                        // form anyway. Both quick-create modals already treat
                                        // the query as a prefill (`initialName || ""`), so an
                                        // empty one simply opens a blank form.
                                        setOpen(false);
                                        onCreateNew(query);
                                    }}
                                >
                                    <Plus size={13} />
                                    {query ? `Create "${query}"` : "Create new"}
                                </button>
                            )}
                            {onCreateNew && filtered.length > 0 && (
                                <div
                                    aria-hidden="true"
                                    style={{ height: 1, background: "var(--border-default)", margin: "4px 0" }}
                                />
                            )}
                            {filtered.map((opt) => (
                                <button
                                    key={opt.id}
                                    type="button"
                                    role="option"
                                    className="xan-row-menu-item"
                                    style={{ fontSize: 13.5, padding: "8px 10px" }}
                                    onClick={(e) => {
                                        e.stopPropagation();
                                        onSelect(opt.id, opt.name);
                                        setOpen(false);
                                    }}
                                >
                                    {opt.name}
                                </button>
                            ))}
                            {filtered.length === 0 && !onCreateNew && (
                                <div className="xan-row-menu-item" style={{ color: "var(--text-tertiary)", cursor: "default" }}>
                                    No matches
                                </div>
                            )}
                        </div>
                    </div>,
                    portalHost()
                )}
        </div>
    );
};

export default AssigneeDropdown;
