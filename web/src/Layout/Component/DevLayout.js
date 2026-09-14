import React, { useEffect, useLayoutEffect, useState } from "react";
import { Route, Routes, Navigate } from "react-router-dom";
import DevSideMenu from "../../Shell/DevSideMenu";
import DevNavbar from "../../Shell/DevNavbar";
import DevIndex from "../../Views/Dev/component/DevIndex";
import { toggleThemeWithReveal } from "../../Common/themeTransition";
import "../../Shell/shell.css";

// The superadmin shell. A sibling of AdminLayout rather than a set of role conditions
// inside it: the two audiences share chrome, not content. Everything tenant-scoped -
// company switcher, account details, trash, business nav - is simply absent here instead
// of being conditionally hidden, so there is no state where an operator half-sees the
// customer app.
//
// This is convenience only. requireSuperAdmin on the server is the real gate: every call
// DevIndex makes 403s for anyone else regardless of which layout rendered it.
const DevLayout = () => {
    const [mobileNavOpen, setMobileNavOpen] = useState(false);
    const [collapsed, setCollapsed] = useState(() => {
        const stored = window.localStorage.getItem("sidebar_collapsed");
        return stored === null ? false : stored === "true";
    });

    useEffect(() => {
        window.localStorage.setItem("sidebar_collapsed", String(collapsed));
    }, [collapsed]);

    const [theme, setTheme] = useState(() => window.localStorage.getItem("theme") || "light");

    const toggleTheme = (event) => {
        toggleThemeWithReveal(event, () => {
            setTheme((prev) => (prev === "dark" ? "light" : "dark"));
        });
    };

    useLayoutEffect(() => {
        // Layout effect, matching AdminLayout: it must run inside the flushSync() the
        // reveal-animation toggle wraps its state update in, or the View Transition API
        // snapshots the DOM before data-theme flips.
        window.localStorage.setItem("theme", theme);
        document.documentElement.setAttribute("data-theme", theme);
    }, [theme]);

    return (
        <div className="shell-root" data-shell data-theme={theme}>
            <DevSideMenu
                collapsed={collapsed}
                onToggleCollapsed={() => setCollapsed((prev) => !prev)}
                mobileOpen={mobileNavOpen}
                onCloseMobile={() => setMobileNavOpen(false)}
            />
            {mobileNavOpen && <div className="shell-nav-backdrop" onClick={() => setMobileNavOpen(false)} />}
            <div className="shell-main">
                <DevNavbar
                    onOpenMobileNav={() => setMobileNavOpen(true)}
                    heading="Dev"
                    theme={theme}
                    onToggleTheme={toggleTheme}
                />
                <div className="shell-content">
                    <Routes>
                        <Route index element={<Navigate to="dev" replace />} />
                        <Route path="dev" element={<DevIndex />} />
                        {/* No tenant routes exist in this shell, so anything else is a
                            stale link or a typed URL - send it back to the panel. */}
                        <Route path="*" element={<Navigate to="dev" replace />} />
                    </Routes>
                </div>
            </div>
        </div>
    );
};

export default DevLayout;
