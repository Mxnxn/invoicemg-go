import React, { useState, useEffect, useCallback, useMemo } from "react";
/* eslint-disable */
import LiteHeader from "../../../Common/Header/LiteHeader";
import {
    Card,
    CardHeader,
    Button,
    CardBody,
    Container,
    Col,
    Row,
    CardTitle,
    Modal,
    ModalHeader,
    ModalBody,
    ModalFooter,
    Input,
    CardFooter,
    Nav,
} from "reactstrap";
import InvoicePagination from "./InvoicePagination";
import InvoiceDetailModal from "./InvoiceDetailModal";
import usePagedRows from "../../../Common/useRowsPerPage";
import InvoicePreviewModal from "./InvoicePreviewModal";
import { invoiceBackend } from "../invoice_backend";
import { CheckCircle, ChevronDown, ChevronUp, Eye, Search, Trash, Trash2, TrendingUp, X, Plus } from "react-feather";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import ModalDelete from "../../Client/component/ModalDelete";
import ConfirmModal from "./ConfirmModal";
import { getDateForEntry } from "../../../Common/DateAndTime/getDate";
import InputGroup from "reactstrap/lib/InputGroup";
import { useDispatch, useSelector } from "react-redux";
import { getAllGeneratedInvoices } from "../../../Redux/Actions/Grab";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import useInnerWidth from "../../../Common/DateAndTime/useInnerWidth";
import DataTable from "../../../Common/DataTable/DataTable";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import NumberBadge from "../../../Common/DataTable/NumberBadge";
import RowCheckbox from "../../../Common/DataTable/RowCheckbox";
import BulkActionsBar from "../../../Common/DataTable/BulkActionsBar";
import useRowSelection from "../../../Common/DataTable/useRowSelection";
import RowActionMenu, { RowActionMenuItem } from "../../../Common/DataTable/RowActionMenu";
import CreateInvoiceFromJobsModal from "./CreateInvoiceFromJobsModal";
import { can } from "../../../Common/access";
import SearchField from "../../../Common/SearchField";
import DocumentCard from "../../../Common/cards/DocumentCard";
import ViewToggle, { VIEW_BOARD, readView } from "../../../Common/cards/ViewToggle";
import { filterInvoicesByGst, countInvoicesByGst } from "../../../Common/gstKind";
const DEMO_INVOICE = {
    date: new Date(),
    invoiceNumber: "DEMO-0001",
    firm: "Your Firm Name",
    address: "123 Business Street, Industrial Area",
    phone: "+91 98765 43210",
    email: "billing@yourfirm.com",
    // Bare, like a real Company record - the templates print their own "GST " label, so a
    // number carrying one too renders "GST GSTIN: 22AAA...".
    gst: "22AAAAA0000A1Z5",
    clientFirm: "Sample Client Pvt. Ltd.",
    clientAddress: "456 Client Avenue, Business District",
    clientPhone: "+91 91234 56789",
    clientGST: "22BBBBB0000B1Z5",
    bank_name: "Sample Bank",
    account: "1234567890",
    ifsc: "SAMP0001234",
    entries: [
        { product: "Granite Slab", description: "Polished, Grade A", qty: 2, length: 8, width: 4, rate: 120 },
        { product: "Marble Tile", description: "Matte finish", qty: 10, length: 2, width: 2, rate: 60 },
        { product: "Sandstone Block", description: "Rough cut", qty: 5, length: 3, width: 3, rate: 45 },
    ],
};

// The sortable columns, in table order. `key` is the field on the row shape /invoice/getAll
// returns; `due` is derived (total - receivedAmount) and handled in the comparator.
const COLUMNS = [
    { key: "invoiceNumber", label: "Inv No", sortable: true },
    { key: "createdAt", label: "Date", sortable: true },
    { key: "clientName", label: "To", sortable: true },
    { key: "clientFirm", label: "Firm", sortable: true },
    { key: "total", label: "Total", sortable: true },
    { key: "notTaxAmount", label: "Excl. Tax", sortable: true },
    { key: "receivedAmount", label: "Received", sortable: true },
    { key: "due", label: "Due Amount", sortable: true },
    { key: "preview", label: "Preview", sortable: false },
];

// Compared as numbers, not as text - "9" sorts after "10" as a string.
const SORT_NUMERIC = new Set(["total", "notTaxAmount", "receivedAmount", "due"]);

const SortIcon = ({ direction }) => {
    if (!direction) return null;
    return direction === "asc" ? (
        <ChevronUp size={12} style={{ marginLeft: 4, verticalAlign: "middle" }} />
    ) : (
        <ChevronDown size={12} style={{ marginLeft: 4, verticalAlign: "middle" }} />
    );
};

const InvoiceIndex = ({ user }) => {
    console.log(user);
    const [demoPreviewOpen, setDemoPreviewOpen] = useState(false);
    const [createFromJobs, setCreateFromJobs] = useState(false);
    const [state, setState] = useState({
        invoices: [],
        preview: { view: false },
        copyInvoices: [],
        invoice_id: "",
        client_id: "",
        closingInvoice: null,
    });
    const [modals, setModals] = useState({
        delete: false,
        receive: false,
    });
    const [copyInvoices, setCopyInvoices] = useState([]);
    // `ready` - the data has arrived and the page may render. NOT a loading flag, despite what
    // it used to be called: it starts false and is set true AFTER the fetch, and the whole page
    // is gated on it below.
    //
    // Renamed because the name caused a real bug. Wiring the shimmer in read it as "is
    // loading" and passed it straight to DataTable, which inverted it - the table then
    // shimmered for ever, starting the moment the rows were actually available.
    const [ready, setReady] = useState(false);
    const [view, setView] = useState(() => readView("invoices"));
    const [search, setSearch] = useState("");
    const getInvoices = useCallback(async () => {
        try {
            const formData = new FormData();
            formData.set("uid", window.localStorage.getItem("uid"));
            const res = await invoiceBackend.getInvoices(formData);
            // Order is the table's business now (see `sort` / visibleInvoices), not the
            // fetch's. This used to sort by invoice number as text, which put MG/26-27/9 after
            // MG/26-27/10 and reshuffled nothing when the user clicked a column.
            const temp = [...res.data];
            setCopyInvoices([...temp]);
            setState({
                ...state,
                invoices: temp,
                copyInvoices: temp,
            });
        } catch (error) {
            console.log(error);
        } finally {
            // In a finally so a failed fetch still renders the page. It used to be set only on
            // success, so an error left the whole screen blank for ever with the reason only in
            // the console - an empty table at least says what happened.
            setReady(true);
        }
    }, []);

    const deleteHandler = async () => {
        try {
            const formData = new FormData();
            formData.set("invoice_id", state.invoice_id);
            const res = await invoiceBackend.deleteInvoice(formData);
            setModals({ ...modals, delete: false });
            const index = state.invoices.findIndex((el) => el._id === state.invoice_id);
            state.invoices.splice(index, 1);
            setState({ ...state, invoice_id: "" });
            setMoreMenu(-1);
        } catch (error) {
            console.log(error);
        }
    };

    useEffect(() => {
        getInvoices();
    }, [getInvoices]);

    const onInvoicePaid = () => {
        setModals((prev) => ({ ...prev, receive: false }));
        setState((prev) => ({ ...prev, invoice_id: "", client_id: "", closingInvoice: null }));
        getInvoices();
    };

    useEffect(() => {
        if (search) {
            const searchResult = copyInvoices.filter(
                (elem) => elem.clientFirm.toUpperCase().includes(search.toUpperCase()) || elem.invoiceNumber.toString().includes(search)
            );
            setState((prev) => ({ ...prev, invoices: searchResult }));
        } else {
            setState((prev) => ({ ...prev, invoices: copyInvoices }));
        }
    }, [search, copyInvoices]);

    // One customer's invoices, in the table itself. This replaces the modal the client name
    // used to open: narrowing in place keeps the sort, the GST tab, the pagination and every
    // column, where the modal had its own reduced list and covered the one you came from.
    const [clientFilter, setClientFilter] = useState({ id: "", name: "" });

    // Built from the invoices actually loaded rather than from /client/list: a customer with
    // no invoices cannot narrow this table to anything, so offering them is a dead end.
    const clientOptions = useMemo(() => {
        const byId = new Map();
        (state.invoices || []).forEach((inv) => {
            const id = String(inv.client_id || "");
            if (!id || byId.has(id)) return;
            byId.set(id, { id, name: inv.clientFirm || inv.clientName || "Unnamed" });
        });
        return [...byId.values()].sort((a, b) => a.name.localeCompare(b.name));
    }, [state.invoices]);

    // Interstate invoices file differently from intrastate ones, so they are listed apart
    // rather than hidden in a wide table behind a non-zero IGST column.
    const [gstKind, setGstKind] = useState("all");
    const clientInvoices = useMemo(
        () =>
            clientFilter.id
                ? (state.invoices || []).filter((inv) => String(inv.client_id) === String(clientFilter.id))
                : state.invoices,
        [state.invoices, clientFilter.id]
    );
    const gstCounts = useMemo(() => countInvoicesByGst(clientInvoices), [clientInvoices]);
    const gstFiltered = useMemo(() => filterInvoicesByGst(clientInvoices, gstKind), [clientInvoices, gstKind]);

    // Newest first by default, on createdAt - when the invoice was RAISED, not the date typed
    // on its face. Two invoices often carry the same printed date and one of them was still
    // entered second; sorting on `date` shuffled those arbitrarily on every load.
    const [sort, setSort] = useState({ key: "createdAt", direction: "desc" });

    const onSortClick = (key) => {
        setSort((prev) => {
            if (prev.key !== key) return { key, direction: "asc" };
            if (prev.direction === "asc") return { key, direction: "desc" };
            // Third press returns to the default order rather than to an unsorted one: a list
            // with no order at all is not a state worth offering.
            return { key: "createdAt", direction: "desc" };
        });
    };

    const visibleInvoices = useMemo(() => {
        const rows = [...gstFiltered];
        const { key, direction } = sort;
        const dir = direction === "asc" ? 1 : -1;
        return rows.sort((a, b) => {
            let av = a[key];
            let bv = b[key];
            if (key === "due") {
                av = Number(a.total) - Number(a.receivedAmount);
                bv = Number(b.total) - Number(b.receivedAmount);
            }
            if (key === "createdAt" || key === "date") {
                av = new Date(av || 0).getTime();
                bv = new Date(bv || 0).getTime();
            }
            if (SORT_NUMERIC.has(key)) {
                av = Number(av) || 0;
                bv = Number(bv) || 0;
            }
            // Natural comparison for text: an invoice number is mostly digits, and plain
            // string order puts MG/26-27/9 after MG/26-27/10.
            if (typeof av === "string" || typeof bv === "string") {
                const cmp = String(av ?? "").localeCompare(String(bv ?? ""), undefined, {
                    numeric: true,
                    sensitivity: "base",
                });
                if (cmp !== 0) return cmp * dir;
            } else {
                if (av < bv) return -dir;
                if (av > bv) return dir;
            }
            // Ties fall back to newest-raised, so equal values do not reshuffle per render.
            return new Date(b.createdAt || 0) - new Date(a.createdAt || 0);
        });
    }, [gstFiltered, sort]);

    // Was a hand-rolled pager with its own hardcoded 30. It worked, but it was the one list
    // that ignored Account settings > Appearance > Rows per page - so changing the setting
    // moved every table except this one. useRowsPerPage also handles the case this version
    // did not: filtering down to fewer rows than the current page would strand you on an
    // empty page with only the pager to escape it.
    const {
        pageRows: splittedEntries,
        page: currentPage,
        setPage: setCurrentPage,
        perPage: entriesPerPage,
    } = usePagedRows(visibleInvoices);
    const [moreMenu, setMoreMenu] = useState(-1);
    const [detailInvoice, setDetailInvoice] = useState(null);

    const invoiceSelection = useRowSelection(splittedEntries);

    const bulkDeleteInvoices = async () => {
        if (!window.confirm(`Delete ${invoiceSelection.count} selected invoice(s)?`)) return;
        for (const id of invoiceSelection.selectedIds) {
            const formData = new FormData();
            formData.set("invoice_id", id);
            await invoiceBackend.deleteInvoice(formData);
        }
        invoiceSelection.clear();
        getInvoices();
    };

    // An invoice's entries have no back-reference to the Job(s) they were converted from
    // (Entry model doesn't track it), so this can only compare entry-level settlement, not
    // job-level - but that's still exactly what "some of what's bundled here is paid, some
    // isn't" means from the invoice's point of view.
    const isMixedPayment = (invoice) => {
        const entries = invoice.entries || [];
        if (entries.length < 2) return false;
        const paidCount = entries.filter((entry) => Number(entry.total) === 0).length;
        return paidCount > 0 && paidCount < entries.length;
    };

    const innerW = useInnerWidth();

    const [responsive, setResponsive] = useState(innerW < 1280 ? true : false);

    useEffect(() => {
        if (innerW < 1280) setResponsive(true);
        else setResponsive(false);
    }, [innerW]);

    // Rendered straight away rather than held back until the data lands: the header, the search
    // and the tabs do not depend on the rows, and the table shimmers in place while they load.
    // Gating the whole page on `ready` meant a blank screen instead of a page filling in.
    return (
            <>
                <LiteHeader bg="primary" />

                <Container className=" " fluid>
                    <Row>
                        <Col className="d-flex flex-row-reverse"></Col>
                    </Row>
                    <Row className="mt-2">
                        <Col xl="12">
                            <div className={`shell-card${view === VIEW_BOARD ? " is-deck" : ""}`}>
                                <div className="shell-card-header invoice-header">
                                    {/* Both ways of narrowing the view, stacked: how the list is
                                        drawn, then which slice of it. They are the same kind of
                                        decision, so they share a column on the left rather than
                                        one sitting in the header and the other in a strip of its
                                        own underneath. */}
                                    <div className="invoice-header-views">
                                        <ViewToggle view={view} onChange={setView} storageKey="invoices" />
                                        <div className="invoice-gst-tabs">
                                            <div className="shell-segmented" role="tablist" aria-label="GST type">
                                                {[
                                                    { id: "all", label: "All" },
                                                    { id: "gst", label: "GST" },
                                                    { id: "igst", label: "IGST" },
                                                ].map((tab) => (
                                                    <button
                                                        key={tab.id}
                                                        type="button"
                                                        role="tab"
                                                        aria-selected={gstKind === tab.id}
                                                        className={["shell-segmented-btn", gstKind === tab.id ? "active" : ""]
                                                            .filter(Boolean)
                                                            .join(" ")}
                                                        onClick={() => {
                                                            setGstKind(tab.id);
                                                            // Page 3 of "All" is usually past the end of
                                                            // a narrower tab, which reads as an empty
                                                            // table.
                                                            setCurrentPage(1);
                                                        }}
                                                    >
                                                        {tab.label}
                                                        <span className="text-body-small" style={{ marginLeft: 6, opacity: 0.75 }}>
                                                            {gstCounts[tab.id]}
                                                        </span>
                                                    </button>
                                                ))}
                                            </div>
                                        </div>
                                    </div>
                                    {/* Narrow the list, then act on it. Spaced apart rather than
                                        butted together: they are three separate decisions, and a
                                        seamless bar said they were one control. */}
                                    <div className="list-header-actions">
                                        <div className="list-header-group">
                                            {/* Searchable rather than a plain select - the list grows
                                                with the customer base. Clearing it (the x) returns to
                                                every client. */}
                                            <div className="invoice-header-filter">
                                                <AssigneeDropdown
                                                    value={clientFilter.name}
                                                    placeholder="All clients"
                                                    options={clientOptions}
                                                    fullWidth
                                                    onSelect={(id, name) => {
                                                        setClientFilter({ id: id || "", name: name || "" });
                                                        // Page 4 of every client is usually past the end
                                                        // of one of them, which reads as an empty table.
                                                        setCurrentPage(1);
                                                    }}
                                                />
                                            </div>
                                            <SearchField
                                                className="list-header-search"
                                                placeholder="Company or Invoice Number"
                                                value={search}
                                                onChange={(evt) => {
                                                    setSearch(evt.target.value);
                                                }}
                                            />
                                            {/* Invoices are stored against Entries, but people think in
                                                job-ids - this builds one from the jobs directly. Hidden
                                                without create permission; /invoice/save enforces it too. */}
                                            {can("invoices", "create") && (
                                                <button
                                                    type="button"
                                                    className="shell-btn shell-btn-primary invoice-create-btn"
                                                    onClick={() => setCreateFromJobs(true)}
                                                >
                                                    <Plus size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
                                                    Create Invoice
                                                </button>
                                            )}
                                        </div>
                                        {/* Under Create rather than beside it. "Preview Demo Invoice"
                                            spelled out sat at the same weight as Create, as though
                                            looking at a sample and raising a real invoice were
                                            comparable acts; a second line, smaller and quieter, says
                                            which of the two the page is actually for. */}
                                        <button
                                            type="button"
                                            className="shell-btn shell-btn-sm shell-btn-secondary d-flex align-items-center invoice-demo-btn"
                                            style={{ gap: 6 }}
                                            title="Preview a sample invoice in the current design"
                                            onClick={() => setDemoPreviewOpen(true)}
                                        >
                                            <Eye size={14} />
                                            Demo
                                        </button>
                                    </div>
                                </div>
                                {view === VIEW_BOARD ? (
                                    !ready ? (
                                        <p className="doc-deck-note text-body-small">Loading...</p>
                                    ) : splittedEntries.length === 0 ? (
                                        <p className="doc-deck-note text-body-small">Nothing to show.</p>
                                    ) : (
                                        <div className="doc-card-deck doc-deck-padded">
                                            {splittedEntries.map((el) => {
                                                const due = Number(el.total) - Number(el.receivedAmount);
                                                return (
                                                    <DocumentCard
                                                        key={el._id}
                                                        number={el.invoiceNumber}
                                                        party={el.clientFirm}
                                                        partySub={el.clientName}
                                                        date={getDateForEntry(el.date)}
                                                        selectable
                                                        selected={invoiceSelection.isSelected(el._id)}
                                                        onToggleSelect={() => invoiceSelection.toggle(el._id)}
                                                        ariaLabel={`Select invoice ${el.invoiceNumber}`}
                                                        figures={[
                                                            { label: "Total", value: `\u20b9${RoundOff(el.total)}` },
                                                            { label: "Received", value: `\u20b9${RoundOff(el.receivedAmount)}` },
                                                            {
                                                                label: "Due",
                                                                value: `\u20b9${RoundOff(due)}`,
                                                                tone: RoundOff(due) === "0.00" ? "good" : "due",
                                                            },
                                                        ]}
                                                        badges={
                                                            isMixedPayment(el) ? (
                                                                <StatusBadge
                                                                    status="pending"
                                                                    title="This invoice bundles entries with different payment statuses"
                                                                >
                                                                    Mixed
                                                                </StatusBadge>
                                                            ) : null
                                                        }
                                                        onOpen={() => setDetailInvoice(el)}
                                                        // The same preview the table offers
                                                        // from its row menu, on the card where
                                                        // the card is. It sets the same state,
                                                        // so one modal serves both views.
                                                        actions={
                                                            <button
                                                                type="button"
                                                                className="doc-card-action"
                                                                title={`Preview ${el.invoiceNumber}`}
                                                                aria-label={`Preview ${el.invoiceNumber}`}
                                                                onClick={() =>
                                                                    setState((prev) => ({
                                                                        ...prev,
                                                                        preview: { ...el, account: user.account, view: true },
                                                                    }))
                                                                }
                                                            >
                                                                <Eye size={14} />
                                                            </button>
                                                        }
                                                    />
                                                );
                                            })}
                                        </div>
                                    )
                                ) : (
                                <DataTable loading={!ready}>
                                    <thead>
                                        <tr>
                                            <th scope="col" className="xan-col-checkbox">
                                                <RowCheckbox
                                                    checked={invoiceSelection.allSelected}
                                                    indeterminate={invoiceSelection.someSelected && !invoiceSelection.allSelected}
                                                    onChange={invoiceSelection.toggleAll}
                                                    ariaLabel="Select all invoices"
                                                />
                                            </th>
                                            {COLUMNS.map((c) => (
                                                <th
                                                    key={c.key}
                                                    scope="col"
                                                    onClick={c.sortable ? () => onSortClick(c.key) : undefined}
                                                    style={{ cursor: c.sortable ? "pointer" : "default", userSelect: "none" }}
                                                    aria-sort={
                                                        sort.key === c.key
                                                            ? sort.direction === "asc"
                                                                ? "ascending"
                                                                : "descending"
                                                            : "none"
                                                    }
                                                >
                                                    {c.label}
                                                    {c.sortable && (
                                                        <SortIcon direction={sort.key === c.key ? sort.direction : null} />
                                                    )}
                                                </th>
                                            ))}
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {splittedEntries.map((el, idx) => (
                                            <tr key={el._id} className={invoiceSelection.isSelected(el._id) ? "selected" : ""}>
                                                <td className="xan-col-checkbox">
                                                    <RowCheckbox
                                                        checked={invoiceSelection.isSelected(el._id)}
                                                        onChange={() => invoiceSelection.toggle(el._id)}
                                                        ariaLabel={`Select invoice ${el.invoiceNumber}`}
                                                    />
                                                </td>
                                                <td className="cell-mono">
                                                    {/* The invoice number opens the invoice, the client name opens
                                                        that customer's whole list - two different questions, so two
                                                        different triggers. Same pattern as the Job Number in Jobs. */}
                                                    <NumberBadge
                                                        accent
                                                        onClick={() => setDetailInvoice(el)}
                                                        title="Open this invoice"
                                                    >
                                                        {el.invoiceNumber}
                                                    </NumberBadge>
                                                    {isMixedPayment(el) && (
                                                        <StatusBadge
                                                            status="pending"
                                                            title="This invoice bundles entries with different payment statuses"
                                                        >
                                                            Mixed
                                                        </StatusBadge>
                                                    )}
                                                </td>
                                                <td className="cell-mono">{getDateForEntry(el.date)}</td>
                                                {/* Plain text. This used to open that customer's entire
                                                    invoice list in a modal, which answered a question the
                                                    table itself now answers better: the Client filter above
                                                    narrows the list in place, keeping the sort, the GST tab,
                                                    the totals and every column - none of which the modal
                                                    carried. /invoice/:cid is still a real URL for deep links. */}
                                                <td>{el.clientName}</td>
                                                <td>{el.clientFirm}</td>
                                                <td className="cell-mono">₹{RoundOff(el.total)}</td>
                                                <td className="cell-mono">₹{RoundOff(el.notTaxAmount)}</td>
                                                <td className="cell-mono">
                                                    ₹{RoundOff(el.receivedAmount)}
                                                    {Number(el.receivedAmount) !== Number(el.total) && el.receivedAmount !== undefined && (
                                                        <StatusBadge status="overdue">Due</StatusBadge>
                                                    )}
                                                </td>
                                                <td className="cell-mono">₹{RoundOff(Number(el.total) - Number(el.receivedAmount))}</td>
                                                <td>
                                                    <RowActionMenu
                                                        open={moreMenu === idx}
                                                        onOpenChange={(next) => setMoreMenu(next ? idx : -1)}
                                                    >
                                                        <RowActionMenuItem
                                                            icon={CheckCircle}
                                                            variant={
                                                                RoundOff(Number(el.total) - Number(el.receivedAmount)) === "0.00"
                                                                    ? "success"
                                                                    : "warning"
                                                            }
                                                            disabled={RoundOff(Number(el.total) - Number(el.receivedAmount)) === "0.00"}
                                                            onClick={() => {
                                                                if (RoundOff(Number(el.total) - Number(el.receivedAmount)) === "0.00") return;
                                                                setModals((prev) => {
                                                                    return { ...prev, receive: true };
                                                                });
                                                                setState({
                                                                    ...state,
                                                                    invoice_id: el._id,
                                                                    client_id: el.client_id,
                                                                    closingInvoice: el,
                                                                });
                                                                setMoreMenu(-1);
                                                            }}
                                                        >
                                                            {RoundOff(Number(el.total) - Number(el.receivedAmount)) === "0.00" ? "Closed" : "Close Invoice"}
                                                        </RowActionMenuItem>
                                                        <RowActionMenuItem
                                                            icon={Eye}
                                                            onClick={() => {
                                                                if (state.preview.view) {
                                                                    setState({
                                                                        ...state,
                                                                        preview: {
                                                                            view: false,
                                                                        },
                                                                    });
                                                                } else {
                                                                    setState({
                                                                        ...state,
                                                                        preview: {
                                                                            ...el,
                                                                            account: user.account,
                                                                            view: true,
                                                                        },
                                                                    });
                                                                }
                                                                setMoreMenu(-1);
                                                            }}
                                                        >
                                                            {state.preview.view && el._id === state.preview._id ? "Cancel" : "Preview"}
                                                        </RowActionMenuItem>
                                                        <RowActionMenuItem
                                                            icon={Trash2}
                                                            variant="danger"
                                                            onClick={() => {
                                                                setModals({
                                                                    ...modals,
                                                                    delete: true,
                                                                });
                                                                setState({
                                                                    ...state,
                                                                    invoice_id: el._id,
                                                                });
                                                                setMoreMenu(-1);
                                                            }}
                                                        >
                                                            Delete
                                                        </RowActionMenuItem>
                                                    </RowActionMenu>
                                                    {/* <Button size="sm" color="success">
                                                        <TrendingUp size="14" style={{ verticalAlign: "sub" }} />
                                                    </Button> */}
                                                    {/* 
                                                    <Button
                                                        size="sm"
                                                        color={state.preview.view && el._id === state.preview._id ? "warning" : "primary"}
                                                        onClick={(evt) => {
                                                            if (state.preview.view) {
                                                                return setState({
                                                                    ...state,
                                                                    preview: {
                                                                        view: false,
                                                                    },
                                                                });
                                                            }
                                                            setState({
                                                                ...state,
                                                                preview: {
                                                                    ...el,
                                                                    date: getDate(el.date),
                                                                    view: true,
                                                                },
                                                            });
                                                        }}
                                                    >
                                                        {state.preview.view && el._id === state.preview._id ? (
                                                            <X
                                                                size="14"
                                                                style={{
                                                                    verticalAlign: "sub",
                                                                }}
                                                            />
                                                        ) : (
                                                            <Eye
                                                                size="14"
                                                                style={{
                                                                    verticalAlign: "sub",
                                                                }}
                                                            />
                                                        )}
                                                    </Button>
                                                    <Button
                                                        className="ml-2"
                                                        size="sm"
                                                        color="danger"
                                                        onClick={() => {
                                                            setModals({
                                                                ...modals,
                                                                delete: true,
                                                            });
                                                            setState({
                                                                ...state,
                                                                invoice_id: el._id,
                                                            });
                                                        }}
                                                    >
                                                        <Trash size="14" style={{ verticalAlign: "sub" }} />
                                                    </Button> */}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </DataTable>
                                )}
                                <div className="shell-card-footer">
                                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                        {visibleInvoices.length} invoices
                                    </span>
                                    <InvoicePagination
                                        // The filtered list, or the pager offers pages the
                                        // active tab has nothing to put on.
                                        invoices={visibleInvoices}
                                        invoicesPerPage={entriesPerPage}
                                        setCurrentPage={setCurrentPage}
                                        currentPage={currentPage}
                                    />
                                </div>
                            </div>
                        </Col>
                        <ModalDelete
                            modal={modals.delete}
                            onSubmitHandler={deleteHandler}
                            onCancelHandler={() => {
                                setModals({ ...modals, delete: false });
                            }}
                        />
                        {state.invoice_id && (
                            <ConfirmModal
                                modal={modals.receive}
                                invoice={state.closingInvoice}
                                onCancelHandler={() => {
                                    setModals({ ...modals, receive: false });
                                    setState({ ...state, invoice_id: "", client_id: "", closingInvoice: null });
                                }}
                                onPaid={onInvoicePaid}
                            />
                        )}
                    </Row>
                    <CreateInvoiceFromJobsModal
                        isOpen={createFromJobs}
                        toggle={() => setCreateFromJobs(false)}
                        onCreated={() => getInvoices()}
                    />
                    <InvoicePreviewModal
                        isOpen={state.preview.view}
                        toggle={() => setState({ ...state, preview: { view: false } })}
                        invoice={state.preview}
                    />
                    <InvoicePreviewModal isOpen={demoPreviewOpen} toggle={() => setDemoPreviewOpen(false)} invoice={DEMO_INVOICE} />
                </Container>
                <BulkActionsBar
                    count={invoiceSelection.count}
                    itemLabel="Invoices"
                    actions={[{ label: "Delete", icon: Trash2, onClick: bulkDeleteInvoices }]}
                />
                {detailInvoice && (
                    <InvoiceDetailModal invoice={detailInvoice} onClose={() => setDetailInvoice(null)} />
                )}
            </>
    );
};

export default InvoiceIndex;
