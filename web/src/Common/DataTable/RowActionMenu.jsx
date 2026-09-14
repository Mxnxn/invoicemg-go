import React, { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { MoreVertical } from "react-feather";
import "./dataTable.css";
import { portalHost } from "../portalHost";

// Floating, theme-aware row action menu. Renders via a portal to <body> and positions
// itself with getBoundingClientRect so it is never clipped by an ancestor's overflow
// (the table panel scrolls horizontally, which otherwise clips any absolutely-positioned
// dropdown that opens inside a row).
const RowActionMenu = ({ open, onOpenChange, children }) => {
    const triggerRef = useRef(null);
    const menuRef = useRef(null);
    const [position, setPosition] = useState(null);

    useEffect(() => {
        if (!open) {
            setPosition(null);
            return;
        }
        const trigger = triggerRef.current;
        if (!trigger) return;
        const rect = trigger.getBoundingClientRect();
        setPosition({ top: rect.bottom + 6, left: rect.right });
    }, [open]);

    useEffect(() => {
        if (!open) return;

        const close = () => onOpenChange(false);

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
    }, [open, onOpenChange]);

    return (
        <div className="xan-row-menu-wrap">
            <button
                ref={triggerRef}
                type="button"
                className="xan-row-menu-trigger"
                aria-label="Row actions"
                aria-haspopup="true"
                aria-expanded={open}
                onClick={() => onOpenChange(!open)}
            >
                <MoreVertical size={16} />
            </button>
            {open &&
                position &&
                createPortal(
                    <div
                        ref={menuRef}
                        className="xan-row-menu"
                        role="menu"
                        style={{ top: position.top, left: position.left, transform: "translateX(-100%)" }}
                    >
                        {children}
                    </div>,
                    portalHost()
                )}
        </div>
    );
};

export const RowActionMenuItem = ({ icon: Icon, variant = "default", children, ...props }) => (
    <button
        type="button"
        role="menuitem"
        className={["xan-row-menu-item", variant !== "default" ? `xan-row-menu-item-${variant}` : ""].filter(Boolean).join(" ")}
        {...props}
    >
        {Icon && <Icon size={14} />}
        <span>{children}</span>
    </button>
);

export default RowActionMenu;
