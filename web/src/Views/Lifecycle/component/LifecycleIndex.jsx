import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Filter, Plus, ChevronUp, ChevronDown, Edit2, Trash2, Search, Move, FileText, Clock, X } from "react-feather";
import LiteHeader from "../../../Common/Header/LiteHeader";
import DataTable from "../../../Common/DataTable/DataTable";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import RowCheckbox from "../../../Common/DataTable/RowCheckbox";
import useRowSelection from "../../../Common/DataTable/useRowSelection";
import BulkActionsBar from "../../../Common/DataTable/BulkActionsBar";
import CreateInvoiceFromJobsModal from "../../Invoice/Component/CreateInvoiceFromJobsModal";
import CreateJobModal from "./CreateJobModal";
import JobCreatedNotifyPrompt from "./JobCreatedNotifyPrompt";
import { whatsappBackend } from "../../../Common/whatsapp_backend";
import JobDetailModal from "./JobDetailModal";
import JobBoardGrid from "./JobBoardGrid";
import ViewToggle, { readView } from "../../../Common/cards/ViewToggle";
import JobBoardOverlay from "./JobBoardOverlay";
import ConfirmDialog from "../../../Common/ConfirmDialog";
import { lifecycleBackend } from "../lifecycle_backend";
import StatCard from "../../../Shell/StatCard";
import { useUndoDelete } from "../../../Common/undoDelete";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { can } from "../../../Common/access";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { useJobCreated } from "../../../Common/jobStore";
import SearchField from "../../../Common/SearchField";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import { readTableSetting, writeTableSetting } from "../../../Common/tableSettings";
import { portalHost } from "../../../Common/portalHost";
import { useSearchParams } from "react-router-dom";
import { jobHasOpenCards } from "./queueConstants";

const STORAGE_KEY = "lifecycle_visible_columns";
const ORDER_STORAGE_KEY = "lifecycle_column_order";

// A job is only "Ready For Invoice" once every one of its rows has individually reached
// Done - completion is tracked per-row (row.queue), not on the job itself, so this can't
// just check job.queue (that field is a vestigial pre-row-level-refactor leftover that
// never advances anymore - see routes/Lifecycle.js's /jobs/convert-to-entries for the
// matching backend-side note).
// invoiceState / readyForInvoice are computed on the API (Helpers/JobInvoiceState.js) and
// attached to every job payload, so the board, the client tab and a freshly-edited job all
// agree. The local fallback keeps this working against an older response shape.
//
// It is derived from the rows' entries rather than stored on the Job, which is why deleting
// an invoice needs nothing here: /invoice/remove clears has_issued and the next fetch simply
// reports a different state.
const jobInvoiceState = (job) => job.invoiceState || "none";
const jobIsInvoiced = (job) => jobInvoiceState(job) === "invoiced";
const jobPartlyInvoiced = (job) => jobInvoiceState(job) === "partial";
const jobReadyForInvoice = (job) =>
    typeof job.readyForInvoice === "boolean"
        ? job.readyForInvoice
        : (job.rows || []).length > 0 &&
          (job.rows || []).every((row) => row.queue === "Done") &&
          jobInvoiceState(job) === "none";
const jobIsQuoted = (job) => (job.rows || []).some((row) => row.quotation_id);

// "All" leads because the others are each a slice - In-Progress deliberately excludes both
// Ready For Invoice and Invoiced, so without it there was no view showing every job-id.
const BOARD_CATEGORIES = ["All", "In-Progress", "Quoted", "Ready For Invoice", "Invoiced"];
const jobsInCategory = (jobs, category) => {
    if (category === "All") return jobs;
    if (category === "Ready For Invoice") return jobs.filter(jobReadyForInvoice);
    if (category === "Invoiced") return jobs.filter((j) => jobIsInvoiced(j) || jobPartlyInvoiced(j));
    if (category === "Quoted") return jobs.filter(jobIsQuoted);
    return jobs.filter((j) => !jobReadyForInvoice(j) && !jobIsInvoiced(j));
};

// Shown in the Invoiced column and used by the board chips.
//
// A fully invoiced job names its invoice rather than saying "Invoiced" - the word only
// repeats the column heading, where the number is the thing you would otherwise have to open
// the job to find. invoiceNumbers comes from the API (routes/Lifecycle.js withInvoiceState);
// it falls back to the word when the list is empty, which is what an older payload sends.
//
// A job billed across two invoices lists both. That is rare and worth seeing - collapsing it
// to the first would quietly misreport which invoice covers the work.
const invoiceBadge = (job) => {
    const state = jobInvoiceState(job);
    const numbers = job.invoiceNumbers || [];
    if (state === "invoiced") return { label: numbers.join(", ") || "Invoiced", status: "green" };
    if (state === "partial") {
        const count = `${job.invoicedRows || 0}/${(job.rows || []).length}`;
        return { label: numbers.length ? `${numbers.join(", ")} · ${count}` : `${count} invoiced`, status: "amber" };
    }
    if (jobReadyForInvoice(job)) return { label: "Ready", status: "pending" };
    return { label: "—", status: "neutral" };
};

const COLUMNS = [
    { key: "receivedDate", label: "Received Date", mono: true, sortable: true },
    { key: "challanNumber", label: "Job Number", mono: true, sortable: true },
    { key: "quotationNumber", label: "Quotation", mono: true },
    { key: "clientFirm", label: "Client Firm", sortable: true },
    { key: "clientPhone", label: "Phone Number", mono: true, sortable: true },
    { key: "queueSummary", label: "Queue", mono: true },
    { key: "rows", label: "Rows", mono: true, sortable: true },
    { key: "total", label: "Total", mono: true },
    { key: "invoiceState", label: "Invoiced" },
];

// Rows and Total start hidden - the table is dense enough without them by default, and the
// column filter lets anyone turn them back on.
const DEFAULT_HIDDEN_COLUMNS = ["rows", "total"];

const loadVisibleColumns = () => {
    try {
        const saved = JSON.parse(window.localStorage.getItem(STORAGE_KEY));
        if (Array.isArray(saved) && saved.length) return saved;
    } catch (error) {
        console.log(error);
    }
    return COLUMNS.map((c) => c.key).filter((key) => !DEFAULT_HIDDEN_COLUMNS.includes(key));
};

// Column order is separate from visibility - a hidden column keeps its place in this list
// so re-showing it later doesn't reset the arrangement.
const loadColumnOrder = () => {
    try {
        const saved = JSON.parse(window.localStorage.getItem(ORDER_STORAGE_KEY));
        if (Array.isArray(saved) && saved.length) {
            // Any column added to COLUMNS after this order was saved (or dropped from it)
            // still needs to show up/be ignored - reconcile rather than trust it blindly.
            const known = new Set(COLUMNS.map((c) => c.key));
            const reconciled = saved.filter((key) => known.has(key));
            COLUMNS.forEach((c) => {
                if (!reconciled.includes(c.key)) reconciled.push(c.key);
            });
            return reconciled;
        }
    } catch (error) {
        console.log(error);
    }
    return COLUMNS.map((c) => c.key);
};

// Portaled out of the row (like AssigneeDropdown's menu) - it used to render inline inside
// .shell-card-header, which .shell-card's overflow:hidden was clipping away whenever the menu
// extended down past the header, so it looked like it opened "behind" the table.
// Queue stage -> StatusBadge variant. Progression reads cool-to-warm-to-done: Created is
// neutral, Printing is in-flight (blue), Ready-to-Pickup is waiting on someone (amber), Done
// is settled (emerald). Anything custom (see CUSTOM_QUEUE_OPTIONS) falls back to neutral.
const QUEUE_VARIANT = {
    Created: "neutral",
    Printing: "open",
    "Ready-to-Pickup": "pending",
    Done: "paid",
};

const ColumnFilterDropdown = ({ visible, onToggleColumn }) => {
    const [open, setOpen] = useState(false);
    const [position, setPosition] = useState(null);
    const triggerRef = useRef(null);
    const menuRef = useRef(null);

    useEffect(() => {
        if (!open) {
            setPosition(null);
            return;
        }
        const rect = triggerRef.current.getBoundingClientRect();
        setPosition({ top: rect.bottom + 6, right: window.innerWidth - rect.right });
    }, [open]);

    useEffect(() => {
        if (!open) return;
        const onOutside = (e) => {
            if (menuRef.current?.contains(e.target) || triggerRef.current?.contains(e.target)) return;
            setOpen(false);
        };
        // The menu is overflowY:auto with a maxHeight, so it scrolls internally. This
        // listener is capture-phase and fires for *any* scroll in the document - including
        // that one - so the menu closed the instant you tried to scroll it. Ignore scrolls
        // that originate inside the menu; the point of closing on scroll is that the trigger
        // moves out from under a fixed-position menu, which an internal scroll doesn't do.
        const close = (e) => {
            if (e && e.target && menuRef.current && menuRef.current.contains(e.target)) return;
            setOpen(false);
        };
        document.addEventListener("mousedown", onOutside);
        window.addEventListener("scroll", close, true);
        window.addEventListener("resize", close);
        return () => {
            document.removeEventListener("mousedown", onOutside);
            window.removeEventListener("scroll", close, true);
            window.removeEventListener("resize", close);
        };
    }, [open]);

    return (
        <>
            <button
                ref={triggerRef}
                type="button"
                className="shell-btn shell-btn-sm shell-btn-secondary"
                onClick={() => setOpen((v) => !v)}
            >
                <Filter size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
                Columns
            </button>
            {open &&
                position &&
                createPortal(
                    <div
                        ref={menuRef}
                        className="xan-row-menu"
                        style={{
                            position: "fixed",
                            top: position.top,
                            right: position.right,
                            transform: "none",
                            minWidth: 210,
                            maxHeight: 260,
                            overflowY: "auto",
                        }}
                    >
                        {COLUMNS.map((col) => (
                            <label
                                key={col.key}
                                className="xan-row-menu-item"
                                style={{ cursor: "pointer", display: "flex", alignItems: "center", gap: 8 }}
                            >
                                <RowCheckbox
                                    checked={visible.includes(col.key)}
                                    onChange={() => onToggleColumn(col.key)}
                                    ariaLabel={`Toggle ${col.label} column`}
                                />
                                <span>{col.label}</span>
                            </label>
                        ))}
                    </div>,
                    portalHost()
                )}
        </>
    );
};

const SortIcon = ({ direction }) => {
    if (!direction) return null;
    return direction === "asc" ? (
        <ChevronUp size={12} style={{ marginLeft: 4, verticalAlign: "middle" }} />
    ) : (
        <ChevronDown size={12} style={{ marginLeft: 4, verticalAlign: "middle" }} />
    );
};

const LifecycleIndex = () => {
    const [jobs, setJobs] = useState([]);
    const [loading, setLoading] = useState(true);
    const [visibleColumns, setVisibleColumns] = useState(loadVisibleColumns);
    // Only Ready-For-Invoice jobs are selectable: an in-progress job has nothing billable,
    // and an invoiced one is already done. Offering a checkbox that leads to a refusal is
    // worse than not offering it.
    const [invoiceOpen, setInvoiceOpen] = useState(false);
    const [columnOrder, setColumnOrder] = useState(loadColumnOrder);
    const [dragColIndex, setDragColIndex] = useState(null);
    const [createOpen, setCreateOpen] = useState(false);
    // "edit" rides with the create action in this permission model (Helpers/Permissions.js) -
    // the create and edit forms are the same screen.
    const canEditJobs = can("lifecycle", "create");
    const canDeleteJobs = can("lifecycle", "delete");
    const [editingJob, setEditingJob] = useState(null);
    const [deleteTarget, setDeleteTarget] = useState(null);
    const { scheduleDelete } = useUndoDelete();
    const [activeJob, setActiveJob] = useState(null);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [sort, setSort] = useState({ key: null, direction: null });
    const [activeCategory, setActiveCategory] = useState("All");
    // "table" or "board". Table deliberately: the board is a second view beside this page, not
    // a replacement, so landing here gives what it has always given.
    const [view, setView] = useState(() => readView("lifecycle"));
    // The job whose board is open, with the rectangle of the card it was opened from - the
    // overlay expands out of that rectangle rather than fading in on the spot.
    const [boardJob, setBoardJob] = useState(null);
    // Arrived from the dashboard's "still open" alert. Held in the URL so the filtered list is
    // a place you can link to, go back to, and reload into - and cleared through the URL too,
    // so the chip and the address bar cannot disagree about what is being filtered.
    const [searchParams, setSearchParams] = useSearchParams();
    const openOnly = searchParams.get("open") === "1";
    const clearOpenOnly = () => {
        const next = new URLSearchParams(searchParams);
        next.delete("open");
        setSearchParams(next, { replace: true });
    };
    const [search, setSearch] = useState("");

    useEffect(() => {
        writeTableSetting(STORAGE_KEY, visibleColumns);
    }, [visibleColumns]);

    useEffect(() => {
        writeTableSetting(ORDER_STORAGE_KEY, columnOrder);
    }, [columnOrder]);

    // Named so creating an invoice can re-read the board: the jobs it billed move out of
    // Ready For Invoice, and a stale list would still offer them.
    const loadJobs = useCallback(() => {
        setLoading(true);
        lifecycleBackend
            .listJobs()
            .then((res) => setJobs(res.data))
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        loadJobs();
    }, [loadJobs]);

    const toggleColumn = (key) => {
        setVisibleColumns((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]));
    };

    // A job-id raised from the navbar lands here without a reload. Guarded against double
    // entry: creating one from this board already prepends it through onCreateJob.
    useJobCreated(
        useCallback((job) => {
            setJobs((prev) => (prev.some((j) => j._id === job._id) ? prev : [job, ...prev]));
        }, [])
    );

    // The job just raised, while the "tell the customer?" card is still on screen.
    const [justCreated, setJustCreated] = useState(null);
    // Company.notifyOnCreate, read once. It only decides whether that card counts itself down,
    // so a failed read simply means the card waits to be pressed - the safe answer.
    const [notifyOnCreate, setNotifyOnCreate] = useState(false);
    // Its sibling for the update message - a separate company-wide switch, because announcing
    // a new job and announcing an edit are different decisions.
    const [notifyOnUpdate, setNotifyOnUpdate] = useState(false);

    useEffect(() => {
        let live = true;
        whatsappBackend
            .getConfig()
            .then((res) => {
                if (!live) return;
                const configured = Boolean(res.data?.configured);
                setNotifyOnCreate(Boolean(res.data?.notifyOnCreate && configured));
                setNotifyOnUpdate(Boolean(res.data?.notifyOnUpdate && configured));
            })
            .catch(() => {});
        return () => {
            live = false;
        };
    }, []);

    const onCreateJob = (formData) => {
        return lifecycleBackend.createJob(formData).then((res) => {
            setJobs((prev) => [res.data, ...prev]);
            // Only worth offering when there is somewhere to send it. A client with no number
            // on file would get a card whose only outcome is an error.
            if (res.data?.client_id?.clientPhone) setJustCreated(res.data);
        });
    };

    const onUpdateJob = (formData) => {
        return lifecycleBackend.updateJob(formData).then((res) => {
            setJobs((prev) => prev.map((j) => (j._id === res.data._id ? res.data : j)));
            // An edit to an already-announced job is news the customer is owed: they are
            // holding a job sheet that no longer matches the work. The server decides what
            // counts as changed (Helpers/JobAlertState.rowFingerprint - material, description,
            // qty, dimensions, rate, tax and discount, but never a queue move), so a stage
            // change still passes through here silently.
            //
            // `changed` is false for a job that was never announced, which is what keeps a
            // declined create offer from being re-asked on every subsequent edit.
            const alert = res.data?.alerts?.created;
            if (alert?.changed && alert?.canSend && res.data?.client_id?.clientPhone) setJustCreated(res.data);
        });
    };

    const confirmDelete = () => {
        if (!deleteTarget) return;
        const job = deleteTarget;
        const index = jobs.findIndex((j) => j._id === job._id);
        setJobs((prev) => prev.filter((j) => j._id !== job._id));
        setDeleteTarget(null);
        scheduleDelete({
            label: `job ${job.challanNumber || ""}`.trim(),
            commit: () => lifecycleBackend.deleteJob({ job_id: job._id }),
            undo: () => setJobs((prev) => [...prev.slice(0, index), job, ...prev.slice(index)]),
        });
    };

    const onSortClick = (key) => {
        setSort((prev) => {
            if (prev.key !== key) return { key, direction: "asc" };
            if (prev.direction === "asc") return { key, direction: "desc" };
            if (prev.direction === "desc") return { key: null, direction: null };
            return { key, direction: "asc" };
        });
    };

    const cols = columnOrder.map((key) => COLUMNS.find((c) => c.key === key)).filter((c) => c && visibleColumns.includes(c.key));

    const onColDragStart = (index) => setDragColIndex(index);
    const onColDragOver = (e, index) => {
        e.preventDefault();
        if (dragColIndex === null || dragColIndex === index) return;
        const draggedKey = cols[dragColIndex].key;
        const overKey = cols[index].key;
        setColumnOrder((prev) => {
            const next = [...prev];
            const from = next.indexOf(draggedKey);
            const to = next.indexOf(overKey);
            next.splice(from, 1);
            next.splice(to, 0, draggedKey);
            return next;
        });
        setDragColIndex(index);
    };
    const onColDragEnd = () => setDragColIndex(null);

    // A category chosen before is an invisible second filter once ?open=1 arrives, and the two
    // can intersect to an empty list that looks like "there is nothing open".
    useEffect(() => {
        if (openOnly) setActiveCategory("All");
    }, [openOnly]);

    // Counted off every job, not off the filtered list - the chip says how many there are, and
    // once the filter is on the filtered list would just report itself.
    const openJobCount = useMemo(() => jobs.filter(jobHasOpenCards).length, [jobs]);

    const jobsInQueue = useMemo(() => {
        // ?open=1 narrows to job-ids with a card short of Done - what the dashboard's "still
        // open" alert links to, so arriving from it lands on those job-ids rather than on the
        // full list with nothing to say which ones were meant.
        const base = openOnly ? jobs.filter(jobHasOpenCards) : jobs;
        const inCategory = jobsInCategory(base, activeCategory);
        if (!search) return inCategory;
        const needle = search.toUpperCase();
        // Matching a job-entry (row) surfaces the whole job-id it's clubbed under, not just
        // the row itself - there's no separate row-level list view.
        return inCategory.filter(
            (j) =>
                (j.challanNumber || "").toUpperCase().includes(needle) ||
                (j.client_id?.clientFirm || "").toUpperCase().includes(needle) ||
                (j.client_id?.clientPhone || "").toUpperCase().includes(needle) ||
                (j.rows || []).some(
                    (r) =>
                        (r.material || "").toUpperCase().includes(needle) ||
                        (r.description || "").toUpperCase().includes(needle) ||
                        (r.rowId || "").toUpperCase().includes(needle)
                )
        );
    }, [jobs, activeCategory, search, openOnly]);

    const sortedJobs = useMemo(() => {
        if (!sort.key || !sort.direction) return jobsInQueue;
        const sorted = [...jobsInQueue].sort((a, b) => {
            let av = a[sort.key];
            let bv = b[sort.key];
            if (sort.key === "clientFirm") {
                av = a.client_id?.clientFirm || "";
                bv = b.client_id?.clientFirm || "";
            }
            if (sort.key === "clientPhone") {
                av = a.client_id?.clientPhone || "";
                bv = b.client_id?.clientPhone || "";
            }
            if (sort.key === "rows") {
                av = a.rows?.length || 0;
                bv = b.rows?.length || 0;
            }
            if (typeof av === "string") av = av.toLowerCase();
            if (typeof bv === "string") bv = bv.toLowerCase();
            if (av < bv) return sort.direction === "asc" ? -1 : 1;
            if (av > bv) return sort.direction === "asc" ? 1 : -1;
            return 0;
        });
        return sorted;
    }, [jobsInQueue, sort]);

    // Page size comes from Account settings > Appearance, so every table in the app agrees
    // about what "25" means rather than each keeping its own.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(sortedJobs);

    // Every job-id except the ones entirely billed.
    //
    // This used to read lock.canEditQueue, which the API derives from "any row invoiced at
    // all" - so a job with one row billed and four still in production vanished from the
    // board completely. Those four rows are the work; hiding them hid the only view that
    // shows where work stands.
    //
    // A fully invoiced job-id does leave: it has nothing left to arrange, and it would collect
    // on the board for ever.
    //
    // Part-invoiced job-ids therefore arrive on a board that cannot move them - the API
    // freezes stages for the whole job the moment one row is billed. That is the API's rule,
    // not something to route around here, so the board says so plainly instead: the card is
    // badged, the chips are not draggable, and the head explains why.
    const boardJobs = useMemo(() => {
        const shown = jobs.filter((j) => jobInvoiceState(j) !== "invoiced");
        // The board answers the same filter as the table - switching view must not silently
        // widen what you are looking at back to everything.
        return openOnly ? shown.filter(jobHasOpenCards) : shown;
    }, [jobs, openOnly]);

    // Selection follows whichever view is on screen. The table is category-filtered and the
    // board is not, so deriving this from sortedJobs alone meant a job-id selected on the board
    // could be absent from invoiceableJobs - the checkbox ticked, the count moved, and Invoice
    // then billed nothing for it.
    const selectableJobs = view === "board" ? boardJobs : sortedJobs;
    const invoiceableJobs = useMemo(() => selectableJobs.filter(jobReadyForInvoice), [selectableJobs]);
    const selection = useRowSelection(invoiceableJobs);
    const chosenJobs = invoiceableJobs.filter((j) => selection.isSelected(j._id));
    // Every invoice belongs to one client, so a mixed selection cannot become one invoice.
    const clientsInSelection = new Set(chosenJobs.map((j) => String(j.client_id?._id || j.client_id)));


    return (
        <>
            <LiteHeader bg="primary" />
            {/* No padding of its own. .shell-content already gives 24px on every side, so the
                page's own 20px 24px was doubling it - which is the extra band of space above
                the view switch. Bottom only, so the last row is not flush with the edge. */}
            <div style={{ padding: "0 0 20px" }}>
                {/* Which view, not which filter. The category strip below is a filter and
                    stays with the table; this chooses how the same jobs are shown. Table is
                    first and is the default: the board is a second view beside this page, not
                    a replacement, so landing here gives exactly what it has always given. */}
                {/* The shared control, so this pair carries the same icons as the Board tabs
                    on invoices, quotations and purchase invoices - and so there is one place
                    left where Table-or-Board is drawn. */}
                <div style={{ marginBottom: 12 }}>
                    <ViewToggle
                        view={view}
                        storageKey="lifecycle"
                        onChange={(next) => {
                            // A selection made here is invisible over there: the two views show
                            // different sets. Carrying it across would leave a bulk bar counting
                            // job-ids nothing on screen is showing.
                            selection.clear();
                            setView(next);
                        }}
                    />
                </div>

                {view === "table" && (
                    <>
                <div
                    className="stats-compact"
                    role="tablist"
                    style={{ marginBottom: 16, display: "flex", gap: 10, overflowX: "auto" }}
                >
                    {BOARD_CATEGORIES.map((category) => (
                        <div key={category} role="tab" aria-selected={activeCategory === category} style={{ flex: "1 0 160px" }}>
                            <StatCard
                                label={category}
                                value={jobsInCategory(jobs, category).length}
                                accent={activeCategory === category}
                                standalone
                                onClick={() => setActiveCategory(category)}
                            />
                        </div>
                    ))}
                </div>
                <div className="shell-card">
                    <div className="shell-card-header">
                        {/* Says what is being hidden and undoes it in one press. A list that is
                            quietly shorter than it should be, with nothing on screen to say
                            why, is the worst way to arrive from a link. */}
                        {openOnly && (
                            <button
                                type="button"
                                className="lifecycle-filter-chip"
                                onClick={clearOpenOnly}
                                title="Showing only job-ids with a card short of Done - press to show all"
                                aria-label={`Clear the still-open filter, showing ${openJobCount} job-ids`}
                            >
                                <Clock size={12} aria-hidden="true" />
                                Still open
                                <span className="lifecycle-filter-chip-count">{openJobCount}</span>
                                <X size={12} aria-hidden="true" />
                            </button>
                        )}
                        <div className="list-header-actions">
                            <div className="list-header-group">
                                <SearchField
                                    className="list-header-search is-wide"
                                    placeholder="Search job-id, entries, client firm, or phone..."
                                    value={search}
                                    onChange={(e) => setSearch(e.target.value)}
                                />
                                <ColumnFilterDropdown visible={visibleColumns} onToggleColumn={toggleColumn} />
                                {canEditJobs && (
                                    <button
                                        type="button"
                                        className="shell-btn shell-btn-primary invoice-create-btn"
                                        onClick={() => setCreateOpen(true)}
                                    >
                                        <Plus size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
                                        Create Job
                                    </button>
                                )}
                            </div>
                        </div>
                    </div>
                    <DataTable loading={loading}>
                        <thead>
                            <tr>
                                <th scope="col" className="xan-col-checkbox">
                                    <RowCheckbox
                                        checked={selection.allSelected}
                                        indeterminate={selection.someSelected && !selection.allSelected}
                                        onChange={selection.toggleAll}
                                        ariaLabel="Select all billable jobs"
                                    />
                                </th>
                                {cols.map((c, index) => (
                                    <th
                                        key={c.key}
                                        scope="col"
                                        draggable
                                        onDragStart={() => onColDragStart(index)}
                                        onDragOver={(e) => onColDragOver(e, index)}
                                        onDragEnd={onColDragEnd}
                                        onClick={c.sortable ? () => onSortClick(c.key) : undefined}
                                        style={{
                                            cursor: c.sortable ? "pointer" : "grab",
                                            userSelect: "none",
                                            opacity: dragColIndex === index ? 0.5 : 1,
                                        }}
                                    >
                                        <Move
                                            size={10}
                                            style={{ marginRight: 4, verticalAlign: "middle", color: "var(--xan-text-dim)", cursor: "grab" }}
                                        />
                                        {c.label}
                                        {c.sortable && (
                                            <Filter
                                                size={11}
                                                style={{
                                                    marginLeft: 5,
                                                    verticalAlign: "middle",
                                                    color: sort.key === c.key ? "var(--xan-blue)" : "var(--xan-text-dim)",
                                                }}
                                            />
                                        )}
                                        {c.sortable && <SortIcon direction={sort.key === c.key ? sort.direction : null} />}
                                    </th>
                                ))}
                                <th scope="col" style={{ width: 44 }} />
                            </tr>
                        </thead>
                        <tbody>
                            {loading ? (
                                <tr>
                                    <td colSpan={cols.length + 2} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 20 }}>
                                        Loading…
                                    </td>
                                </tr>
                            ) : (
                            pageRows.map((job, index) => {
                                return (
                                    <tr key={job._id} onClick={() => setActiveJob(job)} style={{ cursor: "pointer" }}>
                                        {/* Only billable rows get a box - the cell is always
                                            present so the columns stay aligned. stopPropagation
                                            because the row itself opens the job. */}
                                        <td className="xan-col-checkbox" onClick={(e) => e.stopPropagation()}>
                                            {jobReadyForInvoice(job) && (
                                                <RowCheckbox
                                                    checked={selection.isSelected(job._id)}
                                                    onChange={() => selection.toggle(job._id)}
                                                    ariaLabel={`Select ${job.challanNumber}`}
                                                />
                                            )}
                                        </td>
                                        {cols.map((c) => (
                                            <td key={c.key} className={c.mono ? "cell-mono" : ""}>
                                                {c.key === "challanNumber" ? (
                                                    <button type="button" className="lifecycle-link" onClick={() => setActiveJob(job)}>
                                                        {job.challanNumber}
                                                    </button>
                                                ) : c.key === "total" ? (
                                                    // Raw, this printed the float straight out - a job worth 11.80 showed as
                                                    // 11.799999999999999. Money is displayed through RoundOff everywhere else.
                                                    `₹${RoundOff(job[c.key])}`
                                                ) : c.key === "rows" ? (
                                                    job.rows?.length || 0
                                                ) : c.key === "clientFirm" ? (
                                                    job.client_id?.clientFirm || <span style={{ color: "var(--text-tertiary)" }}>—</span>
                                                ) : c.key === "clientPhone" ? (
                                                    job.client_id?.clientPhone || <span style={{ color: "var(--text-tertiary)" }}>—</span>
                                                ) : c.key === "quotationNumber" ? (
                                                    [...new Set((job.rows || []).map((r) => r.quotation_id?.quotationNumber).filter(Boolean))].join(", ") || (
                                                        <span style={{ color: "var(--text-tertiary)" }}>—</span>
                                                    )
                                                ) : c.key === "invoiceState" ? (
                                                    (() => {
                                                        const badge = invoiceBadge(job);
                                                        if (badge.label === "—") return <span style={{ color: "var(--text-tertiary)" }}>—</span>;
                                                        return <StatusBadge status={badge.status}>{badge.label}</StatusBadge>;
                                                    })()
                                                ) : c.key === "queueSummary" ? (
                                                    (() => {
                                                        const stages = [...new Set((job.rows || []).map((r) => r.queue || "Created"))];
                                                        if (stages.length === 0) return <span style={{ color: "var(--text-tertiary)" }}>—</span>;
                                                        return (
                                                            <span style={{ display: "inline-flex", gap: 4, flexWrap: "wrap" }}>
                                                                {stages.map((stage) => (
                                                                    <StatusBadge key={stage} status={QUEUE_VARIANT[stage] || "neutral"}>
                                                                        {stage}
                                                                    </StatusBadge>
                                                                ))}
                                                            </span>
                                                        );
                                                    })()
                                                ) : (
                                                    job[c.key] || <span style={{ color: "var(--text-tertiary)" }}>—</span>
                                                )}
                                            </td>
                                        ))}
                                        <td onClick={(e) => e.stopPropagation()}>
                                            {/* Employees only see the actions their permissions allow.
                                                The API enforces the same rules independently
                                                (requireCreate / requireDelete), so hiding a control is
                                                a convenience, not the security boundary. */}
                                            <RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
                                                {canEditJobs && (
                                                    <RowActionMenuItem
                                                        icon={Edit2}
                                                        onClick={() => {
                                                            setEditingJob(job);
                                                            setMoreMenu(-1);
                                                        }}
                                                    >
                                                        Edit
                                                    </RowActionMenuItem>
                                                )}
                                                {canDeleteJobs && (
                                                    <RowActionMenuItem
                                                        icon={Trash2}
                                                        onClick={() => {
                                                            setDeleteTarget(job);
                                                            setMoreMenu(-1);
                                                        }}
                                                    >
                                                        Delete
                                                    </RowActionMenuItem>
                                                )}
                                                {!canEditJobs && !canDeleteJobs && (
                                                    <RowActionMenuItem disabled>No actions available</RowActionMenuItem>
                                                )}
                                            </RowActionMenu>
                                        </td>
                                    </tr>
                                );
                            })
                            )}
                        </tbody>
                    </DataTable>
                    <div className="shell-card-footer">
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            {/* The page's share, then the total - "showing 25 of 340" answers
                                "where am I" in a way a bare total does not. */}
                            {perPage > 0 && total > perPage
                                ? `${pageRows.length} of ${total} jobs in ${activeCategory}`
                                : `${total} jobs in ${activeCategory}`}
                        </span>
                        <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                    </div>
                </div>
                    </>
                )}

                {view === "board" && (
                    <JobBoardGrid
                        jobs={boardJobs}
                        loading={loading}
                        onOpen={(job, rect) => setBoardJob({ job, rect })}
                        // Billing from the board, through exactly the path the table uses - the
                        // same selection, the same bulk bar, the same modal. A second way to
                        // raise an invoice is a second set of rules about what may be billed.
                        selection={selection}
                        isInvoiceable={jobReadyForInvoice}
                        // The same modal, the same permission gate as the table's own button.
                        onCreate={canEditJobs ? () => setCreateOpen(true) : undefined}
                    />
                )}
            </div>

            {boardJob && (
                <JobBoardOverlay
                    job={boardJob.job}
                    rect={boardJob.rect}
                    onClose={() => setBoardJob(null)}
                    // The list behind the board has to agree with what the board just did, or
                    // closing it would show the card back where it started.
                    onChanged={(updated) => {
                        if (!updated) return;
                        setJobs((prev) => prev.map((j) => (j._id === updated._id ? { ...j, ...updated } : j)));
                        setBoardJob((b) => (b ? { ...b, job: { ...b.job, ...updated } } : b));
                    }}
                />
            )}

            {justCreated && (
                <JobCreatedNotifyPrompt
                    job={justCreated}
                    // Which company default applies depends on which message this card is
                    // about, and only the card knows that (from the server's alert state).
                    autoSend={justCreated?.alerts?.created?.isUpdate ? notifyOnUpdate : notifyOnCreate}
                    onClose={(updated) => {
                        // A send returns the updated job, whose alert state decides what the
                        // detail view offers next - keep the board in step with it.
                        if (updated) setJobs((prev) => prev.map((j) => (j._id === updated._id ? updated : j)));
                        setJustCreated(null);
                    }}
                />
            )}
            <CreateJobModal
                isOpen={createOpen || !!editingJob}
                toggle={() => {
                    setCreateOpen(false);
                    setEditingJob(null);
                }}
                onCreate={onCreateJob}
                onUpdate={onUpdateJob}
                editingJob={editingJob}
                // Locking/unlocking from inside the edit form returns the updated job; keep
                // the open form and the board row in step with it.
                onLockChange={(updated) => {
                    setEditingJob(updated);
                    setJobs((prev) => prev.map((j) => (j._id === updated._id ? updated : j)));
                }}
            />
            <BulkActionsBar
                count={selection.count}
                itemLabel="Job-ids"
                actions={[
                    {
                        label: clientsInSelection.size > 1 ? "One client at a time" : "Invoice",
                        icon: FileText,
                        onClick: () => clientsInSelection.size === 1 && setInvoiceOpen(true),
                    },
                ]}
            />

            <CreateInvoiceFromJobsModal
                isOpen={invoiceOpen}
                toggle={() => setInvoiceOpen(false)}
                initialClient={
                    chosenJobs.length
                        ? {
                              id: String(chosenJobs[0].client_id?._id || chosenJobs[0].client_id),
                              name: chosenJobs[0].client_id?.clientFirm || chosenJobs[0].client_id?.clientName || "",
                          }
                        : null
                }
                initialJobIds={chosenJobs.map((j) => j._id)}
                onCreated={() => {
                    setInvoiceOpen(false);
                    selection.clear();
                    loadJobs();
                }}
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
            <ConfirmDialog
                open={!!deleteTarget}
                message={deleteTarget ? `Move ${deleteTarget.challanNumber} to trash? This can't be undone from here yet.` : ""}
                confirmLabel="Delete"
                onConfirm={confirmDelete}
                onCancel={() => setDeleteTarget(null)}
            />
        </>
    );
};

export default LifecycleIndex;
