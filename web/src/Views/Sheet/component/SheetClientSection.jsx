import React from "react";

import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { initials } from "../../dashboardCards";
import JobCard from "../../../Common/cards/JobCard";

const money = (n) => `₹${RoundOff(Number(n) || 0)}`;
const isPaid = (job) => Number(job.total) > 0 && Number(job.advance) >= Number(job.total);

/**
 * One customer's work on one day.
 *
 * The header is the customer and what they owe; the cards under it are their job-ids. This
 * replaces a thirteen-column table per customer, which put the day's most-read page behind a
 * horizontal scroll and drew its tree with box-drawing characters in a cell.
 *
 * The job-id cards are Common/cards/JobCard - the same drawing the customer page uses, because
 * it is the same object. This section supplies only what "status" MEANS here: on a day's sheet
 * the question is whether the customer has paid for it.
 */
export default function SheetClientSection({ group, onOpenJob }) {
    return (
        <section className="sheet-client">
            <header className="sheet-client-head">
                <span className="sheet-client-who">
                    <span className="sheet-monogram" aria-hidden="true">
                        {initials(group.firm || group.person)}
                    </span>
                    <span className="sheet-client-names">
                        <span className="sheet-client-firm">{group.firm || "—"}</span>
                        {group.person && group.person !== group.firm && (
                            <span className="sheet-client-person">{group.person}</span>
                        )}
                    </span>
                </span>

                {/* What this customer's day came to. Due is the figure anyone is actually
                    looking for, so it is the one that keeps a colour. */}
                <span className="sheet-client-money">
                    <span className="sheet-money-item">
                        <span className="sheet-money-label">Job-ids</span>
                        <span className="sheet-money-value">{group.jobs.length}</span>
                    </span>
                    <span className="sheet-money-item">
                        <span className="sheet-money-label">Total</span>
                        <span className="sheet-money-value">{money(group.total)}</span>
                    </span>
                    <span className="sheet-money-item">
                        <span className="sheet-money-label">Received</span>
                        <span className="sheet-money-value">{money(group.advance)}</span>
                    </span>
                    <span className={`sheet-money-item${group.due > 0 ? " is-due" : ""}`}>
                        <span className="sheet-money-label">Due</span>
                        <span className="sheet-money-value">{money(group.due)}</span>
                    </span>
                </span>
            </header>

            <div className="job-card-deck">
                {group.jobs.map((job) => (
                    <JobCard
                        key={job._id}
                        job={job}
                        onOpen={onOpenJob}
                        badge={
                            <StatusBadge status={isPaid(job) ? "paid" : "open"}>
                                {isPaid(job) ? "Paid" : "Due"}
                            </StatusBadge>
                        }
                    />
                ))}
            </div>
        </section>
    );
}
