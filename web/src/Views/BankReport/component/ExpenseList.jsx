import React, { useCallback, useEffect, useState } from "react";
import { Input } from "reactstrap";
import { Download, FileText, Trash2 } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import defaultDateRange from "../../../Common/DateAndTime/defaultRange";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import { useUndoDelete } from "../../../Common/undoDelete";
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

const EXPENSE_COLUMNS = [
    { key: "date", label: "Date" },
    { key: "bank", label: "Bank" },
    { key: "notes", label: "Notes" },
    // Two money columns rather than one signed Amount, matching the table and the ledger
    // layout - expenses only ever fill the Expense column, Credit is here so the shape lines
    // up rather than because a real credit figure ever lands in it.
    { key: "credit", label: "Credit", numeric: true },
    { key: "expense", label: "Expense", numeric: true },
];

// Newest first, because an expense list is read as "what did we just spend", not as a
// running account - the Bank Report keeps the chronological view, where a running balance
// requires oldest-first. The API already sorts date desc (routes/Expense.js).
const ExpenseList = ({ reloadKey }) => {
    const [rows, setRows] = useState([]);
    const [from, setFrom] = useState(() => defaultDateRange().from);
    const [to, setTo] = useState(() => defaultDateRange().to);
    const [loading, setLoading] = useState(true);
    // Expenses used to delete on the first click, with no confirmation and no way back -
    // the only row in Bank Transfers that behaved that way. Same confirm-then-undo flow as
    // Received and Paid now.
    const [deleteTarget, setDeleteTarget] = useState(null);
    const { scheduleDelete } = useUndoDelete();

    // `live` drops the response of a request the user has already moved on from. Typing in
    // a date field fires a request per keystroke-ish change, and they do not come back in
    // the order they were sent - without this, a slower earlier request can land last and
    // repaint the table with the range the user just left.
    //
    // `load` stays a callback because the reloadKey prop re-runs it; the effect owns the
    // flag and hands it in, so a caller with no opinion gets the always-live default.
    const load = useCallback((isLive = () => true) => {
        setLoading(true);
        const formData = new FormData();
        if (from) formData.set("from", from);
        if (to) formData.set("to", to);
        bankReportBackend
            .listExpenses(formData)
            .then((res) => isLive() && setRows(res.data || []))
            .catch(() => {})
            .finally(() => isLive() && setLoading(false));
    }, [from, to]);

    useEffect(() => {
        let live = true;
        load(() => live);
        return () => {
            live = false;
        };
    }, [load, reloadKey]);

    const total = rows.reduce((sum, r) => sum + (Number(r.amount) || 0), 0);
    // The total above and the exports below deliberately use `rows`, not the page - a
    // footer that summed only what is visible would be wrong the moment paging kicks in.
    const { pageRows, page, setPage, perPage, total: rowCount } = usePagedRows(rows);

    // Same text the PDF prints in its title (ExpenseReportPDF) - the XLSX carried no date
    // range at all before, the one asymmetry between the two that this plan exists to close.
    const dateRangeSubtitle = from || to ? `${from ? getDate(from) : "Start"} to ${to ? getDate(to) : "Today"}` : undefined;

    const confirmDelete = () => {
        if (!deleteTarget) return;
        const row = deleteTarget;
        const index = rows.findIndex((r) => r._id === row._id);
        setRows((prev) => prev.filter((r) => r._id !== row._id));
        setDeleteTarget(null);
        scheduleDelete({
            label: "expense",
            commit: () => {
                const formData = new FormData();
                formData.set("expense_id", row._id);
                return bankReportBackend.removeExpense(formData);
            },
            undo: () => setRows((prev) => [...prev.slice(0, index), row, ...prev.slice(index)]),
        });
    };

    const downloadXLSX = async () => {
        try {
            const res = await companyBackend.activeCompany().catch(() => null);
            const company = res?.data || null;
            const blob = await exportWorkbook({
                title: "Expenses",
                subtitle: dateRangeSubtitle,
                columns: EXPENSE_COLUMNS,
                rows: rows.map((r) => [r.date, r.bank_id?.name || "", r.notes || "", "", RoundOff(r.amount)]),
                foot: ["", "", "Total", "", RoundOff(total)],
                company,
                template: company?.exportTemplate || {},
                logoUrl: company?.url ? `${import.meta.env.VITE_API_URL}/uploads/${company.url}` : "",
            });
            triggerDownload(blob, downloadName({ firm: company?.firm, ext: "xlsx", suffix: "Expenses" }));
        } catch {
            notifyError("Couldn't generate the download - please try again.");
        }
    };

    const downloadPDF = async () => {
        // The company, not a cached `user` record - same reasoning as downloadXLSX above.
        // Wrapped in try/catch like downloadXLSX: a failed chunk load or render is otherwise
        // an unhandled rejection behind a button that never re-enables anything to say why.
        try {
            const [{ pdf }, { default: ExpenseReportPDF }, res] = await Promise.all([
                import("@react-pdf/renderer"),
                import("../ExpenseReportPDF"),
                companyBackend.activeCompany().catch(() => null),
            ]);
            const company = res?.data || null;
            const blob = await pdf(<ExpenseReportPDF rows={rows} total={total} user={company} from={from} to={to} />).toBlob();
            triggerDownload(blob, downloadName({ firm: company?.firm, ext: "pdf", suffix: "Expenses" }));
        } catch {
            notifyError("Couldn't generate the download - please try again.");
        }
    };

    return (
        <div style={{ marginTop: 20 }}>
            <div className="bank-report-toolbar">
                <span className="text-heading-brand">Expenses</span>
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

            <DataTable loading={loading}>
                <thead>
                    <tr>
                        <th scope="col">Date</th>
                        <th scope="col">Bank</th>
                        <th scope="col">Notes</th>
                        {/* Two money columns rather than one signed Amount: read as a ledger,
                            credit on one side and spend on the other. Expenses only ever fill
                            the Expense column - the Credit column is here so the layout matches
                            the bank ledger and the export. */}
                        <th scope="col">Credit</th>
                        <th scope="col">Expense</th>
                        <th scope="col" />
                    </tr>
                </thead>
                <tbody>
                    {loading ? (
                        <tr>
                            <td colSpan={6} className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                Loading…
                            </td>
                        </tr>
                    ) : rows.length === 0 ? (
                        <tr>
                            <td colSpan={6} className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                No expenses in this range.
                            </td>
                        </tr>
                    ) : (
                        pageRows.map((row) => (
                            <tr key={row._id}>
                                <td className="cell-mono">{row.date}</td>
                                <td>{row.bank_id?.name || <Blank />}</td>
                                <td className="text-body-small" style={{ color: "var(--text-secondary)" }}>
                                    {row.notes || <Blank />}
                                </td>
                                <td className="cell-mono" style={{ color: "var(--text-tertiary)" }}>
                                    —
                                </td>
                                <td className="cell-mono" style={{ color: "var(--status-red-text)" }}>
                                    ₹{RoundOff(row.amount)}
                                </td>
                                <td>
                                    <button
                                        type="button"
                                        className="dev-icon-btn"
                                        aria-label="Delete expense"
                                        title="Delete"
                                        onClick={() => setDeleteTarget(row)}
                                    >
                                        <Trash2 size={14} />
                                    </button>
                                </td>
                            </tr>
                        ))
                    )}
                </tbody>
                {rows.length > 0 && (
                    <tfoot>
                        <tr>
                            <td colSpan={3} style={{ fontWeight: 600 }}>
                                Total
                            </td>
                            <td className="cell-mono" style={{ fontWeight: 600, color: "var(--text-tertiary)" }}>
                                —
                            </td>
                            <td className="cell-mono" style={{ fontWeight: 600, color: "var(--status-red-text)" }}>
                                ₹{RoundOff(total)}
                            </td>
                            <td />
                        </tr>
                    </tfoot>
                )}
            </DataTable>
            <div className="table-foot">
                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                    {perPage > 0 && rowCount > perPage ? `${pageRows.length} of ${rowCount} expenses` : `${rowCount} expenses`}
                </span>
                <Pagination totalItems={rowCount} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
            </div>

            <ConfirmDialog
                open={!!deleteTarget}
                message="Delete this expense? It stops counting against that bank."
                confirmLabel="Delete"
                position="bottom"
                danger
                onConfirm={confirmDelete}
                onCancel={() => setDeleteTarget(null)}
            />
        </div>
    );
};

export default ExpenseList;
