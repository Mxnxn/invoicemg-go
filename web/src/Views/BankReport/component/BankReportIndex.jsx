import React, { useCallback, useEffect, useState } from "react";
import { Input } from "reactstrap";
import { Briefcase, TrendingUp, TrendingDown, CreditCard, Download, FileText } from "react-feather";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import defaultDateRange from "../../../Common/DateAndTime/defaultRange";
import DataTable from "../../../Common/DataTable/DataTable";
import StatCard from "../../../Shell/StatCard";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { bankReportBackend } from "../bank_report_backend";
import { triggerDownload } from "../downloads";
import { downloadName } from "../../../Common/downloadName";
import { exportWorkbook } from "../../../Common/reports/exportWorkbook";
import { companyBackend } from "../../../Common/company_backend";
import { notifyError } from "../../../global/toast";
import "../bankReport.css";
import DateField from "../../../Common/DateField";
import Blank from "../../../Common/DataTable/Blank";

const money = (n) => `₹${RoundOff(n || 0)}`;

// A bank column beats one tab per bank when the point is usually to filter or pivot the lot
// in Excel, so this stays one flat sheet rather than one sheet per bank.
const BANK_COLUMNS = [
    { key: "bank", label: "Bank" },
    { key: "date", label: "Date" },
    { key: "type", label: "Type" },
    { key: "notes", label: "Notes" },
    { key: "credit", label: "Credit", numeric: true },
    { key: "expense", label: "Expense", numeric: true },
    { key: "balance", label: "Balance", numeric: true },
];

const BankReportIndex = () => {
    const [data, setData] = useState({ rows: [], totals: null, banks: [] });
    const [bankId, setBankId] = useState("");
    // The picker shows a name; the query needs an id. Empty name = the "All banks" default.
    const [bankName, setBankName] = useState("");
    const [from, setFrom] = useState(() => defaultDateRange().from);
    const [to, setTo] = useState(() => defaultDateRange().to);
    const [loading, setLoading] = useState(true);

    // `live` drops the response of a request the user has already moved on from. Typing in
    // a date field fires a request per keystroke-ish change, and they do not come back in
    // the order they were sent - without this, a slower earlier request can land last and
    // repaint the table with the range the user just left.
    const load = useCallback((isLive = () => true) => {
        setLoading(true);
        const formData = new FormData();
        if (bankId) formData.set("bank_id", bankId);
        if (from) formData.set("from", from);
        if (to) formData.set("to", to);
        bankReportBackend
            .report(formData)
            .then((res) => isLive() && setData(res.data))
            .catch(() => {})
            .finally(() => isLive() && setLoading(false));
    }, [bankId, from, to]);

    useEffect(() => {
        let live = true;
        load(() => live);
        return () => {
            live = false;
        };
    }, [load]);

    const { rows, totals, banks } = data;

    // Same text the PDF prints in its title (BankReportPDF) - the XLSX carried no date range
    // at all before, the one asymmetry between the two that this plan exists to close.
    const dateRangeSubtitle = from || to ? `${from ? getDate(from) : "Start"} to ${to ? getDate(to) : "Today"}` : undefined;

    const downloadXLSX = async () => {
        const tableRows = [];
        rows.forEach((bank) => {
            // The opening figure is the first row of bank.rows now, so it is no longer
            // written separately - it used to be pushed here and would appear twice.
            bank.rows.forEach((tx) => {
                // Debits are stored negative; the column carries the direction, so the figure
                // itself is written as a magnitude.
                // The opening balance is not money that moved, so it carries neither
                // column - only the balance it sets the list off from.
                const credit = !tx.opening && tx.amount >= 0 ? RoundOff(tx.amount) : "";
                const expense = !tx.opening && tx.amount < 0 ? RoundOff(Math.abs(tx.amount)) : "";
                tableRows.push([bank.bankName, tx.date, tx.type, tx.note || "", credit, expense, RoundOff(tx.balance)]);
            });
            tableRows.push([bank.bankName, "", "Current", "", "", "", RoundOff(bank.current)]);
        });

        // The company, not a cached `user` record: two tabs can act as two companies at once,
        // so a letterhead read from the user record can carry the wrong firm - fetched fresh
        // here at click time, same reasoning as ReportDownloads.
        try {
            const res = await companyBackend.activeCompany().catch(() => null);
            const company = res?.data || null;
            const blob = await exportWorkbook({
                title: "Bank Report",
                subtitle: dateRangeSubtitle,
                columns: BANK_COLUMNS,
                rows: tableRows,
                company,
                template: company?.exportTemplate || {},
                logoUrl: company?.url ? `${import.meta.env.VITE_API_URL}/uploads/${company.url}` : "",
            });
            triggerDownload(blob, downloadName({ firm: company?.firm, ext: "xlsx", suffix: "Bank Report" }));
        } catch {
            notifyError("Couldn't generate the download - please try again.");
        }
    };

    const downloadPDF = async () => {
        // @react-pdf/renderer has a top-level require() that is invalid in a browser ESM
        // bundle, so it is only imported when a download is actually asked for - same reason
        // Invoice/Component/InvoicePreviewModal.js lazy-loads its template. The company, not a
        // cached `user` record - same reasoning as downloadXLSX above. Wrapped in try/catch
        // like downloadXLSX: a failed chunk load or render is otherwise an unhandled rejection
        // behind a button that never re-enables anything to say why.
        try {
            const [{ pdf }, { default: BankReportPDF }, res] = await Promise.all([
                import("@react-pdf/renderer"),
                import("../BankReportPDF"),
                companyBackend.activeCompany().catch(() => null),
            ]);
            const company = res?.data || null;
            const blob = await pdf(
                <BankReportPDF report={{ rows, totals }} user={company} from={from} to={to} />
            ).toBlob();
            triggerDownload(blob, downloadName({ firm: company?.firm, ext: "pdf", suffix: "Bank Report" }));
        } catch {
            notifyError("Couldn't generate the download - please try again.");
        }
    };



    return (
        <div>
            {totals && (
                <div className="bank-stat-row stats-compact">
                    <StatCard standalone label="Opening" value={money(totals.opening)} icon={Briefcase} />
                    <StatCard standalone label="In" value={money(totals.credits)} icon={TrendingUp} />
                    <StatCard standalone label="Out" value={money(totals.debits)} icon={TrendingDown} />
                    <StatCard standalone label="Current" value={money(totals.current)} icon={CreditCard} accent />
                </div>
            )}

            <div className="bank-report-toolbar">
                {/* Searchable, like every other picker in the app - a plain <select> meant
                    scrolling to find a bank once there were more than a handful. Clearing it
                    (the x) returns to All banks. */}
                <div style={{ maxWidth: 220, width: "100%" }}>
                    <AssigneeDropdown
                        value={bankName}
                        placeholder="All banks"
                        options={(banks || []).map((b) => ({ id: b._id, name: b.name }))}
                        fullWidth
                        onSelect={(id, name) => {
                            setBankId(id || "");
                            setBankName(name || "");
                        }}
                    />
                </div>
                <DateField style={{ maxWidth: 170 }} value={from} onChange={(e) => setFrom(e.target.value)} />
                <DateField style={{ maxWidth: 170 }} value={to} onChange={(e) => setTo(e.target.value)} />
                <div className="bank-report-actions report-downloads">
                    <button
                        type="button"
                        className="shell-btn shell-btn-xlsx"
                        onClick={downloadXLSX}
                        disabled={rows.length === 0}
                    >
                        <Download size={14} /> XLSX
                    </button>
                    <button
                        type="button"
                        className="shell-btn shell-btn-pdf"
                        onClick={downloadPDF}
                        disabled={rows.length === 0}
                    >
                        <FileText size={14} /> PDF
                    </button>
                </div>
            </div>

            {loading ? (
                <div className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                    Loading…
                </div>
            ) : rows.length === 0 ? (
                <div className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                    No bank activity yet. Batch receives, supplier payments and expenses show up here.
                </div>
            ) : (
                rows.map((bank) => (
                    <div key={bank.bankId} style={{ marginBottom: 24 }}>
                        <div
                            style={{
                                display: "flex",
                                gap: 14,
                                alignItems: "baseline",
                                flexWrap: "wrap",
                                marginBottom: 8,
                            }}
                        >
                            <span className="text-heading-brand">{bank.bankName}</span>
                            <span className="text-body-small" style={{ color: "var(--text-secondary)" }}>
                                Opening {money(bank.opening)} · In {money(bank.credits)} · Out {money(bank.debits)}
                            </span>
                            <span
                                className="cell-mono"
                                style={{
                                    marginLeft: "auto",
                                    fontWeight: 700,
                                    color: bank.current < 0 ? "var(--status-red-text)" : "var(--text-primary)",
                                }}
                            >
                                Current {money(bank.current)}
                            </span>
                        </div>
                        <DataTable loading={loading}>
                            <thead>
                                <tr>
                                    <th scope="col">Date</th>
                                    <th scope="col">Type</th>
                                    <th scope="col">Notes</th>
                                    <th scope="col">Credit</th>
                                    <th scope="col">Expense</th>
                                    <th scope="col">Balance</th>
                                </tr>
                            </thead>
                            <tbody>
                                {bank.rows.length === 0 ? (
                                    <tr>
                                        <td colSpan={6} className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                            Nothing in this range.
                                        </td>
                                    </tr>
                                ) : (
                                    bank.rows.map((tx, i) => (
                                        <tr key={`${bank.bankId}-${i}`} className={tx.opening ? "row-opening" : ""}>
                                            <td className="cell-mono">{tx.date || <Blank />}</td>
                                            <td>{tx.type}</td>
                                            <td className="text-body-small" style={{ color: "var(--text-secondary)" }}>
                                                {tx.note || <Blank />}
                                            </td>
                                            {/* The opening balance sets where the list starts
                                                rather than moving money, so it fills neither the
                                                credit nor the expense column. */}
                                            <td className="cell-mono" style={{ color: "var(--status-lime-text)" }}>
                                                {!tx.opening && tx.amount >= 0 ? money(tx.amount) : <Blank />}
                                            </td>
                                            <td className="cell-mono" style={{ color: "var(--status-red-text)" }}>
                                                {!tx.opening && tx.amount < 0 ? money(Math.abs(tx.amount)) : <Blank />}
                                            </td>
                                            <td className="cell-mono">{money(tx.balance)}</td>
                                        </tr>
                                    ))
                                )}
                            </tbody>
                        </DataTable>
                    </div>
                ))
            )}
        </div>
    );
};

export default BankReportIndex;
