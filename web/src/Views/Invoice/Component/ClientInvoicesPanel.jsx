import React, { useCallback, useEffect, useState } from "react";
import { Menu } from "react-feather";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { invoiceBackend } from "../invoice_backend";
import DataTable from "../../../Common/DataTable/DataTable";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import ReceivedHistory from "./RecivedHistory";
import InvoiceJobGroup from "./InvoiceJobGroup";
import JobDetailModal from "../../Lifecycle/component/JobDetailModal";
import "../invoice.css";

// One customer's invoices: the totals, the invoice rows, each row's job-ids as a tree, and
// the payment history in a side panel.
//
// Extracted from PreInvoiceComponents so the same thing can be a full page (deep links from
// WhatsApp and the customer list still land on /invoice/:cid) and a modal opened from the
// invoice list, without the two drifting into different views of one record.
//
// It owns its own fetch: a modal that had to be handed pre-loaded invoices would make every
// caller responsible for the shape of them, which is how two callers end up passing subtly
// different things.
const ClientInvoicesPanel = ({ cid, onLoaded }) => {
    const [state, setState] = useState({
        invoices: [],
        total: 0,
        received: 0,
        clientName: "",
        clientFirm: "",
        history: [],
        loaded: false,
    });
    const [sidebarOpen, setSidebarOpen] = useState(false);
    // Which invoices have their job-id tree expanded, and which job-card is open. The tree is
    // the invoice's contents; the modal is the job itself.
    const [expanded, setExpanded] = useState({});
    const [activeJob, setActiveJob] = useState(null);

    const load = useCallback(async () => {
        if (!cid) return;
        try {
            const formData = new FormData();
            formData.set("uid", window.localStorage.getItem("uid"));
            formData.set("cid", cid);
            const res = await invoiceBackend.getClientInvoices(formData);
            const sorted = [...(res.data || [])].sort((a, b) => String(b.invoiceId).localeCompare(String(a.invoiceId)));
            setState({
                invoices: sorted,
                clientName: res.clientName,
                clientFirm: res.clientFirm,
                total: RoundOff(res.totalDue),
                received: RoundOff(res.totalReceived),
                history: [...(res.receivedHistory || [])],
                loaded: true,
            });
            // Lets a modal title itself with the customer's name without fetching them again.
            onLoaded?.({ clientName: res.clientName, clientFirm: res.clientFirm });
        } catch (error) {
            // The interceptor toasts the reason; `loaded` stays false and the empty state shows.
            setState((prev) => ({ ...prev, loaded: true }));
        }
        // onLoaded is a caller's inline arrow more often than not; depending on it would
        // refetch on every render of the parent.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [cid]);

    useEffect(() => {
        load();
    }, [load]);

    const { pageRows, page, setPage, perPage, total } = usePagedRows(state.invoices);

    const toggleInvoice = (invoiceId) => setExpanded((prev) => ({ ...prev, [invoiceId]: !prev[invoiceId] }));

    if (!state.loaded) {
        return (
            <p className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                Loading…
            </p>
        );
    }

    return (
        <>
            <div className="client-invoices-head">
                <button
                    className="shell-icon-btn"
                    type="button"
                    aria-label="Open payment history"
                    onClick={() => setSidebarOpen(true)}
                    style={{ color: "var(--text-primary)" }}
                >
                    <Menu size={20} />
                </button>
                <span className="text-heading-brand client-invoices-totals">
                    Total due ₹{state.total} · Received ₹{state.received}
                </span>
            </div>

            {/* Two columns, not a portal. The payments panel used to slide over the whole
                viewport from document.body, which escaped the modal it belonged to. Beside
                the invoices it can be read against them, which is the point of having it. */}
            <div className="client-invoices-body">
            <div className="client-invoices-main">

            {state.invoices.length === 0 ? (
                <p className="text-body-small" style={{ color: "var(--text-tertiary)", padding: "0 4px 12px" }}>
                    No invoices for this customer yet.
                </p>
            ) : (
                <>
                    <DataTable>
                        <thead>
                            <tr>
                                <th scope="col">Date</th>
                                <th scope="col">Invoice No.</th>
                                <th scope="col">Taxed</th>
                                <th scope="col">Non-taxed</th>
                                <th scope="col">Received</th>
                                <th scope="col">Due (₹)</th>
                            </tr>
                        </thead>
                        <tbody>
                            {pageRows.map((el, index) => {
                                const jobs = el.job_ids || [];
                                const isOpen = Boolean(expanded[el._id]);
                                const due = el.taxedValue - el.entryReceived;
                                return (
                                    <React.Fragment key={el._id || index}>
                                        <tr
                                            onClick={() => jobs.length > 0 && toggleInvoice(el._id)}
                                            style={{ cursor: jobs.length > 0 ? "pointer" : undefined }}
                                        >
                                            <td className="cell-mono">
                                                {jobs.length > 0 && (
                                                    <span style={{ color: "var(--text-tertiary)", marginRight: 6 }}>
                                                        {isOpen ? "▾" : "▸"}
                                                    </span>
                                                )}
                                                {getDate(el.date)}
                                            </td>
                                            <td className="cell-mono">{el.invoiceId}</td>
                                            <td className="cell-mono">₹{RoundOff(el.taxedValue)}</td>
                                            <td className="cell-mono">₹{RoundOff(el.nonTaxedValue)}</td>
                                            <td className="cell-mono">₹{RoundOff(el.entryReceived)}</td>
                                            <td className="cell-mono">
                                                ₹{RoundOff(due)}{" "}
                                                {due !== 0 ? (
                                                    <StatusBadge status="pending">pending</StatusBadge>
                                                ) : (
                                                    <StatusBadge status="paid">paid</StatusBadge>
                                                )}
                                            </td>
                                        </tr>
                                        {isOpen && <InvoiceJobGroup jobs={jobs} columnCount={6} onOpenJob={setActiveJob} />}
                                    </React.Fragment>
                                );
                            })}
                        </tbody>
                    </DataTable>
                    <div className="table-foot">
                        <span className="text-body-small table-foot-count">
                            {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} invoices` : `${total} invoices`}
                        </span>
                        <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                    </div>
                </>
            )}

            </div>

            {sidebarOpen && (
                <ReceivedHistory
                    historyArr={state.history}
                    cname={state.clientName}
                    onClose={() => setSidebarOpen(false)}
                    onDeleteBtnHandler={() => {}}
                />
            )}
            </div>

            {activeJob && (
                <JobDetailModal
                    job={activeJob}
                    onClose={() => setActiveJob(null)}
                    onChange={(updated) => {
                        setState((prev) => ({
                            ...prev,
                            invoices: prev.invoices.map((inv) => ({
                                ...inv,
                                job_ids: (inv.job_ids || []).map((j) => (j._id === updated._id ? updated : j)),
                            })),
                        }));
                        setActiveJob(updated);
                    }}
                />
            )}
        </>
    );
};

export default ClientInvoicesPanel;
