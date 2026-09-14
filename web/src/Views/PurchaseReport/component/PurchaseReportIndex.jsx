import React, { useEffect, useMemo, useState } from "react";
import { TrendingUp, Users, FileText, CreditCard } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import defaultDateRange from "../../../Common/DateAndTime/defaultRange";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import StatCard from "../../../Shell/StatCard";
import ReportDownloads from "../../../Common/reports/ReportDownloads";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { purchaseReportBackend } from "../purchase_report_backend";
import { formatOwed, outstandingSuppliers, settledSupplierCount, supplierLabel } from "../purchaseReportMath";
import DateField from "../../../Common/DateField";
import Blank from "../../../Common/DataTable/Blank";

// More > Purchase Report. Two views over the same payables data: a summary of every supplier,
// and one supplier's ledger. Entirely derived server-side (routes/PurchaseReport.js) from
// PurchaseInvoice and SupplierPayment - nothing is stored here.
const PurchaseReportIndex = () => {
    const [rows, setRows] = useState([]);
    const [totals, setTotals] = useState(null);
    const [loading, setLoading] = useState(true);
    const [showSettled, setShowSettled] = useState(false);
    const [supplierId, setSupplierId] = useState("");
    const [supplierName, setSupplierName] = useState("");
    const [ledger, setLedger] = useState(null);
    // Both were blank, which asked the server for every purchase ever recorded.
    const [from, setFrom] = useState(() => defaultDateRange().from);
    const [to, setTo] = useState(() => defaultDateRange().to);

    useEffect(() => {
        purchaseReportBackend
            .dues()
            .then((res) => {
                setRows(res.data.rows);
                setTotals(res.data.totals);
            })
            .finally(() => setLoading(false));
    }, []);

    // `live` drops the response of a request the user has already moved on from. Typing in
    // a date field fires a request per keystroke-ish change, and they do not come back in
    // the order they were sent - without this, a slower earlier request can land last and
    // repaint the table with the range the user just left.
    // Same for the supplier: a stale ledger under a freshly picked supplier reads as that
    // supplier's account.
    useEffect(() => {
        if (!supplierId) {
            setLedger(null);
            return undefined;
        }
        let live = true;
        purchaseReportBackend.supplierLedger({ supplierId, from, to }).then((res) => live && setLedger(res.data));
        return () => {
            live = false;
        };
    }, [supplierId, from, to]);

    const visibleRows = useMemo(() => (showSettled ? rows : outstandingSuppliers(rows)), [rows, showSettled]);

    // Honours Account settings > Appearance > Rows per page, like every other list.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(visibleRows);
    const hiddenCount = settledSupplierCount(rows);

    const supplierOptions = rows.map((row) => ({
        id: row.supplierId,
        name: supplierLabel(row),
        search: [row.supplierFirm, row.supplierName, row.supplierPhone].filter(Boolean).join(" ").toLowerCase(),
    }));

    return (
        <>
            {totals && (
                <div className="stats-compact" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 10, marginBottom: 16 }}>
                    <StatCard standalone label="Total Owed" value={`₹${RoundOff(totals.due)}`} icon={TrendingUp} accent />
                    <StatCard standalone label="Suppliers Owed" value={totals.outstandingSuppliers} icon={Users} />
                    <StatCard standalone label="Total Purchased" value={`₹${RoundOff(totals.billed)}`} icon={FileText} />
                    <StatCard standalone label="Total Paid" value={`₹${RoundOff(totals.paid)}`} icon={CreditCard} />
                </div>
            )}

            <div className="d-flex align-items-center" style={{ gap: 12, marginBottom: 12, flexWrap: "wrap" }}>
                <div style={{ minWidth: 260 }}>
                    <AssigneeDropdown
                        value={supplierName}
                        placeholder="All suppliers (pick one for its ledger)"
                        options={supplierOptions}
                        fullWidth
                        menuWidth={320}
                        onSelect={(id, name) => {
                            setSupplierId(id || "");
                            setSupplierName(name || "");
                        }}
                    />
                </div>
                {supplierId && (
                    <>
                        <DateField style={{ width: 160 }} value={from} onChange={(e) => setFrom(e.target.value)} />
                        <DateField style={{ width: 160 }} value={to} onChange={(e) => setTo(e.target.value)} />
                        <button
                            type="button"
                            className="shell-btn shell-btn-sm shell-btn-secondary"
                            onClick={() => {
                                setSupplierId("");
                                setSupplierName("");
                                setFrom("");
                                setTo("");
                            }}
                        >
                            Back to all suppliers
                        </button>
                    </>
                )}
                {!supplierId && hiddenCount > 0 && (
                    <button type="button" className="shell-btn shell-btn-sm shell-btn-secondary" onClick={() => setShowSettled((v) => !v)}>
                        {showSettled ? "Hide" : "Show"} {hiddenCount} settled {hiddenCount === 1 ? "supplier" : "suppliers"}
                    </button>
                )}
                {/* Exports the supplier summary - the settled filter above applies, so the
                    file matches the table. Drilling into one supplier switches the table to
                    that supplier's ledger; this stays on the summary either way. */}
                <ReportDownloads
                    title="Purchase Report"
                    summary={
                        totals
                            ? [
                                  { label: "Total Purchased", value: `₹${RoundOff(totals.billed)}` },
                                  { label: "Total Paid", value: `₹${RoundOff(totals.received)}` },
                                  { label: "Total Owed", value: `₹${RoundOff(totals.due)}` },
                              ]
                            : []
                    }
                    columns={[
                        { label: "Supplier", width: "40%" },
                        { label: "Purchased", width: "20%", align: "right" },
                        { label: "Paid", width: "20%", align: "right" },
                        { label: "Owed", width: "20%", align: "right" },
                    ]}
                    rows={visibleRows.map((r) => [r.supplierName, RoundOff(r.billed), RoundOff(r.received), RoundOff(r.due)])}
                    foot={totals ? ["Total", RoundOff(totals.billed), RoundOff(totals.received), RoundOff(totals.due)] : null}
                />
            </div>

            {supplierId && ledger ? (
                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">Sr</th>
                            <th scope="col">Date</th>
                            <th scope="col">Type</th>
                            <th scope="col">Reference</th>
                            <th scope="col">Bill</th>
                            <th scope="col">Paid</th>
                            <th scope="col">Balance</th>
                        </tr>
                    </thead>
                    <tbody>
                        {ledger.rows.length === 0 ? (
                            <tr>
                                <td colSpan={7} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                    Nothing recorded for this supplier in the selected range.
                                </td>
                            </tr>
                        ) : (
                            ledger.rows.map((row) => (
                                <tr key={row.sr}>
                                    <td className="cell-mono">{row.sr}</td>
                                    <td className="cell-mono">{row.date}</td>
                                    <td>{row.type}</td>
                                    <td>{row.reference || <Blank />}</td>
                                    <td className="cell-mono">{row.bill ? `₹${RoundOff(row.bill)}` : "—"}</td>
                                    <td className="cell-mono">{row.payment ? `₹${RoundOff(row.payment)}` : "—"}</td>
                                    <td className="cell-mono">₹{RoundOff(row.balance)}</td>
                                </tr>
                            ))
                        )}
                    </tbody>
                    <tfoot>
                        <tr>
                            <td colSpan={6} style={{ fontWeight: 600 }}>
                                Outstanding as of today
                            </td>
                            <td className="cell-mono" style={{ fontWeight: 600, color: "var(--xan-rose)" }}>
                                {formatOwed(ledger.currentBalance)}
                            </td>
                        </tr>
                    </tfoot>
                </DataTable>
            ) : (
                <>
                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">Supplier</th>
                            <th scope="col">Phone</th>
                            <th scope="col">Purchased</th>
                            <th scope="col">Paid</th>
                            <th scope="col">Owed</th>
                        </tr>
                    </thead>
                    <tbody>
                        {loading ? (
                            <tr>
                                <td colSpan={5} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                    Loading…
                                </td>
                            </tr>
                        ) : visibleRows.length === 0 ? (
                            <tr>
                                <td colSpan={5} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                    {rows.length === 0 ? "No purchase invoices recorded yet." : "Every supplier is settled up."}
                                </td>
                            </tr>
                        ) : (
                            pageRows.map((row) => (
                                <tr
                                    key={row.supplierId}
                                    style={{ cursor: "pointer" }}
                                    onClick={() => {
                                        setSupplierId(row.supplierId);
                                        setSupplierName(supplierLabel(row));
                                    }}
                                >
                                    <td>
                                        <div>{supplierLabel(row)}</div>
                                        {row.supplierFirm && row.supplierName && (
                                            <div className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                                {row.supplierName}
                                            </div>
                                        )}
                                    </td>
                                    <td className="cell-mono">{row.supplierPhone || <Blank />}</td>
                                    <td className="cell-mono">₹{RoundOff(row.billed)}</td>
                                    <td className="cell-mono">₹{RoundOff(row.paid)}</td>
                                    <td
                                        className="cell-mono"
                                        style={{ color: row.due > 0 ? "var(--xan-rose)" : row.due < 0 ? "var(--xan-blue)" : "var(--text-tertiary)" }}
                                    >
                                        {formatOwed(row.due)}
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                    {totals && visibleRows.length > 0 && (
                        <tfoot>
                            <tr>
                                <td colSpan={2} style={{ fontWeight: 600 }}>
                                    Total
                                </td>
                                <td className="cell-mono" style={{ fontWeight: 600 }}>
                                    ₹{RoundOff(totals.billed)}
                                </td>
                                <td className="cell-mono" style={{ fontWeight: 600 }}>
                                    ₹{RoundOff(totals.paid)}
                                </td>
                                <td className="cell-mono" style={{ fontWeight: 600, color: "var(--xan-rose)" }}>
                                    ₹{RoundOff(totals.due)}
                                </td>
                            </tr>
                        </tfoot>
                    )}
                </DataTable>
                <div className="table-foot">
                    <span className="text-body-small table-foot-count">
                        {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} rows` : `${total} rows`}
                    </span>
                    <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                </div>
                </>
            )}
        </>
    );
};

export default PurchaseReportIndex;
