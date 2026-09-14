import React from "react";
import { Link, useLocation } from "react-router-dom";
import { Shield, ChevronsLeft, ChevronsRight, X } from "react-feather";
import "./shell.css";

// The superadmin's sidebar. Deliberately a separate component from SideMenu rather than a
// role branch inside it: a superadmin is a developer-team operator, not a customer, so it
// shares no nav with the tenant app. Nothing here is company-scoped - no company switcher,
// no account details, no "add account" - because the operator never acts as a tenant.
const DevSideMenu = ({ collapsed, onToggleCollapsed, mobileOpen, onCloseMobile }) => {
    const location = useLocation();

    return (
        <aside
            className={["shell-sidebar", collapsed ? "collapsed" : "", mobileOpen ? "mobile-open" : ""].filter(Boolean).join(" ")}
        >
            <button type="button" className="shell-drawer-close" aria-label="Close navigation" onClick={onCloseMobile}>
                <X size={18} />
            </button>
            <div className="shell-brand">
                <div className="shell-brand-text">
                    <span className="text-heading-brand name">InvoiceMG</span>
                    <span className="text-body-regular plan">Developer</span>
                </div>
                <button
                    className="shell-collapse-btn"
                    type="button"
                    onClick={onToggleCollapsed}
                    aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
                >
                    {collapsed ? <ChevronsRight size={16} /> : <ChevronsLeft size={16} />}
                </button>
            </div>

            <nav className="shell-nav" aria-label="Developer">
                <div className="shell-nav-items">
                    <Link
                        to="/admin/dev"
                        className={[
                            "shell-nav-item text-body-regular",
                            location.pathname.startsWith("/admin/dev") ? "active" : "",
                        ]
                            .filter(Boolean)
                            .join(" ")}
                        title={collapsed ? "Dev" : undefined}
                    >
                        <Shield size={16} />
                        <span>Dev</span>
                    </Link>
                </div>
            </nav>
        </aside>
    );
};

export default DevSideMenu;
