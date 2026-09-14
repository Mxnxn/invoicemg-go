import React from "react";
import { Link, useLocation } from "react-router-dom";
import { Home, FileText, PieChart, Sliders, ChevronsLeft, ChevronsRight, Layers, List, ShoppingCart, ShoppingBag, BarChart2, X, Repeat } from "react-feather";
import CompanySwitcher from "./CompanySwitcher";
import { hasAccess, can } from "../Common/access";
import "./shell.css";

// "Configure" lives in its own pinned group at the sidebar's bottom (see shell-bottom-group
// below), not in NAV_SECTIONS - it's gated on `keys` (any-of) instead of a single `key`,
// since it's a launcher for five separately-permissioned managers (see
// Views/Configure/ConfigureIndex.jsx).
const CONFIGURE_ITEM = { to: "/admin/configure", label: "Configure", icon: Sliders, keys: ["customers", "products", "challan"] };

const NAV_SECTIONS = [
    {
        label: "Operations",
        items: [
            { to: "/admin/analytics", label: "Analytics", icon: PieChart, key: "analytics" },
            { to: "/admin/dashboard", label: "Home", icon: Home, key: "dashboard" },
            { to: "/admin/lifecycle", label: "Jobs", icon: Layers, key: "lifecycle" },
            { to: "/admin/invoices", label: "Invoices", icon: FileText, key: "invoices" },
            { to: "/admin/purchase-invoices", label: "Purchase Invoices", icon: ShoppingCart, key: "purchase_invoices" },
            { to: "/admin/purchase-orders", label: "Purchase Orders", icon: ShoppingBag, key: "purchase_orders" },
            { to: "/admin/quotations", label: "Quotations", icon: List, key: "quotations" },
            { to: "/admin/bank-transfers", label: "Bank Transfers", icon: Repeat, key: "batch_receive" },
            // Empty for now (see MoreIndex.jsx/MORE_SECTIONS) now that Bank Transfers has its
            // own spot - parked here for whatever gets added next, so it's admin-only
            // (key: null) rather than gated on a feature nothing inside it uses anymore.
            { to: "/admin/more", label: "Reports", icon: BarChart2, key: null },
        ],
    },
];

const SideMenu = ({ collapsed, onToggleCollapsed, email, mobileOpen, onCloseMobile }) => {
    const location = useLocation();
    const role = window.localStorage.getItem("role") || "admin";
    const visibleSections =
        role === "employee"
            ? NAV_SECTIONS.map((section) => ({
                  ...section,
                  items: section.items.filter((item) => (item.keys ? item.keys.some(hasAccess) : item.key && hasAccess(item.key))),
              })).filter((section) => section.items.length > 0)
            : NAV_SECTIONS;
    // Configure is a management surface: it only earns a place in the sidebar if the user can
    // actually change something there. Viewing alone is not enough - an employee with
    // view-only access would land on screens where every control is hidden.
    const showConfigure = role !== "employee" || CONFIGURE_ITEM.keys.some((k) => can(k, "create"));

    return (
        <aside
            className={["shell-sidebar", collapsed ? "collapsed" : "", mobileOpen ? "mobile-open" : ""].filter(Boolean).join(" ")}
        >
            {/* Phone-only: the collapse chevron is meaningless in a drawer, so closing gets
                its own control. Hidden by CSS at desktop widths. */}
            <button type="button" className="shell-drawer-close" aria-label="Close navigation" onClick={onCloseMobile}>
                <X size={18} />
            </button>
            <div className="shell-brand">
                <div className="shell-brand-text">
                    <span className="text-heading-brand name">InvoiceMG</span>
                    <span className="text-body-regular plan">Admin</span>
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

            {visibleSections.map((section) => {
                const activeIndex = section.items.findIndex((item) => location.pathname.startsWith(item.to));
                return (
                    <nav className="shell-nav" key={section.label} aria-label={section.label}>
                        <div className="text-label-caps shell-nav-group-label">{section.label}</div>
                        <div className="shell-nav-items">
                            {activeIndex >= 0 && (
                                <div
                                    className="shell-nav-indicator"
                                    style={{ transform: `translateY(${activeIndex * 42}px)` }}
                                />
                            )}
                            {section.items.map(({ to, label, icon: Icon }) => {
                                const active = location.pathname.startsWith(to);
                                return (
                                    <Link
                                        key={to}
                                        to={to}
                                        className={["shell-nav-item text-body-regular", active ? "active" : ""].filter(Boolean).join(" ")}
                                        title={collapsed ? label : undefined}
                                    >
                                        <Icon size={16} />
                                        <span>{label}</span>
                                    </Link>
                                );
                            })}
                        </div>
                    </nav>
                );
            })}

            <div className="shell-bottom-group">
                {showConfigure && (
                    <nav className="shell-nav" aria-label="Configure">
                        <div className="shell-nav-items">
                            <Link
                                to={CONFIGURE_ITEM.to}
                                className={[
                                    "shell-nav-item text-body-regular",
                                    location.pathname.startsWith(CONFIGURE_ITEM.to) ? "active" : "",
                                ]
                                    .filter(Boolean)
                                    .join(" ")}
                                title={collapsed ? CONFIGURE_ITEM.label : undefined}
                            >
                                <Sliders size={16} />
                                <span>{CONFIGURE_ITEM.label}</span>
                            </Link>
                        </div>
                    </nav>
                )}
                <div className="shell-user">
                    <CompanySwitcher email={email} />
                </div>
            </div>
        </aside>
    );
};

export default SideMenu;
