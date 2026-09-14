import React, { useEffect, useMemo, useState } from "react";
import { Check, Loader, Users, FileText, CreditCard, TrendingDown, Clock } from "react-feather";
import DataTable from "../../../Common/DataTable/DataTable";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import StatCard from "../../../Shell/StatCard";
import ReportDownloads from "../../../Common/reports/ReportDownloads";
import WhatsAppIcon from "../../../Common/WhatsAppIcon";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { ledgerBackend } from "../../Ledger/ledger_backend";
import { notifySuccess } from "../../../global/toast";
import { canRemind, displayName, formatDue, formatLastRemind, outstandingRows, remindedToday, settledCount } from "../duesMath";
import Blank from "../../../Common/DataTable/Blank";

// Every client's outstanding balance in one table. Entirely derived - /ledger/dues runs the
// same math as the Client Ledger (Helpers/ClientDues.js is shared by both), so a client's
// due here always equals their closing balance there.
const DuesIndex = () => {
    const [rows, setRows] = useState([]);
    const [totals, setTotals] = useState(null);
    const [loading, setLoading] = useState(true);
    const [showSettled, setShowSettled] = useState(false);
    const [sendingId, setSendingId] = useState(null);
    const [sentId, setSentId] = useState(null);

    useEffect(() => {
        ledgerBackend
            .dues()
            .then((res) => {
                setRows(res.data.rows);
                setTotals(res.data.totals);
            })
            .finally(() => setLoading(false));
    }, []);

    // Settled and overpaid clients are hidden by default - the report is about who owes
    // money, and a long tail of zero rows buries the handful that need chasing.
    const visibleRows = useMemo(() => (showSettled ? rows : outstandingRows(rows)), [rows, showSettled]);
    const hiddenCount = settledCount(rows);
    // Honours Account settings > Appearance > Rows per page. The export above still takes
    // `visibleRows`, so a downloaded chase list is never just the page on screen.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(visibleRows);

    // The row waiting on "you already chased them today - send anyway?".
    const [confirmRow, setConfirmRow] = useState(null);

    const onSendReminder = (row, force = false) => {
        if (!canRemind(row) || sendingId) return;
        // Asked here rather than letting the server's 409 come back, so the common case costs
        // no round trip and raises no error toast for something that is not an error. The
        // server enforces the same rule for a page left open since yesterday, or a colleague
        // who pressed it first.
        if (!force && remindedToday(row)) {
            setConfirmRow(row);
            return;
        }
        setConfirmRow(null);
        setSendingId(row.clientId);
        const name = displayName(row);
        const formData = new FormData();
        formData.set("client_id", row.clientId);
        if (force) formData.set("force", "true");
        // A template, not the free text this used to send: free text only reaches a customer
        // inside the 24-hour service window, and a debt worth chasing is almost always outside
        // it - so the old send was refused by Meta for most of the customers it was aimed at.
        ledgerBackend
            .remindDue(formData)
            .then((res) => {
                notifySuccess(`Reminder sent to ${name}.`);
                // So the column and the same-day guard are right immediately, without
                // refetching the whole report for one changed field.
                const at = res?.data?.lastRemindedAt || new Date().toISOString();
                setRows((prev) => prev.map((r) => (r.clientId === row.clientId ? { ...r, lastRemindedAt: at } : r)));
                setSentId(row.clientId);
                setTimeout(() => setSentId((current) => (current === row.clientId ? null : current)), 2000);
            })
            // Failures already surface through the global axios interceptor (src/api/errorInterceptor.js),
            // which shows Meta's own message - no per-call toast here.
            .finally(() => setSendingId(null));
    };

    return (
        <>
            {/* The same-day warning. Not a refusal - a wrong number corrected mid-morning is a
                real reason to send again - but it has to be said out loud, because the button
                cannot show that a colleague pressed it an hour ago. */}
            <ConfirmDialog
                open={Boolean(confirmRow)}
                message={
                    confirmRow
                        ? `${displayName(confirmRow)} was already reminded ${formatLastRemind(confirmRow).toLowerCase()}. Send another?`
                        : ""
                }
                confirmLabel="Send anyway"
                cancelLabel="Don't send"
                onConfirm={() => onSendReminder(confirmRow, true)}
                onCancel={() => setConfirmRow(null)}
            />
            {totals && (
                <div className="stats-compact" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 10, marginBottom: 16 }}>
                    <StatCard standalone label="Total Outstanding" value={`₹${RoundOff(totals.due)}`} icon={TrendingDown} accent />
                    <StatCard standalone label="Clients Owing" value={totals.outstandingClients} icon={Users} />
                    <StatCard standalone label="Total Billed" value={`₹${RoundOff(totals.billed)}`} icon={FileText} />
                    <StatCard standalone label="Total Received" value={`₹${RoundOff(totals.received)}`} icon={CreditCard} />
                    {/* Work done but not yet billed. Kept apart from Total Outstanding on
                        purpose - it is not receivable until an invoice exists for it. */}
                    <StatCard standalone label="Pending (not invoiced)" value={`₹${RoundOff(totals.pending || 0)}`} icon={Clock} />
                </div>
            )}

            <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 12 }}>
                {hiddenCount > 0 && (
                    <button type="button" className="shell-btn shell-btn-sm shell-btn-secondary" onClick={() => setShowSettled((v) => !v)}>
                        {showSettled ? "Hide" : "Show"} {hiddenCount} settled {hiddenCount === 1 ? "client" : "clients"}
                    </button>
                )}
                {/* Exports what is on screen - the settled-client filter above applies to
                    both, so the file matches the table rather than quietly holding more. */}
                <ReportDownloads
                    title="Customer Dues"
                    summary={
                        totals
                            ? [
                                  { label: "Total Billed", value: `₹${RoundOff(totals.billed)}` },
                                  { label: "Total Received", value: `₹${RoundOff(totals.received)}` },
                                  { label: "Total Outstanding", value: `₹${RoundOff(totals.due)}` },
                                  { label: "Pending (not invoiced)", value: `₹${RoundOff(totals.pending || 0)}` },
                              ]
                            : []
                    }
                    columns={[
                        { label: "Client", width: "24%" },
                        { label: "Firm", width: "20%" },
                        { label: "Phone", width: "14%" },
                        { label: "Billed", width: "14%", align: "right" },
                        { label: "Received", width: "14%", align: "right" },
                        { label: "Due", width: "14%", align: "right" },
                    ]}
                    rows={visibleRows.map((r) => [
                        r.clientName,
                        r.clientFirm || "",
                        r.clientPhone || "",
                        RoundOff(r.billed),
                        RoundOff(r.received),
                        RoundOff(r.due),
                    ])}
                    foot={totals ? ["Total", "", "", RoundOff(totals.billed), RoundOff(totals.received), RoundOff(totals.due)] : null}
                />
            </div>

            <DataTable loading={loading}>
                <thead>
                    <tr>
                        <th scope="col">Client</th>
                        <th scope="col">Phone</th>
                        <th scope="col">Billed</th>
                        <th scope="col">Received</th>
                        <th scope="col">Due</th>
                        <th scope="col" title="Job rows completed but not yet on any invoice">
                            Pending
                        </th>
                        <th scope="col">Remind</th>
                        <th scope="col" title="When a payment reminder last went to this customer">
                            Last remind
                        </th>
                    </tr>
                </thead>
                <tbody>
                    {loading ? (
                        <tr>
                            <td colSpan={8} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                Loading…
                            </td>
                        </tr>
                    ) : visibleRows.length === 0 ? (
                        <tr>
                            <td colSpan={8} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                {rows.length === 0 ? "No invoices raised yet." : "Every client is settled up."}
                            </td>
                        </tr>
                    ) : (
                        pageRows.map((row) => (
                            <tr key={row.clientId}>
                                <td>
                                    <div>{row.clientFirm || row.clientName}</div>
                                    {row.clientFirm && row.clientName && (
                                        <div className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                            {row.clientName}
                                        </div>
                                    )}
                                </td>
                                <td className="cell-mono">{row.clientPhone || <Blank />}</td>
                                <td className="cell-mono">₹{RoundOff(row.billed)}</td>
                                <td className="cell-mono">₹{RoundOff(row.received)}</td>
                                <td
                                    className="cell-mono"
                                    style={{ color: row.due > 0 ? "var(--xan-rose)" : row.due < 0 ? "var(--xan-blue)" : "var(--text-tertiary)" }}
                                >
                                    {formatDue(row.due)}
                                </td>
                                <td
                                    className="cell-mono"
                                    title={row.pendingJobIds && row.pendingJobIds.length ? `Job ${row.pendingJobIds.join(", ")}` : undefined}
                                    style={{ color: row.pending > 0 ? "var(--xan-amber)" : "var(--text-tertiary)" }}
                                >
                                    {row.pending > 0 ? `₹${RoundOff(row.pending)}` : "—"}
                                </td>
                                <td>
                                    {row.due > 0 &&
                                        (sentId === row.clientId ? (
                                            <span className="whatsapp-send-btn is-sent">
                                                <Check size={13} />
                                                Sent
                                            </span>
                                        ) : (
                                            <button
                                                type="button"
                                                className="whatsapp-send-btn"
                                                title={row.clientPhone ? "Send a payment reminder on WhatsApp" : "No phone number on file for this client"}
                                                disabled={!canRemind(row) || sendingId === row.clientId}
                                                onClick={() => onSendReminder(row)}
                                            >
                                                {sendingId === row.clientId ? (
                                                    <Loader size={13} className="whatsapp-send-btn-spin" />
                                                ) : (
                                                    <WhatsAppIcon size={13} />
                                                )}
                                                {sendingId === row.clientId ? "Sending" : "Remind"}
                                            </button>
                                        ))}
                                </td>
                                <td
                                    className="text-body-small"
                                    style={{ color: remindedToday(row) ? "var(--xan-amber)" : "var(--text-tertiary)", whiteSpace: "nowrap" }}
                                    title={row.lastRemindedAt ? new Date(row.lastRemindedAt).toLocaleString() : "Never reminded"}
                                >
                                    {formatLastRemind(row)}
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
                                ₹{RoundOff(totals.received)}
                            </td>
                            <td className="cell-mono" style={{ fontWeight: 600, color: "var(--xan-rose)" }}>
                                ₹{RoundOff(totals.due)}
                            </td>
                            <td className="cell-mono" style={{ fontWeight: 600, color: "var(--xan-amber)" }}>
                                ₹{RoundOff(totals.pending || 0)}
                            </td>
                            <td />
                            <td />
                        </tr>
                    </tfoot>
                )}
            </DataTable>
            <div className="table-foot">
                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                    {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} customers` : `${total} customers`}
                </span>
                <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
            </div>
        </>
    );
};

export default DuesIndex;
