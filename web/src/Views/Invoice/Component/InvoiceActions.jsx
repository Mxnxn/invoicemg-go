import React, { useState } from "react";
import { Download, ExternalLink } from "react-feather";
import { getInvoiceTemplate } from "../Template/registry";
import { downloadName } from "../../../Common/downloadName";

// Download / open-in-tab for an invoice, and the whole story on phones.
//
// @react-pdf/renderer's PDFViewer renders into an <iframe>. Mobile browsers do not display
// PDFs inside an iframe - Chrome on Android shows a placeholder with an "Open" button that
// cannot escape the frame, which is why the preview looked dead on a phone. So on narrow
// screens the viewer is replaced by these actions: a real download, and opening the blob in
// a tab, both of which mobile handles natively.
//
// The blob is built on demand rather than up front - generating a PDF for a preview nobody
// asked to keep is wasted work on a phone.
const InvoiceActions = ({ invoice, templateKey, fontKey, scaleId, compact = false }) => {
    const [busy, setBusy] = useState("");

    const filename = downloadName({
        // The invoice is about the client, so the client's firm names the file.
        firm: invoice?.clientFirm || invoice?.clientName,
        ext: "pdf",
        date: invoice?.date,
    });

    const buildBlob = async () => {
        const { pdf } = await import("@react-pdf/renderer");
        const Template = getInvoiceTemplate(templateKey || "classic");
        return pdf(<Template invoice={invoice} fontKey={fontKey} scaleId={scaleId} />).toBlob();
    };

    const onDownload = async () => {
        setBusy("download");
        try {
            const url = URL.createObjectURL(await buildBlob());
            const link = document.createElement("a");
            link.href = url;
            link.download = filename;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            URL.revokeObjectURL(url);
        } finally {
            setBusy("");
        }
    };

    // iOS in particular renders a PDF opened as a top-level document but not one embedded in
    // a frame, so this is the reliable way to actually LOOK at it on a phone.
    const onOpen = async () => {
        setBusy("open");
        try {
            const url = URL.createObjectURL(await buildBlob());
            window.open(url, "_blank", "noopener");
            // Not revoked immediately - the new tab still needs the URL to load from.
            setTimeout(() => URL.revokeObjectURL(url), 60000);
        } finally {
            setBusy("");
        }
    };

    return (
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
            <button type="button" className="shell-btn shell-btn-primary" onClick={onDownload} disabled={!!busy}>
                <Download size={14} /> {busy === "download" ? "Preparing…" : "Download PDF"}
            </button>
            <button type="button" className="shell-btn shell-btn-secondary" onClick={onOpen} disabled={!!busy}>
                <ExternalLink size={14} /> {busy === "open" ? "Preparing…" : "Open in new tab"}
            </button>
            {compact && (
                <span className="text-body-small" style={{ color: "var(--text-tertiary)", width: "100%" }}>
                    Inline preview is not supported on phones — open or download the PDF instead.
                </span>
            )}
        </div>
    );
};

export default InvoiceActions;
