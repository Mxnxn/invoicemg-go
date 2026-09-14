import React, { useState, useEffect, useLayoutEffect, useCallback } from "react";
import { Route, Routes, Navigate, useLocation } from "react-router-dom";
import SideMenu from "../../Shell/SideMenu";
import { PendingApprovalsProvider } from "../../Shell/pendingApprovals";
import AppNavbar from "../../Shell/AppNavbar";
import CreateJobModal from "../../Views/Lifecycle/component/CreateJobModal";
import { lifecycleBackend } from "../../Views/Lifecycle/lifecycle_backend";
import { notifySuccess } from "../../global/toast";
import { publishJobCreated } from "../../Common/jobStore";
import "../../Shell/shell.css";
import Index from "../../Views/Index";
import { toggleThemeWithReveal } from "../../Common/themeTransition";

import UserProfileIndex from "../../Views/UserProfile/component/UserProfileIndex";
import { userBackend } from "../../Views/UserProfile/user_backend";
import ConfigureIndex from "../../Views/Configure/ConfigureIndex";
import Trash from "../../Views/Trash/components/Trash";
import InvoiceIndex from "../../Views/Invoice/Component/InvoiceIndex";
import AnalyticsIndex from "../../Views/Analytics/component/AnalyticsIndex";
import LifecycleIndex from "../../Views/Lifecycle/component/LifecycleIndex";
import QuotationsIndex from "../../Views/Quotation/QuotationsIndex";
import PurchaseInvoiceIndex from "../../Views/PurchaseInvoice/component/PurchaseInvoiceIndex";
import PurchaseOrderIndex from "../../Views/PurchaseOrder/component/PurchaseOrderIndex";
import MoreIndex from "../../Views/More/MoreIndex";
import BankTransfersIndex from "../../Views/BatchReceive/component/BankTransfersIndex";

const AdminLayout = ({ uid }) => {
    // Phone-sized screens hide the sidebar entirely and reach it through the navbar's
    // hamburger; this is that drawer's open state. Deliberately not persisted - a drawer
    // should never be open on first paint.
    const [mobileNavOpen, setMobileNavOpen] = useState(false);
    // The job-id form lives at the layout, not in a view, so the navbar button works from
    // wherever you are - including screens that have nothing to do with jobs.
    const [createJobOpen, setCreateJobOpen] = useState(false);

    const [collapsed, setCollapsed] = useState(() => {
        const stored = window.localStorage.getItem("sidebar_collapsed");
        return stored === null ? false : stored === "true";
    });

    const toggleSidebarCollapsed = () => {
        setCollapsed((prev) => !prev);
    };

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
        // Layout effect (not a passive one) so it runs synchronously inside the
        // flushSync() the reveal-animation toggle wraps state updates in - otherwise the
        // View Transition API snapshots the DOM before data-theme actually flips.
        window.localStorage.setItem("theme", theme);
        // Mirrored onto <html> so portaled content (toasts, etc.) that renders outside
        // the .shell-root subtree can still react to the current theme.
        document.documentElement.setAttribute("data-theme", theme);
    }, [theme]);

    const [email] = useState(window.localStorage.getItem("email"));
    const role = window.localStorage.getItem("role") || "admin";

    const [analyticsSource, setAnalyticsSource] = useState("invoiced");

    const [heading, setHeading] = useState("Home");
    const location = useLocation();
    const path = location.pathname;

    // Navigating should dismiss the drawer, or it stays parked over the page you just opened.
    useEffect(() => {
        setMobileNavOpen(false);
    }, [location.pathname]);

    // The heading is the URL segment, which is fine while the two agree. Where a section was
    // renamed in the UI but kept its path - so existing links and bookmarks still work -
    // the override below keeps the title matching the sidebar instead of showing the old name.
    const HEADING_OVERRIDES = { more: "Reports", lifecycle: "Jobs" };

    useEffect(() => {
        const temp = path.split("/");
        if (temp[2] === "" || !temp[2]) {
            return setHeading("Home");
        } else {
            return setHeading(HEADING_OVERRIDES[temp[2]] || temp[2]);
        }
        // HEADING_OVERRIDES is a module-level constant in effect - re-created each render but
        // never changing, so it is deliberately not a dependency.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [path]);

    const [state, setState] = useState({
        clients: [],
        dates: [],
        user: {},
    });

    const getUserInfo = useCallback(async () => {
        if (uid) {
            const formData = new FormData();
            formData.set("uid", uid);
            const res = await userBackend.getUserInfo(formData, window.localStorage.getItem("session_token"));
            setState({
                ...state,
                user: res.data,
            });
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [uid]);

    useEffect(() => {
        // /userinfo is admin-only (see routes/UserInfo.js) — employee sessions would get
        // a 403 "Forbidden" toast from the global axios interceptor on every page load.
        if (role !== "employee") {
            getUserInfo();
        }
    }, [getUserInfo, role]);

    return (
        // Wraps the whole shell: the navbar reads the count, and the purchase-order routes
        // inside refresh it.
        <PendingApprovalsProvider>
        <div className="shell-root" data-shell data-theme={theme}>
            <SideMenu
                collapsed={collapsed}
                onToggleCollapsed={toggleSidebarCollapsed}
                email={email}
                mobileOpen={mobileNavOpen}
                onCloseMobile={() => setMobileNavOpen(false)}
            />
            {/* Backdrop only exists while the drawer is open, so it never intercepts clicks
                on desktop. */}
            {mobileNavOpen && <div className="shell-nav-backdrop" onClick={() => setMobileNavOpen(false)} />}
            <div className="shell-main">
                <AppNavbar
                    onOpenMobileNav={() => setMobileNavOpen(true)}
                    heading={heading}
                    theme={theme}
                    onToggleTheme={toggleTheme}
                    analyticsSource={heading === "analytics" ? analyticsSource : null}
                    onToggleAnalyticsSource={() => setAnalyticsSource((prev) => (prev === "invoiced" ? "all" : "invoiced"))}
                    onCreateJob={() => setCreateJobOpen(true)}
                />
                <div className="shell-content">
                    <Routes>
                        <Route index element={<Navigate to="analytics" replace />} />
                        <Route path="dashboard" element={<Index />} />
                        <Route path="trash" element={<Trash />} />
                        <Route path="invoices" element={<InvoiceIndex user={state.user} />} />
                        <Route path="purchase-invoices" element={<PurchaseInvoiceIndex />} />
                        <Route path="purchase-orders" element={<PurchaseOrderIndex uid={uid} />} />
                        <Route path="quotations" element={<QuotationsIndex user={state.user} />} />
                        <Route path="more" element={<MoreIndex />} />
                        <Route path="more/:section" element={<MoreIndex />} />
                        <Route path="bank-transfers" element={<BankTransfersIndex />} />
                        <Route path="accounts" element={<UserProfileIndex />} />
                        <Route path="configure" element={<ConfigureIndex />} />
                        <Route path="configure/:section" element={<ConfigureIndex />} />
                        {/* Customers/Products/Suppliers/Logs moved under Configure - keep old bookmarks/links working. */}
                        <Route path="customers" element={<Navigate to="/admin/configure/customers" replace />} />
                        <Route path="products" element={<Navigate to="/admin/configure/products" replace />} />
                        <Route path="suppliers" element={<Navigate to="/admin/configure/suppliers" replace />} />
                        <Route path="challan" element={<Navigate to="/admin/configure/challan" replace />} />
                        <Route path="analytics" element={<AnalyticsIndex uid={uid} source={analyticsSource} />} />
                        <Route path="lifecycle" element={<LifecycleIndex />} />
                    </Routes>
                </div>
            </div>
            <CreateJobModal
                isOpen={createJobOpen}
                toggle={() => setCreateJobOpen(false)}
                onCreate={(formData) =>
                    lifecycleBackend.createJob(formData).then((res) => {
                        // The Jobs board keeps its own list and cannot see this modal, so the
                        // new job-id is announced rather than left to a page reload.
                        publishJobCreated(res.data);
                        notifySuccess(`Job-id ${res.data?.challanNumber || ""} created.`);
                        return res;
                    })
                }
            />
        </div>
        </PendingApprovalsProvider>
    );
};
export default AdminLayout;
