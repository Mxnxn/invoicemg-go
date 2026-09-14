import React from "react";
import { Link, useNavigate } from "react-router-dom";
import { Menu, Sun, Moon, LogOut, Trash2, Plus } from "react-feather";
import { hasAccess } from "../Common/access";
import NotificationBell from "./NotificationBell";
import { usePendingApprovals } from "./pendingApprovals";
import { signOut } from "../Common/signOut";
import "./shell.css";

const AppNavbar = ({ heading, theme, onToggleTheme, analyticsSource, onToggleAnalyticsSource, onOpenMobileNav, onCreateJob }) => {
    // One shared sign-out: the local session always ends, whatever the server says.
    // See Common/signOut.js for what the two hand-rolled versions got wrong.
    const logout = () => signOut();
    const navigate = useNavigate();
    const {
        count: pendingCount,
        totalPending,
        items: pendingItems,
        refresh: refreshPending,
        dismiss: dismissPending,
    } = usePendingApprovals();

    return (
        <header className="shell-navbar">
            {/* Only rendered visibly on phones - the sidebar is hidden there, so this is the
                sole way to reach navigation. */}
            <button type="button" className="shell-hamburger" aria-label="Open navigation" onClick={onOpenMobileNav}>
                <Menu size={20} />
            </button>
            <span className="text-heading-page shell-navbar-title">{heading}</span>
            {analyticsSource && (
                <div className="shell-segmented" role="tablist" aria-label="Analytics data source">
                    <button
                        type="button"
                        role="tab"
                        aria-selected={analyticsSource === "invoiced"}
                        className={["shell-segmented-btn", analyticsSource === "invoiced" ? "active" : ""].filter(Boolean).join(" ")}
                        onClick={() => analyticsSource !== "invoiced" && onToggleAnalyticsSource()}
                    >
                        Invoiced
                    </button>
                    <button
                        type="button"
                        role="tab"
                        aria-selected={analyticsSource === "all"}
                        className={["shell-segmented-btn", analyticsSource === "all" ? "active" : ""].filter(Boolean).join(" ")}
                        onClick={() => analyticsSource !== "all" && onToggleAnalyticsSource()}
                    >
                        All Entries
                    </button>
                </div>
            )}
            <div className="shell-navbar-spacer" />
            {/* A job-id can be started from wherever you happen to be - taking a new order is
                not something you do only while looking at the Jobs board. */}
            {onCreateJob && hasAccess("lifecycle") && (
                <button
                    className="shell-icon-btn shell-navbar-create"
                    type="button"
                    aria-label="Create job-id"
                    title="Create job-id"
                    onClick={onCreateJob}
                >
                    <Plus size={17} />
                </button>
            )}
            {/* Between the create button and the theme toggle. Hidden for anyone who cannot
                approve - a queue of things you may not act on is only clutter. */}
            {hasAccess("purchase_orders_approve") && (
                <NotificationBell
                    count={pendingCount}
                    totalPending={totalPending}
                    items={pendingItems}
                    onOpen={refreshPending}
                    onDismiss={dismissPending}
                    // No id means "just take me to the list" - the panel's own link when
                    // everything has been marked read but orders are still unapproved.
                    onSelect={(id) => navigate(id ? `/admin/purchase-orders?open=${id}` : "/admin/purchase-orders")}
                />
            )}
            <button
                className="shell-icon-btn"
                type="button"
                aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
                onClick={onToggleTheme}
            >
                {theme === "dark" ? (
                    <Sun size={16} color="#f5c518" />
                ) : (
                    <Moon size={16} color="var(--text-primary)" fill="var(--text-primary)" />
                )}
            </button>
            {hasAccess("trash") && (
                <Link className="shell-icon-btn" to="/admin/trash" aria-label="Trash">
                    <Trash2 size={16} />
                </Link>
            )}
            <button className="shell-icon-btn" type="button" aria-label="Logout" onClick={logout}>
                <LogOut size={16} />
            </button>
        </header>
    );
};

export default AppNavbar;
