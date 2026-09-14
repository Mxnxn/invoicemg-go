import React, { useEffect, useState } from "react";
import { Download, FileText } from "react-feather";
import { triggerDownload } from "../../Views/BankReport/downloads";
import { downloadName } from "../downloadName";
import { exportWorkbook } from "./exportWorkbook";
import { companyBackend } from "../company_backend";
import { notifyError } from "../../global/toast";
import "./reportDownloads.css";

// The XLSX and PDF pair, for every report in Reports.
//
// Each report knows its own columns and rows; everything else - the file name, the firm
// letterhead, the Blob-and-click plumbing, the lazy import of @react-pdf - is identical, so
// it lives here rather than being written out four more times.
//
// `columns` and `rows` are the same arrays the PDF takes, so the spreadsheet and the printed
// page can never show different columns.
const ReportDownloads = ({ title, subtitle, summary = [], columns = [], rows = [], foot = null, disabled = false }) => {
    const [company, setCompany] = useState(null);
    const [busy, setBusy] = useState("");

    // The company, not the user: two tabs can act as two companies at once, so a letterhead
    // read from the user record can carry the wrong firm. /company/active resolves through
    // the TAB-ID header, and is fetched here rather than cached globally for the same reason.
    useEffect(() => {
        companyBackend
            .activeCompany()
            .then((res) => setCompany(res.data || null))
            .catch(() => setCompany(null));
    }, []);

    const filename = (ext) => downloadName({ firm: company?.firm, ext, suffix: title });
    const nothingToExport = disabled || rows.length === 0;

    // Both handlers are fire-and-forget from the button's onClick, so a rejection here would
    // otherwise surface as nothing but a re-enabled button and an unhandled rejection in the
    // console - the errorInterceptor only watches axios, not this. exportWorkbook failing to
    // load ExcelJS, or @react-pdf/renderer failing to load, are real (if rare) failure modes
    // a download button needs to report, same as a network error would.
    const onXlsx = async () => {
        setBusy("xlsx");
        try {
            const blob = await exportWorkbook({
                title,
                subtitle,
                summary,
                columns,
                rows,
                foot,
                company,
                template: company?.exportTemplate || {},
                logoUrl: company?.url ? `${import.meta.env.VITE_API_URL}/uploads/${company.url}` : "",
            });
            triggerDownload(blob, filename("xlsx"));
        } catch {
            notifyError("Couldn't generate the spreadsheet - please try again.");
        } finally {
            setBusy("");
        }
    };

    const onPdf = async () => {
        setBusy("pdf");
        try {
            // @react-pdf has a top-level require() that is invalid in a browser ESM bundle,
            // so it is only pulled in when a download is actually asked for.
            const template = company?.exportTemplate || {};
            const wantedLogo = template.logo !== false && company?.url
                ? `${import.meta.env.VITE_API_URL}/uploads/${company.url}`
                : "";
            const [{ pdf }, { default: ReportPDF }] = await Promise.all([
                import("@react-pdf/renderer"),
                import("./ReportPDF"),
            ]);
            // The same two the spreadsheet gets, so Configure > Exports reaches both formats
            // rather than only the one.
            const render = (logoUrl) =>
                pdf(
                    <ReportPDF
                        title={title}
                        subtitle={subtitle}
                        user={company}
                        summary={summary}
                        columns={columns}
                        rows={rows}
                        foot={foot}
                        template={template}
                        logoUrl={logoUrl}
                    />
                ).toBlob();

            // @react-pdf throws on an <Image src> it cannot fetch, and that failure takes the
            // whole document with it - a company whose logo file has gone missing would lose
            // the PDF entirely rather than lose the logo. Retrying without it costs nothing in
            // the normal case and keeps the report available in the broken one. Only worth a
            // second attempt if there was a logo to blame.
            let blob;
            try {
                blob = await render(wantedLogo);
            } catch (logoError) {
                if (!wantedLogo) throw logoError;
                blob = await render("");
            }
            triggerDownload(blob, filename("pdf"));
        } catch {
            notifyError("Couldn't generate the PDF - please try again.");
        } finally {
            setBusy("");
        }
    };

    return (
        <div className="report-downloads">
            <button type="button" className="shell-btn shell-btn-xlsx" onClick={onXlsx} disabled={nothingToExport || !!busy}>
                <Download size={14} /> {busy === "xlsx" ? "Preparing…" : "XLSX"}
            </button>
            <button type="button" className="shell-btn shell-btn-pdf" onClick={onPdf} disabled={nothingToExport || !!busy}>
                <FileText size={14} /> {busy === "pdf" ? "Preparing…" : "PDF"}
            </button>
        </div>
    );
};

export default ReportDownloads;
