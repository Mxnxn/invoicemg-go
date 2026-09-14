import React, { Suspense, lazy } from "react";

const PDFPreview = lazy(() => import("./PDFPreview"));

// Renders whichever template is currently focused (hovered or selected) in
// TemplatesManager.jsx, using demo data - see templatePreviewDemoData.js.
const TemplatePreviewPanel = ({ Component, componentProps, label }) => (
    <div className="shell-card" style={{ height: 560, display: "flex", flexDirection: "column" }}>
        <div className="shell-card-header">
            <span className="text-heading-brand">Preview{label ? ` — ${label}` : ""}</span>
        </div>
        <div style={{ flex: 1, minHeight: 0 }}>
            <Suspense
                fallback={
                    <div className="text-body-small" style={{ padding: 20, color: "var(--text-tertiary)" }}>
                        Loading preview...
                    </div>
                }
            >
                <PDFPreview Component={Component} componentProps={componentProps} />
            </Suspense>
        </div>
    </div>
);

export default TemplatePreviewPanel;
