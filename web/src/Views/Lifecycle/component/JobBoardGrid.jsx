import React, { useMemo, useRef, useState } from "react";
import { Lock, Plus } from "react-feather";

import SearchField from "../../../Common/SearchField";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import { jobGrandTotal } from "../jobMath";
import { cardSize } from "../jobBoard";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import RowCheckbox from "../../../Common/DataTable/RowCheckbox";
import "./jobBoard.css";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

// A fixed cap rather than a measurement. Measuring real overflow would make an equal-sized grid
// render differently depending on how long someone's product names happen to be, which fights
// the one property the grid is for. A long single name is truncated in CSS for the same reason.
const PRODUCTS_SHOWN = 3;

// Fixed for this view, rather than Appearance's rows-per-page. That setting is a reading
// preference about table rows; this is a grid whose cards are sized by column width, and 15
// fills it evenly at every width the grid actually uses - 3, 4 or 5 to a row.
const BOARD_PER_PAGE = 15;

// Name and size together, the same pairing the board's chips use - a job-id carrying three
// badges reading "Vinyl, Vinyl, Vinyl" says less than one carrying its sizes.
const productsOf = (job) =>
    (job.rows || [])
        .map((r) => ({ name: r.material || r.description || "", size: cardSize(r) }))
        .filter((p) => p.name);

/**
 * The job-ids, as cards.
 *
 * Search only - the category StatCards stay on the table view. They answer "which of these is
 * ready to invoice", a lookup question; this view answers "where is the work", and a second row
 * of filters above it would be furniture in the way.
 *
 * `onOpen(job, rect)` hands back the clicked card's rectangle, which is what lets the overlay
 * expand out of that card rather than fade in on the spot.
 */
export default function JobBoardGrid({ jobs = [], loading = false, onOpen, selection, isInvoiceable, onCreate }) {
    const [search, setSearch] = useState("");
    const cardRefs = useRef({});

    // Newest first. Sorted here rather than relying on the order the list arrived in, so the
    // grid says the same thing whatever the table happens to be sorted by.
    const ordered = useMemo(
        () => [...jobs].sort((a, b) => new Date(b.createdAt || 0) - new Date(a.createdAt || 0)),
        [jobs]
    );

    const filtered = useMemo(() => {
        const term = search.trim().toLowerCase();
        if (!term) return ordered;
        return ordered.filter((job) =>
            `${job.challanNumber || ""} ${productsOf(job)
                .map((p) => p.name)
                .join(" ")}`
                .toLowerCase()
                .includes(term)
        );
    }, [ordered, search]);

    // Deliberately NOT Appearance's rows-per-page - see BOARD_PER_PAGE.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(filtered, BOARD_PER_PAGE);

    return (
        <div>
            <div className="job-board-head">
                <SearchField
                    style={{ flex: 1, maxWidth: 480 }}
                    placeholder="Search job-id or product"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                />
                {/* The same modal the table raises a job-id with, at the far end of the same
                    row the search sits on - so "make a new one" is in the place it is in every
                    other list in this app rather than only in the view you came from.
                    Pushed right by its own margin rather than by space-between on the row: the
                    search has a max-width, so space-between would strand the button in the
                    middle of a wide screen. */}
                {onCreate && (
                    <button
                        type="button"
                        className="shell-btn shell-btn-sm shell-btn-primary job-board-create"
                        onClick={onCreate}
                    >
                        <Plus size={13} style={{ marginRight: 6, verticalAlign: "text-bottom" }} />
                        Create Job
                    </button>
                )}
            </div>

            {!loading && total === 0 ? (
                <p className="job-board-empty text-body-regular">
                    {search ? "No job-id matches that search." : "No job-ids with work in them."}
                </p>
            ) : (
                <>
                    <div className="job-board-grid">
                        {pageRows.map((job) => {
                            const products = productsOf(job);
                            const shown = products.slice(0, PRODUCTS_SHOWN);
                            const more = products.length - shown.length;
                            const cards = (job.rows || []).length;
                            // canEditQueue is false the moment ANY row is billed. A fully
                            // invoiced job-id is not on this board at all, so here it means
                            // exactly one thing: part-invoiced, and therefore frozen.
                            const locked = job.lock?.canEditQueue === false;
                            const invoicedRows = Number(job.invoicedRows) || 0;

                            // Only a billable job-id gets a box, the same rule the table's
                            // rows follow - offering a checkbox that leads to a refusal is
                            // worse than offering none.
                            const billable = Boolean(selection && isInvoiceable?.(job));

                            return (
                                <div
                                    key={job._id}
                                    className={[
                                        "job-board-card-wrap",
                                        // The whole card, not only the badge on it. Scanning a
                                        // grid for "what can I bill" is scanning for an edge,
                                        // not for a word inside twelve similar boxes.
                                        job.readyForInvoice && job.invoiceState !== "invoiced" ? "is-ready" : "",
                                        billable && selection.isSelected(job._id) ? "is-selected" : "",
                                    ]
                                        .filter(Boolean)
                                        .join(" ")}
                                >
                                    {/* A sibling of the card, not a child of it: the card is a
                                        <button>, and a checkbox inside a button is neither
                                        valid nor operable - the button swallows the click. */}
                                    {billable && (
                                        <span className="job-board-card-check">
                                            <RowCheckbox
                                                checked={selection.isSelected(job._id)}
                                                onChange={() => selection.toggle(job._id)}
                                                ariaLabel={`Select ${job.challanNumber}`}
                                            />
                                        </span>
                                    )}
                                <button
                                    type="button"
                                    ref={(el) => {
                                        cardRefs.current[job._id] = el;
                                    }}
                                    className="job-board-card"
                                    onClick={() => onOpen?.(job, cardRefs.current[job._id]?.getBoundingClientRect())}
                                >
                                    <span className="job-board-card-top">
                                        <span className="text-body-medium">{job.challanNumber}</span>
                                        {locked && (
                                            <Lock size={14} className="job-board-card-lock" title="Billed in part - unlock it to edit or move its cards." aria-label="Billed in part - locked" />
                                        )}
                                    </span>

                                    {/* Where the job stands, which arranging cards cannot say.
                                        readyForInvoice and invoiceState are computed on the API
                                        (Helpers/JobInvoiceState.js) and ride on every job, so
                                        this agrees with the table beside it rather than working
                                        it out a second way. */}
                                    <span className="job-board-card-badges">
                                        {job.invoiceState === "invoiced" ? (
                                            <StatusBadge status="neutral">Invoiced</StatusBadge>
                                        ) : job.invoiceState === "partial" ? (
                                            // The count, not the word. "Part-invoiced" says a
                                            // state; "2/6 billed" says how much work is left,
                                            // which is the question this board exists to
                                            // answer - and why the card cannot be rearranged.
                                            <StatusBadge status="amber">{`${invoicedRows}/${cards} billed`}</StatusBadge>
                                        ) : job.readyForInvoice ? (
                                            <StatusBadge status="green">Ready for invoice</StatusBadge>
                                        ) : null}
                                    </span>

                                    <span className="text-body-small job-board-card-meta">
                                        {money(job.total ?? jobGrandTotal(job.rows))} · {cards}{" "}
                                        {cards === 1 ? "job" : "jobs"}
                                    </span>

                                    <span className="job-board-card-products">
                                        {shown.map((p, i) => (
                                            <span key={`${p.name}-${i}`} className="job-board-product text-body-small">
                                                {p.name}
                                                {p.size && <span className="job-board-dims">{p.size}</span>}
                                            </span>
                                        ))}
                                        {more > 0 && (
                                            <span className="job-board-product is-more text-body-small">+{more} more</span>
                                        )}
                                    </span>
                                </button>
                                </div>
                            );
                        })}
                    </div>
                    {/* Its own row rather than butting up against the last row of cards -
                        a grid has no footer rule to separate them the way a table does. */}
                    <div className="job-board-pagination">
                        <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
                    </div>
                </>
            )}
        </div>
    );
}
