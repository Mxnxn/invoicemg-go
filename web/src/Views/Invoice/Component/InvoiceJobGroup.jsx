import React from "react";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { rowTotal } from "../../Lifecycle/jobMath";
import Blank from "../../../Common/DataTable/Blank";

// The job-ids one invoice bills, rendered under that invoice's row with the same tree
// connector strings the Sheet detail used to use before it became a card grid - job-id
// as the branch, its job-entries beneath it. Clicking a job-id opens the same JobDetailModal
// job-card the Sheet detail opens, so a job reads identically from either side.
const InvoiceJobGroup = ({ jobs, columnCount, onOpenJob }) => {
    if (!jobs || jobs.length === 0) return null;
    return (
        <>
            {jobs.map((job, jobIndex) => {
                const rows = job.rows || [];
                const isLastJob = jobIndex === jobs.length - 1;
                return (
                    <React.Fragment key={job._id}>
                        <tr className="sheet-tree-root" onClick={() => onOpenJob(job)} style={{ cursor: "pointer" }}>
                            <td className="cell-mono">
                                <span style={{ color: "var(--text-tertiary)" }}>{isLastJob && rows.length === 0 ? "└─ " : "├─ "}</span>
                            </td>
                            <td className="cell-mono" style={{ fontWeight: 700 }}>
                                {job.challanNumber}
                            </td>
                            <td className="cell-mono">{rows.length} item{rows.length === 1 ? "" : "s"}</td>
                            <td className="cell-mono">
                                <StatusBadge status={job.queue === "Done" ? "paid" : "pending"}>{job.queue || "Created"}</StatusBadge>
                            </td>
                            <td className="cell-mono">&#8377;{RoundOff(job.advance)}</td>
                            <td className="cell-mono">&#8377;{RoundOff(job.total)}</td>
                            {/* Keep the row's cell count equal to the header's, whatever the
                                parent table is currently rendering. */}
                            {Array.from({ length: Math.max(0, columnCount - 6) }).map((_, i) => (
                                <td key={i} />
                            ))}
                        </tr>
                        {rows.map((row, index) => {
                            const isLast = index === rows.length - 1;
                            return (
                                <tr key={row._id || index} className="sheet-tree-child">
                                    <td />
                                    <td className="cell-mono">
                                        <span style={{ color: "var(--text-tertiary)" }}>{isLast ? "   └─ " : "   ├─ "}</span>
                                        {row.material || <Blank />}
                                    </td>
                                    <td>{row.description || <Blank />}</td>
                                    <td className="cell-mono">{row.qty}</td>
                                    <td className="cell-mono">&#8377;{row.rate}</td>
                                    <td className="cell-mono">&#8377;{RoundOff(rowTotal(row))}</td>
                                    {Array.from({ length: Math.max(0, columnCount - 6) }).map((_, i) => (
                                        <td key={i} />
                                    ))}
                                </tr>
                            );
                        })}
                    </React.Fragment>
                );
            })}
        </>
    );
};

export default InvoiceJobGroup;
