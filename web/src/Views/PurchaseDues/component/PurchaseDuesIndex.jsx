import React, { useEffect, useMemo, useState } from "react";
import { Users, CreditCard, Clock, Phone } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import StatCard from "../../../Shell/StatCard";
import ReportDownloads from "../../../Common/reports/ReportDownloads";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { purchaseReportBackend } from "../../PurchaseReport/purchase_report_backend";
import { supplierLabel, formatOwed, outstandingSuppliers, settledSupplierCount } from "../../PurchaseReport/purchaseReportMath";

// What we owe, per supplier. The payables counterpart of Customer Dues, and deliberately NOT
// the same screen as Purchase Report: that one is a ledger - pick a supplier, read their
// invoices and payments in order. This one is a chase list, answering "who needs paying and
// how much" without asking you to select anyone first.
//
// Both read /purchase-report/dues, so the figures cannot drift: the math lives once, in
// Helpers/SupplierDues.js.
const PurchaseDuesIndex = () => {
    const [rows, setRows] = useState([]);
    const [totals, setTotals] = useState(null);
    const [loading, setLoading] = useState(true);
    const [showSettled, setShowSettled] = useState(false);

    useEffect(() => {
        purchaseReportBackend
            .dues()
            .then((res) => {
                setRows(res.data.rows);
                setTotals(res.data.totals);
            })
            .catch(() => {})
            .finally(() => setLoading(false));
    }, []);

    // Settled suppliers are hidden by default - the report exists to show who is still owed,
    // and a long tail of zero rows buries the few that need paying.
    const visibleRows = useMemo(() => (showSettled ? rows : outstandingSuppliers(rows)), [rows, showSettled]);

    // Honours Account settings > Appearance > Rows per page, like every other list.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(visibleRows);
    const hiddenCount = settledSupplierCount(rows);

    const exportColumns = [
        { label: "Supplier", width: "34%" },
        { label: "Phone", width: "16%" },
        { label: "Purchased", width: "16%", align: "right" },
        { label: "Paid", width: "16%", align: "right" },
        { label: "Owed", width: "18%", align: "right" },
    ];
    const exportRows = visibleRows.map((row) => [
        supplierLabel(row),
        row.supplierPhone || "",
        RoundOff(row.billed),
        RoundOff(row.paid),
        RoundOff(row.due),
    ]);

    if (loading) return null;

    return (
        <>
            {totals && (
                <div
                    className="stats-compact"
                    style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 10, marginBottom: 16 }}
                >
                    <StatCard standalone label="Total Owed" value={`₹${RoundOff(totals.due)}`} icon={Clock} accent />
                    <StatCard standalone label="Suppliers Owed" value={totals.outstandingSuppliers} icon={Users} />
                    <StatCard standalone label="Total Paid" value={`₹${RoundOff(totals.paid)}`} icon={CreditCard} />
                </div>
            )}

            <div className="shell-card">
                <div className="shell-card-header" style={{ flexWrap: "wrap", gap: 12 }}>
                    <span className="text-heading-brand">Purchase Dues</span>
                    <div className="d-flex align-items-center" style={{ gap: 12, marginLeft: "auto" }}>
                        {hiddenCount > 0 && (
                            <button
                                type="button"
                                className="shell-btn shell-btn-secondary"
                                onClick={() => setShowSettled((v) => !v)}
                            >
                                {showSettled ? "Hide settled" : `Show ${hiddenCount} settled`}
                            </button>
                        )}
                        <ReportDownloads
                            title="Purchase Dues"
                            columns={exportColumns}
                            rows={exportRows}
                            foot={["Total", "", RoundOff(totals?.billed), RoundOff(totals?.paid), RoundOff(totals?.due)]}
                        />
                    </div>
                </div>

                <DataTable loading={loading}>
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
                        {visibleRows.length === 0 ? (
                            <tr>
                                <td colSpan={5} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                    Nothing owed to any supplier.
                                </td>
                            </tr>
                        ) : (
                            pageRows.map((row) => (
                                <tr key={row.supplierId}>
                                    <td className="text-body-medium">{supplierLabel(row)}</td>
                                    <td className="cell-mono">
                                        {row.supplierPhone ? (
                                            <span className="d-flex align-items-center" style={{ gap: 6 }}>
                                                <Phone size={12} style={{ color: "var(--text-tertiary)" }} />
                                                {row.supplierPhone}
                                            </span>
                                        ) : (
                                            <span style={{ color: "var(--text-tertiary)" }}>—</span>
                                        )}
                                    </td>
                                    <td className="cell-mono">₹{RoundOff(row.billed)}</td>
                                    <td className="cell-mono">₹{RoundOff(row.paid)}</td>
                                    {/* Owed is the number the screen exists for, so it carries the
                                        weight. A negative due is credit with the supplier, not a
                                        debt - formatOwed says so rather than printing "-1500". */}
                                    <td
                                        className="cell-mono"
                                        style={{ fontWeight: 700, color: row.due > 0 ? "var(--status-red-text)" : "var(--text-tertiary)" }}
                                    >
                                        {formatOwed(row.due)}
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </DataTable>
                <div className="table-foot">
                    <span className="text-body-small table-foot-count">
                        {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} suppliers` : `${total} suppliers`}
                    </span>
                    <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                </div>

            </div>
        </>
    );
};

export default PurchaseDuesIndex;
