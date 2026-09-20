import React, { useEffect, useState } from "react";
import ReportDownloads from "../../../Common/reports/ReportDownloads";
import defaultDateRange from "../../../Common/DateAndTime/defaultRange";
import DataTable from "../../../Common/DataTable/DataTable";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import DateField from "../../../Common/DateField";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import { lifecycleBackend } from "../../Lifecycle/lifecycle_backend";
import { jobReportBackend } from "../job_report_backend";
import { notifyError } from "../../../global/toast";

// Job Date carries the time (a job is created at a moment, not on a day), so it is formatted
// "04-Apr-2026 12:08:54" rather than through getDate, which drops the clock. Local time, like
// everything else the operator reads on screen.
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const two = (n) => String(n).padStart(2, "0");
const formatDateTime = (value) => {
    if (!value) return "";
    const d = new Date(value);
    if (Number.isNaN(d.getTime())) return "";
    return `${two(d.getDate())}-${MONTHS[d.getMonth()]}-${d.getFullYear()} ${two(d.getHours())}:${two(d.getMinutes())}:${two(d.getSeconds())}`;
};

const num = (value) => RoundOff(Number(value) || 0);

// The report's shape, once. `job` columns carry the header row; `line` columns carry each item
// row; `finalBill` is the header again. The screen table and the two downloads all read this so
// they cannot show different columns. Widths are WEIGHTS, normalised to 100% for the exports the
// same way the GST report does it - a react-pdf row wider than 100% overlaps its cells.
const COLUMNS = [
    { key: "jobcardNo", label: "Jobcard No", weight: 8, group: "job" },
    { key: "invoiceNo", label: "Invoice No", weight: 8, group: "job" },
    { key: "poNo", label: "Po No", weight: 4, group: "job" },
    { key: "jobDate", label: "Job Date", weight: 11, group: "job" },
    { key: "customerName", label: "Customer Name", weight: 13, group: "job" },
    { key: "mobile", label: "Mobile No", weight: 8, group: "job" },
    { key: "jobName", label: "Job Name", weight: 11, group: "job" },
    { key: "media", label: "Media Name", weight: 9, group: "line" },
    { key: "hsn", label: "HSN/SAC", weight: 6, group: "line" },
    { key: "description", label: "Description", weight: 9, group: "line" },
    { key: "size", label: "Size", weight: 6, group: "line" },
    { key: "rate", label: "Rate", weight: 5, group: "line", align: "right" },
    { key: "qty", label: "Quantity", weight: 6, group: "line", align: "right" },
    { key: "area", label: "Area", weight: 6, group: "line", align: "right" },
    { key: "tax", label: "Tax", weight: 6, group: "line", align: "right" },
    { key: "amount", label: "Amount", weight: 7, group: "line", align: "right" },
    { key: "finalBill", label: "Final Bill", weight: 8, group: "final", align: "right" },
];

// Flatten the grouped data to the rows the exports take: a header cell-array per job, then a
// cell-array per line under it, in the same column order the table renders.
const buildExport = (jobs, grandTotal) => {
    const totalWeight = COLUMNS.reduce((sum, c) => sum + c.weight, 0);
    const columns = COLUMNS.map(({ weight, label, align }) => ({
        label,
        align,
        width: `${((weight / totalWeight) * 100).toFixed(3)}%`,
    }));

    const rows = [];
    jobs.forEach((job) => {
        rows.push(COLUMNS.map((c) => {
            if (c.key === "jobDate") return formatDateTime(job.jobDate);
            if (c.key === "finalBill") return num(job.finalBill);
            return c.group === "job" ? job[c.key] ?? "" : "";
        }));
        (job.lines || []).forEach((line) => {
            rows.push(COLUMNS.map((c) => {
                if (c.group !== "line") return "";
                if (["rate", "area", "tax", "amount"].includes(c.key)) return num(line[c.key]);
                if (c.key === "qty") return String(line.qty ?? "");
                return line[c.key] ?? "";
            }));
        });
    });

    const foot = COLUMNS.map((c) => (c.key === "finalBill" ? num(grandTotal) : c.key === "jobcardNo" ? "Total" : ""));
    return { columns, rows, foot };
};

const JobReportIndex = () => {
    const [from, setFrom] = useState(() => defaultDateRange().from);
    const [to, setTo] = useState(() => defaultDateRange().to);
    // The client filter: "" is every customer (the report's default). name is kept alongside the
    // id only to label the selector's trigger.
    const [clientId, setClientId] = useState("");
    const [clientName, setClientName] = useState("");
    const [clients, setClients] = useState([]);
    const [report, setReport] = useState(null);
    const [loading, setLoading] = useState(false);

    // The customers this user can pick from - the same name list the Jobs board's own client
    // selector uses, so the two never disagree about who exists.
    useEffect(() => {
        lifecycleBackend
            .lookupClients()
            .then((res) => setClients(res.data || []))
            .catch(() => {});
    }, []);

    const clientOptions = clients.map((c) => ({
        id: c._id,
        name: c.clientFirm || c.clientName,
        search: [c.clientFirm, c.clientName, c.clientPhone].filter(Boolean).join(" ").toLowerCase(),
    }));

    // `live` drops a response the user has already moved past - typing in a date field fires a
    // request per change and they do not always return in order (see GstReportIndex).
    useEffect(() => {
        let live = true;
        setLoading(true);
        const formData = new FormData();
        if (from) formData.set("from", from);
        if (to) formData.set("to", to);
        if (clientId) formData.set("client_id", clientId);
        jobReportBackend
            .get(formData)
            .then((res) => live && setReport(res.data))
            .catch((err) => live && notifyError(err.message || "Couldn't load the job report."))
            .finally(() => live && setLoading(false));
        return () => {
            live = false;
        };
    }, [from, to, clientId]);

    const jobs = report?.jobs || [];
    const grandTotal = report?.grandTotal || 0;

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">Job Report</span>
            </div>
            <div style={{ padding: 20 }}>
                <div className="d-flex align-items-end" style={{ gap: 16, marginBottom: 20, flexWrap: "wrap" }}>
                    <div>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            From
                        </label>
                        <DateField value={from} onChange={(e) => setFrom(e.target.value)} />
                    </div>
                    <div>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            To
                        </label>
                        <DateField value={to} onChange={(e) => setTo(e.target.value)} />
                    </div>
                    <div style={{ minWidth: 220 }}>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            Customer
                        </label>
                        <AssigneeDropdown
                            value={clientName}
                            placeholder="All customers"
                            options={clientOptions}
                            onSelect={(id, name) => {
                                setClientId(id || "");
                                setClientName(name || "");
                            }}
                            fullWidth
                            variant="dashed"
                            menuWidth={320}
                        />
                    </div>
                    <ReportDownloads title="Job Report" disabled={loading || jobs.length === 0} {...buildExport(jobs, grandTotal)} />
                </div>

                {loading && (
                    <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Loading...
                    </p>
                )}

                {!loading && report && (
                    <div style={{ overflowX: "auto" }}>
                        <DataTable>
                            <thead>
                                <tr>
                                    {COLUMNS.map((c) => (
                                        <th key={c.key} scope="col" style={c.align === "right" ? { textAlign: "right" } : undefined}>
                                            {c.label}
                                        </th>
                                    ))}
                                </tr>
                            </thead>
                            <tbody>
                                {jobs.length === 0 ? (
                                    <tr>
                                        <td colSpan={COLUMNS.length} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                            No jobs in this range.
                                        </td>
                                    </tr>
                                ) : (
                                    jobs.map((job, jobIndex) => (
                                        <React.Fragment key={`${job.jobcardNo}-${jobIndex}`}>
                                            {/* The job header: identity + Final Bill, with the line columns blank.
                                                A tinted row so the eye can find where each job begins. */}
                                            <tr style={{ background: "var(--bg-field-on-canvas)" }}>
                                                <td className="cell-mono" style={{ fontWeight: 600 }}>{job.jobcardNo}</td>
                                                <td className="cell-mono">{job.invoiceNo || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                                <td className="cell-mono">{job.poNo}</td>
                                                <td className="cell-mono">{formatDateTime(job.jobDate)}</td>
                                                <td>{job.customerName}</td>
                                                <td className="cell-mono">{job.mobile}</td>
                                                <td>{job.jobName}</td>
                                                <td colSpan={9} />
                                                <td className="cell-mono" style={{ textAlign: "right", fontWeight: 700 }}>{num(job.finalBill)}</td>
                                            </tr>
                                            {(job.lines || []).map((line, lineIndex) => (
                                                <tr key={`${job.jobcardNo}-${jobIndex}-${lineIndex}`}>
                                                    <td colSpan={7} />
                                                    <td>{line.media}</td>
                                                    <td className="cell-mono">{line.hsn}</td>
                                                    <td>{line.description}</td>
                                                    <td className="cell-mono">{line.size}</td>
                                                    <td className="cell-mono" style={{ textAlign: "right" }}>{num(line.rate)}</td>
                                                    <td className="cell-mono" style={{ textAlign: "right" }}>{line.qty}</td>
                                                    <td className="cell-mono" style={{ textAlign: "right" }}>{num(line.area)}</td>
                                                    <td className="cell-mono" style={{ textAlign: "right" }}>{num(line.tax)}</td>
                                                    <td className="cell-mono" style={{ textAlign: "right" }}>{num(line.amount)}</td>
                                                    <td />
                                                </tr>
                                            ))}
                                        </React.Fragment>
                                    ))
                                )}
                                {jobs.length > 0 && (
                                    <tr>
                                        <td colSpan={16} style={{ fontWeight: 700 }}>
                                            Total
                                        </td>
                                        <td className="cell-mono" style={{ textAlign: "right", fontWeight: 700 }}>
                                            {num(grandTotal)}
                                        </td>
                                    </tr>
                                )}
                            </tbody>
                        </DataTable>
                    </div>
                )}
            </div>
        </div>
    );
};

export default JobReportIndex;
