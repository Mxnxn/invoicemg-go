import React from "react";
import ToastProvider from "../global/ToastProvider";
import { UndoDeleteProvider } from "../Common/undoDelete";

import AdminLayout from "../Layout/Component/AdminLayout";
import DevLayout from "../Layout/Component/DevLayout";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import PreClientComponents from "../Views/Client/component/PreClientComponents";
import { ProtectiveRoute } from "../Views/Auth/ProtectiveRoute";
import PreSheetComponents from "../Views/Sheet/component/PreSheetComponent";
import PreInvoiceComponents from "../Views/Invoice/Component/PreInvoiceComponents";
import PrePurchaseOrder from "../Views/PurchaseOrder/PrePurchaseOrder";
import AlertJobPage from "../Views/Alert/component/AlertJobPage";

// Lazy so neither the landing page's JavaScript nor its Tailwind stylesheet is
// ever loaded on an admin route.
const LandingPage = React.lazy(() => import("../Landing"));

export default function App() {
    return (
        <>
            {/* Outside the router deliberately: a toast raised by a redirect (an expired
                session, a 403 bouncing an employee elsewhere) must survive the navigation
                that raised it. */}
            <ToastProvider />
            <>
                <UndoDeleteProvider>
                <BrowserRouter basename="/">
                    <Routes>
                        <Route
                            path="/admin/*"
                            element={
                                <ProtectiveRoute>
                                    {/* Superadmins get their own shell - see DevLayout. The
                                        server gate (requireSuperAdmin) is what actually
                                        protects /dev/*; this only picks the chrome. */}
                                    {window.localStorage.getItem("role") === "superadmin" ? <DevLayout /> : <AdminLayout />}
                                </ProtectiveRoute>
                            }
                        />
                        <Route
                            path="/customer/:id"
                            element={
                                <ProtectiveRoute>
                                    <PreClientComponents />
                                </ProtectiveRoute>
                            }
                        />
                        <Route
                            path="/"
                            element={
                                <React.Suspense fallback={null}>
                                    <LandingPage />
                                </React.Suspense>
                            }
                        />
                        <Route
                            path="/sheet/:id"
                            element={
                                <ProtectiveRoute>
                                    <PreSheetComponents />
                                </ProtectiveRoute>
                            }
                        />
                        <Route
                            path="/invoice/:id"
                            element={
                                <ProtectiveRoute>
                                    <PreInvoiceComponents />
                                </ProtectiveRoute>
                            }
                        />
                        {/* The WhatsApp "View job details" link. Outside ProtectiveRoute, unlike
                            the /customer and /invoice pages above: those wrap it and fall back
                            to the login screen, which is the wrong answer for a customer who
                            has no account at all. */}
                        <Route path="/alerts/:job_id/job/:jobcard_id" element={<AlertJobPage />} />
                        {/* The link a supplier opens to read a purchase order. Outside
                            ProtectiveRoute for the same reason as the alert page above - the
                            reader is a supplier with no account. Case matters: this path is
                            pasted into messages, so it is /PO/ exactly. */}
                        <Route path="/PO/:supplier_id/:po_id" element={<PrePurchaseOrder />} />
                    </Routes>
                </BrowserRouter>
                </UndoDeleteProvider>
            </>
        </>
    );
}
