import React, { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import "./slideOverlay.css";
import { portalHost } from "../portalHost";

// Shared fixed-left slide-in panel used by the client-detail "sidebars" (materials,
// history, received-history). Closes on backdrop click or Escape, and animates in/out
// instead of just popping into place.
const SlideOverlay = ({ onClose, width = 480, children }) => {
    const [visible, setVisible] = useState(false);

    useEffect(() => {
        const raf = requestAnimationFrame(() => setVisible(true));
        return () => cancelAnimationFrame(raf);
    }, []);

    useEffect(() => {
        const onKeyDown = (e) => {
            if (e.key === "Escape") onClose();
        };
        document.addEventListener("keydown", onKeyDown);
        return () => document.removeEventListener("keydown", onKeyDown);
    }, [onClose]);

    return createPortal(
        <div
            className={["slide-overlay-backdrop", visible ? "visible" : ""].filter(Boolean).join(" ")}
            onMouseDown={(e) => {
                if (e.target === e.currentTarget) onClose();
            }}
        >
            <div
                className={["slide-overlay-panel", visible ? "visible" : ""].filter(Boolean).join(" ")}
                style={{ maxWidth: width }}
                onMouseDown={(e) => e.stopPropagation()}
            >
                {children}
            </div>
        </div>,
        portalHost()
    );
};

export default SlideOverlay;
