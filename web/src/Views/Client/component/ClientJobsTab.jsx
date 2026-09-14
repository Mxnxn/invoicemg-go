import React, { useEffect, useMemo, useState } from "react";
import { Row, Col, Button } from "reactstrap";
import { Plus, FileText, Download, Calendar, X, ArrowUp, ArrowDown } from "react-feather";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { dimensionFactor } from "../../../Common/rowPricing";
import RowCheckbox from "../../../Common/DataTable/RowCheckbox";
import BulkActionsBar from "../../../Common/DataTable/BulkActionsBar";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import JobDetailModal from "../../Lifecycle/component/JobDetailModal";
import CreateJobModal from "../../Lifecycle/component/CreateJobModal";
import { lifecycleBackend } from "../../Lifecycle/lifecycle_backend";
import Pagination from "../../../Shell/Pagination";
import {
    pendingRowsCount,
    isReadyForInvoice,
    isFullyConverted,
    isFullyInvoiced,
    isJobPaid,
} from "../clientJobsMath";
import { notifyError } from "../../../global/toast";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import SearchField from "../../../Common/SearchField";
import JobCard from "../../../Common/cards/JobCard";
import { groupJobsByDate } from "../clientJobGroups";
import { dayParts, relativeDayLabel } from "../../dashboardCards";
import "../client.css";
import DateField from "../../../Common/DateField";


// A row's own net amount before tax - qty * <area> * rate, net of discount/charges, where
// <area> is length*width for a by-dimension row and 1 for a by-quantity one. Same formula the
// backend uses for job/entry totals (see Helpers/JobTotals.js's rowGrossTotal).
const rowNetAmount = (row) => {
    const qty = Number(row.qty) || 0;
    const rate = Number(row.rate) || 0;
    // dimensionFactor is 1 on a by-quantity row, so this stays qty * rate for those.
    const amount = qty * dimensionFactor(row) * rate;
    return amount - (Number(row.discount) || 0) + (Number(row.charges) || 0);
};

const rowTaxPct = (row) => (Number(row.cgst) || 0) + (Number(row.sgst) || 0) + (Number(row.igst) || 0);
const rowTaxAmount = (row) => rowNetAmount(row) * (rowTaxPct(row) / 100);

const jobTaxTotal = (job) => (job.rows || []).reduce((sum, row) => sum + rowTaxAmount(row), 0);
const jobDiscountTotal = (job) => (job.rows || []).reduce((sum, row) => sum + (Number(row.discount) || 0), 0);
const jobChargesTotal = (job) => (job.rows || []).reduce((sum, row) => sum + (Number(row.charges) || 0), 0);
// Only worth showing a "%" alongside the tax amount when every row actually shares one -
// a job mixing tax rates across rows just shows the amount on its own.
const jobTaxPct = (job) => {
    const rows = job.rows || [];
    if (rows.length === 0) return null;
    const first = rowTaxPct(rows[0]);
    return rows.every((row) => rowTaxPct(row) === first) ? first : null;
};

// "Finished" rather than "Ready For Invoice": the work being done is the fact the row is
// reporting, and whether it is ready to bill is what the Invoiced tab beside it answers.
const STATUS_TABS = [
    { key: "all", label: "All" },
    { key: "ready", label: "Finished" },
    { key: "pending", label: "Pending" },
    { key: "invoiced", label: "Invoiced" },
];


// Client-scoped jobs list, styled to match the Entries table (shell-card, header toolbar,
// DataTable) - the full sortable/editable table with column filters and row actions lives
// on the main Lifecycle page. Clicking a row here just opens the same Queue Detail modal
// (notes/queue/duration) Lifecycle uses.
const ClientJobsTab = ({
    clientId,
    clientName,
    entries,
    onJobCreated,
    onEntriesCreated,
    onDownloadEntries,
    onInvoiceJobs,
}) => {
    const [jobs, setJobs] = useState([]);
    const [loading, setLoading] = useState(true);
    const [activeJob, setActiveJob] = useState(null);
    const [createModal, setCreateModal] = useState(false);
    const [selected, setSelected] = useState([]);
    const [confirmConvert, setConfirmConvert] = useState(false);
    const [oldestFirst, setOldestFirst] = useState(false);
    const [dateRange, setDateRange] = useState({ from: "", to: "" });

    // Converting Done jobs into billable Entries changes client billing data - restrict it
    // to admins the same way the backend route does (requireAdmin on /jobs/convert-to-entries).
    const isAdmin = (window.localStorage.getItem("role") || "admin") !== "employee";

    useEffect(() => {
        if (!clientId) return;
        setLoading(true);
        lifecycleBackend
            .listJobs({ client_id: clientId })
            .then((res) => setJobs(res.data))
            .finally(() => setLoading(false));
    }, [clientId]);

    const onCreateJob = (formData) => {
        return lifecycleBackend.createJob(formData).then((res) => {
            setJobs((prev) => [res.data, ...prev]);
            // Pass the job up so the stat cards can re-bucket it, not just bump a count.
            onJobCreated?.(res.data);
        });
    };

    // Completion tracks per-row (row.queue), not on the job itself - see routes/Lifecycle.js's
    // /jobs/convert-to-entries for the matching backend-side fix.
    const isConvertible = (job) => (job.rows || []).some((row) => row.queue === "Done" && !row.entry_id);
    const hasConvertedEntries = (job) => (job.rows || []).some((row) => row.entry_id);
    // A job is selectable if there's something to do with the selection - either it can be
    // converted to entries, or it already has entries a Download/Invoice action can use.
    const isSelectable = (job) => isConvertible(job) || hasConvertedEntries(job);
    const selectableJobs = jobs.filter(isSelectable);

    const toggleSelected = (jobId) => {
        setSelected((prev) => (prev.includes(jobId) ? prev.filter((id) => id !== jobId) : [...prev, jobId]));
    };

    const toggleSelectAll = () => {
        setSelected((prev) => (prev.length === selectableJobs.length ? [] : selectableJobs.map((j) => j._id)));
    };

    // Entries billed under the selected jobs' converted rows - Download/Invoice operate on
    // these, matched against the client's full entry list (a job row only carries the
    // entry_id reference, not the entry document itself).
    const selectedEntries = useMemo(() => {
        const entryIds = new Set();
        jobs
            .filter((j) => selected.includes(j._id))
            .forEach((job) =>
                (job.rows || []).forEach((row) => row.entry_id && entryIds.add(String(row.entry_id._id || row.entry_id)))
            );
        return (entries || []).filter((entry) => entryIds.has(String(entry._id)));
    }, [jobs, selected, entries]);

    const onConvertToEntries = async () => {
        try {
            const formData = new FormData();
            formData.set("job_ids", JSON.stringify(selected));
            const res = await lifecycleBackend.convertJobsToEntries(formData);
            const { entries: createdEntries, jobs: updatedJobs } = res.data;
            setJobs((prev) => prev.map((j) => updatedJobs.find((u) => u._id === j._id) || j));
            onEntriesCreated?.(createdEntries);
            setSelected([]);
        } catch (error) {
            notifyError(error.message || "Couldn't convert the selected jobs to entries.");
        } finally {
            setConfirmConvert(false);
        }
    };

    // The bulk bar only appears with a selection, so without this there'd be no way to export
    // the client's full history - only whichever jobs you'd ticked.
    const onDownloadAll = () => {
        if ((entries || []).length === 0) {
            notifyError("This client has no billed entries to download yet.");
            return;
        }
        onDownloadEntries?.(entries);
    };

    const onDownloadClick = () => {
        if (selectedEntries.length === 0) {
            notifyError("None of the selected jobs have billed entries yet - convert them first.");
            return;
        }
        onDownloadEntries?.(selectedEntries);
    };

    // Invoicing no longer needs converted entries: /invoice/save takes job_ids and converts
    // any Done row itself. The only requirement is that a job is finished on the shop floor.
    const onInvoiceClick = () => {
        const chosen = jobs.filter((j) => selected.includes(j._id));
        const ready = chosen.filter(isReadyForInvoice);
        if (ready.length === 0) {
            notifyError("Only jobs with every row Done can be invoiced.");
            return;
        }
        onInvoiceJobs?.(ready);
    };


    // receivedDate is stored as a plain "YYYY-MM-DD" string (see CreateJobModal's today()
    // default) so lexicographic comparison already sorts chronologically - no Date parsing
    // needed for the range filter.
    const dateFilteredJobs = useMemo(() => {
        if (!dateRange.from && !dateRange.to) return jobs;
        return jobs.filter((job) => {
            if (!job.receivedDate) return false;
            if (dateRange.from && job.receivedDate < dateRange.from) return false;
            if (dateRange.to && job.receivedDate > dateRange.to) return false;
            return true;
        });
    }, [jobs, dateRange]);

    // The columns this used to sort by are gone, and with them the reason to sort by them:
    // job-ids are grouped under their date now, so ordering by job number across those groups
    // says nothing, and ordering by date IS the grouping. What is left is which end to start
    // from, which is a real question on a customer with two years of work.
    const sortedJobs = dateFilteredJobs;

    const [statusFilter, setStatusFilter] = useState("all");
    const [search, setSearch] = useState("");
    const [page, setPage] = useState(1);

    // Status lives as tabs in this card's own header rather than as stat cards above it -
    // six cards stacked over the table pushed the actual jobs below the fold. Counts come
    // from the same predicates that render the row badges, so a tab's number always matches
    // the rows it shows.
    const searchedJobs = useMemo(() => {
        if (!search) return sortedJobs;
        const needle = search.toUpperCase();
        return sortedJobs.filter(
            (j) =>
                (j.challanNumber || "").toUpperCase().includes(needle) ||
                (j.rows || []).some(
                    (r) =>
                        (r.material || "").toUpperCase().includes(needle) ||
                        (r.description || "").toUpperCase().includes(needle) ||
                        (r.rowId || "").toUpperCase().includes(needle)
                )
        );
    }, [sortedJobs, search]);

    const statusFilteredJobs = useMemo(() => {
        if (statusFilter === "all") return searchedJobs;
        if (statusFilter === "invoiced") return searchedJobs.filter(isFullyInvoiced);
        if (statusFilter === "ready") return searchedJobs.filter((job) => !isFullyInvoiced(job) && isReadyForInvoice(job));
        if (statusFilter === "pending") return searchedJobs.filter((job) => !isFullyInvoiced(job) && !isReadyForInvoice(job));
        return searchedJobs;
    }, [searchedJobs, statusFilter]);

    const PER_PAGE = 25;
    const pageCount = Math.ceil(statusFilteredJobs.length / PER_PAGE);

    // Any change to what's being listed can leave you stranded past the last page.
    useEffect(() => {
        setPage(1);
    }, [statusFilter, search, dateRange.from, dateRange.to, oldestFirst]);

    const visibleJobs = useMemo(
        () => (statusFilteredJobs.length > PER_PAGE ? statusFilteredJobs.slice((page - 1) * PER_PAGE, page * PER_PAGE) : statusFilteredJobs),
        [statusFilteredJobs, page]
    );

    // Grouped for display. Everything above this line - the date range, the search, the status
    // tabs, the paging - still decides WHICH job-ids; this only decides how they are laid out.
    const dateGroups = useMemo(() => groupJobsByDate(visibleJobs, { oldestFirst }), [visibleJobs, oldestFirst]);

    const statusCounts = useMemo(() => {
        const counts = { all: searchedJobs.length, ready: 0, pending: 0, invoiced: 0 };
        for (const job of searchedJobs) {
            if (isFullyInvoiced(job)) counts.invoiced += 1;
            else if (isReadyForInvoice(job)) counts.ready += 1;
            else counts.pending += 1;
        }
        return counts;
    }, [searchedJobs]);

    const colSpan = isAdmin ? 12 : 11;

    return (
        <Row className="mt-3">
            <Col>
                <div className="shell-card">
                    <div className="shell-card-header" style={{ flexWrap: "wrap", gap: 12 }}>
                        <div className="d-flex align-items-center" style={{ gap: 12, flexWrap: "wrap" }}>
                            <SearchField style={{ minWidth: 260 }} placeholder="Search job-id, entries, or product..." value={search} onChange={(e) => setSearch(e.target.value)} />
                            <div className="shell-segmented shell-segmented--fill" role="tablist">
                                {STATUS_TABS.map((tab) => {
                                    const active = statusFilter === tab.key;
                                    return (
                                        <button
                                            key={tab.key}
                                            type="button"
                                            role="tab"
                                            aria-selected={active}
                                            className="shell-segmented-btn"
                                            style={active ? { background: "var(--xan-blue-bg)", color: "var(--xan-blue)" } : undefined}
                                            onClick={() => setStatusFilter(tab.key)}
                                        >
                                            {tab.label}
                                            <span
                                                className="text-body-small"
                                                style={{
                                                    marginLeft: 6,
                                                    opacity: 0.75,
                                                    fontVariantNumeric: "tabular-nums",
                                                }}
                                            >
                                                {statusCounts[tab.key]}
                                            </span>
                                        </button>
                                    );
                                })}
                            </div>
                        </div>
                        <div className="d-flex align-items-center" style={{ gap: 8, flexWrap: "wrap" }}>
                            {/* The date range is a filter, the two beside it are actions - a flat
                                gap put them all the same distance apart and read as one row of
                                five controls. The extra margin here groups the filter away from
                                the button pair, which stays tight at 8px. */}
                            <div className="d-flex align-items-center" style={{ gap: 6, marginRight: 12 }}>
                                <Calendar size={14} style={{ color: "var(--text-tertiary)" }} />
                                <DateField value={dateRange.from} onChange={(e) => setDateRange((prev) => ({ ...prev, from: e.target.value }))} style={{ width: 150 }} />
                                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    to
                                </span>
                                <DateField value={dateRange.to} onChange={(e) => setDateRange((prev) => ({ ...prev, to: e.target.value }))} style={{ width: 150 }} />
                                {(dateRange.from || dateRange.to) && (
                                    <button
                                        type="button"
                                        className="shell-icon-btn"
                                        aria-label="Clear date range"
                                        onClick={() => setDateRange({ from: "", to: "" })}
                                    >
                                        <X size={14} />
                                    </button>
                                )}
                            </div>
                            {/* Which end of the customer's history to start from. The only
                                ordering question a date-grouped deck can still be asked. */}
                            <button
                                type="button"
                                className="shell-btn shell-btn-secondary d-flex align-items-center"
                                style={{ gap: 6 }}
                                aria-pressed={oldestFirst}
                                title={oldestFirst ? "Showing oldest first" : "Showing newest first"}
                                onClick={() => setOldestFirst((v) => !v)}
                            >
                                {oldestFirst ? <ArrowUp size={15} /> : <ArrowDown size={15} />}
                                {oldestFirst ? "Oldest" : "Newest"}
                            </button>
                            <Button
                                className="shell-btn shell-btn-secondary d-flex align-items-center"
                                style={{ gap: 6 }}
                                onClick={onDownloadAll}
                            >
                                <Download size={15} />
                                Download
                            </Button>
                            <Button
                                className="shell-btn shell-btn-primary d-flex align-items-center"
                                style={{ gap: 6 }}
                                onClick={() => setCreateModal(true)}
                            >
                                <Plus size={15} />
                                Create Job
                            </Button>
                        </div>
                    </div>
                    {/* The job-ids as cards, gathered under the day they came in - the same
                        card the sheet draws, and the inversion of it: that page is one day
                        across many customers, this is one customer across many days.

                        It replaces an eleven-column table. Eleven columns of a job-id is a
                        horizontal scroll holding four figures anybody actually reads, and it
                        put the products - the thing that says what the work IS - behind a
                        column that only ever showed the first one. */}
                    <div className="client-deck">
                        {loading ? (
                            <p className="client-deck-note text-body-small">Loading...</p>
                        ) : dateGroups.length === 0 ? (
                            <p className="client-deck-note text-body-small">
                                {jobs.length > 0 ? "No job-ids match these filters." : "No job-ids for this customer yet."}
                            </p>
                        ) : (
                            dateGroups.map((group) => {
                                const parts = dayParts(group.date);
                                const relative = relativeDayLabel(group.date);
                                return (
                                    <section className="client-day" key={group.date || "undated"}>
                                        <header className="client-day-head">
                                            <span className="client-day-when">
                                                {group.date ? (
                                                    <>
                                                        <span className="client-day-num">{parts.day}</span>
                                                        <span className="client-day-rest">
                                                            <span className="client-day-mon">
                                                                {parts.month} {parts.year}
                                                            </span>
                                                            <span className="client-day-weekday">
                                                                {parts.weekday}
                                                                {relative && <span className="client-day-chip">{relative}</span>}
                                                            </span>
                                                        </span>
                                                    </>
                                                ) : (
                                                    <span className="client-day-mon">No date on these</span>
                                                )}
                                            </span>

                                            <span className="client-day-money">
                                                <span className="client-day-figure">
                                                    <span className="client-day-label">Job-ids</span>
                                                    <span className="client-day-value">{group.jobs.length}</span>
                                                </span>
                                                <span className="client-day-figure">
                                                    <span className="client-day-label">Total</span>
                                                    <span className="client-day-value">&#8377;{RoundOff(group.total)}</span>
                                                </span>
                                                <span className={`client-day-figure${group.due > 0 ? " is-due" : ""}`}>
                                                    <span className="client-day-label">Due</span>
                                                    <span className="client-day-value">&#8377;{RoundOff(group.due)}</span>
                                                </span>
                                            </span>
                                        </header>

                                        <div className="job-card-deck">
                                            {group.jobs.map((job) => {
                                                const readyForInvoice = isReadyForInvoice(job);
                                                const invoiced = isFullyInvoiced(job);
                                                const converted = isFullyConverted(job);
                                                return (
                                                    <JobCard
                                                        key={job._id}
                                                        job={job}
                                                        onOpen={setActiveJob}
                                                        selectable={isAdmin && isSelectable(job)}
                                                        selected={selected.includes(job._id)}
                                                        onToggleSelect={(j) => toggleSelected(j._id)}
                                                        badge={
                                                            !readyForInvoice ? (
                                                                <StatusBadge status="pending">
                                                                    {pendingRowsCount(job)} pending
                                                                </StatusBadge>
                                                            ) : invoiced ? (
                                                                <StatusBadge status="paid">Invoiced</StatusBadge>
                                                            ) : converted ? (
                                                                <StatusBadge status="open">Awaiting payment</StatusBadge>
                                                            ) : (
                                                                <StatusBadge status="pending">Finished</StatusBadge>
                                                            )
                                                        }
                                                    />
                                                );
                                            })}
                                        </div>
                                    </section>
                                );
                            })
                        )}
                    </div>
                    <div className="shell-card-footer" style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            {statusFilteredJobs.length} job{statusFilteredJobs.length === 1 ? "" : "s"}
                            {pageCount > 1 ? ` - page ${page} of ${pageCount}` : ""}
                        </span>
                        {statusFilteredJobs.length > PER_PAGE && (
                            <Pagination totalItems={statusFilteredJobs.length} perPage={PER_PAGE} currentPage={page} setCurrentPage={setPage} />
                        )}
                    </div>
                </div>
            </Col>
            {isAdmin && (
                <BulkActionsBar
                    count={selected.length}
                    itemLabel="Job"
                    actions={[
                        { label: "Download", icon: Download, onClick: onDownloadClick },
                        { label: "Invoice", icon: FileText, onClick: onInvoiceClick },
                    ]}
                />
            )}
            <ConfirmDialog
                open={confirmConvert}
                message={`Convert ${selected.length} job${selected.length === 1 ? "" : "s"} to entries? This can't be undone.`}
                confirmLabel="Convert"
                position="bottom"
                onConfirm={onConvertToEntries}
                onCancel={() => setConfirmConvert(false)}
            />
            {activeJob && (
                <JobDetailModal
                    job={activeJob}
                    onClose={() => setActiveJob(null)}
                    onChange={(updated) => {
                        setJobs((prev) => prev.map((j) => (j._id === updated._id ? updated : j)));
                        setActiveJob(updated);
                    }}
                />
            )}
            <CreateJobModal
                isOpen={createModal}
                toggle={() => setCreateModal(false)}
                onCreate={onCreateJob}
                editingJob={null}
                fixedClient={{ id: clientId, name: clientName }}
            />
        </Row>
    );
};

export default ClientJobsTab;
