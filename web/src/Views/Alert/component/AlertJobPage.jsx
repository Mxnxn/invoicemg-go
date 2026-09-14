import React, { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { FileText, CheckCircle, Clock, AlertCircle } from "react-feather";
import { alertBackend } from "../alert_backend";
import ReviewForm from "./ReviewForm";
import { triggerDownload } from "../../BankReport/downloads";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { rowDimensions } from "../../Lifecycle/jobMath";
import "./alertJob.css";
import Blank from "../../../Common/DataTable/Blank";

// The page a customer lands on from the "View job details" button in the WhatsApp alert.
//
// Pre-auth by definition - the visitor has no login and never will - so it renders standalone
// rather than inside AdminLayout, and reads the public /alert endpoint. The PDF is built in
// the browser on demand: the droplet is a 1 GB box already allocating 980 MB across its
// containers, and rendering a document per customer tap is the one thing it cannot afford.

const DONE = "Done";

// Widths total 100 and are fixed here rather than in ReportPDF, which takes them from the
// caller precisely because only the caller knows which column holds prose.
// Status earns a column here as well as on screen: a downloaded sheet that omits what the
// page showed invites "which one is right?".
const PDF_COLUMNS = [
    { label: "Sr", width: "5%" },
    { label: "Material", width: "14%" },
    { label: "Description", width: "24%" },
    { label: "Status", width: "11%" },
    { label: "Qty", width: "7%", align: "right" },
    { label: "Size", width: "13%" },
    { label: "Rate", width: "13%", align: "right" },
    { label: "Amount", width: "13%", align: "right" },
];

// Which of the three status looks a stage gets. Only Done is a finished state; anything
// else is in-flight, and an unknown/custom stage still has to render as something.
const toneFor = (queue) => {
    if (queue === DONE) return "done";
    if (queue === "Ready-to-Pickup") return "ready";
    return "progress";
};

const StatusBadge = ({ queue }) => {
    const tone = toneFor(queue);
    const Icon = tone === "done" ? CheckCircle : Clock;
    return (
        <span className={`alert-badge alert-badge-${tone}`}>
            <Icon size={13} aria-hidden="true" />
            {queue || "In progress"}
        </span>
    );
};

const AlertJobPage = () => {
    const { job_id: jobId, jobcard_id: jobcardId } = useParams();
    const [state, setState] = useState({ status: "loading", data: null, message: "" });
    const [busy, setBusy] = useState(false);

    useEffect(() => {
        let live = true;
        alertBackend
            .jobCard(jobId, jobcardId)
            .then((res) => {
                if (!live) return;
                if (res.code !== 200) return setState({ status: "error", data: null, message: res.message });
                setState({ status: "ready", data: res.data, message: "" });
            })
            // This page is opened by a customer with no support channel and no console, so a
            // network failure has to say something human rather than render an empty card.
            .catch(() => live && setState({ status: "error", data: null, message: "We couldn't load this job. Please check your connection and try again." }));
        return () => {
            live = false;
        };
    }, [jobId, jobcardId]);

    const { data } = state;

    const onDownload = async () => {
        setBusy(true);
        try {
            // @react-pdf has a top-level require() that is invalid in a browser ESM bundle,
            // so it is imported only when a download is actually asked for - the same reason
            // Common/reports/ReportDownloads.jsx defers it.
            const [{ pdf }, { default: ReportPDF }] = await Promise.all([
                import("@react-pdf/renderer"),
                import("../../../Common/reports/ReportPDF"),
            ]);
            const blob = await pdf(
                <ReportPDF
                    title={`Job ${data.challanNumber}`}
                    subtitle={data.receivedDate ? `Received ${getDate(data.receivedDate)}` : ""}
                    user={{ firm: data.company.name, address: data.company.address, gst: data.company.gst }}
                    summary={[
                        { label: "Customer", value: data.client.firm || data.client.name },
                        { label: "Status", value: data.jobQueue || "-" },
                        { label: "Total", value: `₹${RoundOff(data.total)}` },
                    ]}
                    columns={PDF_COLUMNS}
                    rows={data.rows.map((row, index) => [
                        index + 1,
                        row.material || "-",
                        row.description || "-",
                        row.queue || "In progress",
                        row.qty,
                        rowDimensions(row),
                        RoundOff(row.rate),
                        RoundOff(row.total),
                    ])}
                    foot={["", "", "", "", "", "", "Total", RoundOff(data.total)]}
                />
            ).toBlob();
            triggerDownload(blob, `Job-${String(data.challanNumber).replace(/[^\w-]+/g, "-")}.pdf`);
        } finally {
            setBusy(false);
        }
    };

    if (state.status === "loading") {
        return (
            <main className="alert-page">
                <div className="alert-card alert-card-centered" role="status" aria-live="polite">
                    <span className="alert-spinner" aria-hidden="true" />
                    <p className="alert-muted">Loading your job…</p>
                </div>
            </main>
        );
    }

    if (state.status === "error") {
        return (
            <main className="alert-page">
                <div className="alert-card alert-card-centered">
                    <AlertCircle size={26} className="alert-error-icon" aria-hidden="true" />
                    <h1 className="alert-title">This link isn't valid</h1>
                    <p className="alert-muted">{state.message || "The job may have been removed, or the link was copied incompletely."}</p>
                </div>
            </main>
        );
    }

    return (
        <main className="alert-page">
            <div className="alert-card">
                <header className="alert-head">
                    <div>
                        <p className="alert-firm">{data.company.name}</p>
                        {data.company.address ? <p className="alert-muted alert-muted-tight">{data.company.address}</p> : null}
                        {data.company.gst ? <p className="alert-muted alert-muted-tight">GST No: {data.company.gst}</p> : null}
                    </div>
                    <StatusBadge queue={data.jobQueue} />
                </header>

                <h1 className="alert-title">Job {data.challanNumber}</h1>
                <p className="alert-muted">
                    For {data.client.firm || data.client.name}
                    {data.receivedDate ? ` · Received ${getDate(data.receivedDate)}` : ""}
                </p>

                <div className="alert-table-scroll">
                    <table className="alert-table">
                        <thead>
                            <tr>
                                <th scope="col">Sr</th>
                                <th scope="col">Material</th>
                                <th scope="col">Description</th>
                                <th scope="col">Status</th>
                                <th scope="col" className="alert-num">Qty</th>
                                <th scope="col">Size</th>
                                <th scope="col" className="alert-num">Rate</th>
                                <th scope="col" className="alert-num">Amount</th>
                            </tr>
                        </thead>
                        <tbody>
                            {data.rows.map((row, index) => (
                                // The row the link points at is marked, since a multi-row job
                                // otherwise gives no clue which item the message was about.
                                <tr key={row.jobcard_id} className={row.jobcard_id === data.jobcard.jobcard_id ? "is-linked" : undefined}>
                                    <td>{index + 1}</td>
                                    <td>{row.material || <Blank />}</td>
                                    <td>{row.description || <Blank />}</td>
                                    <td>
                                        <StatusBadge queue={row.queue} />
                                    </td>
                                    <td className="alert-num">{row.qty}</td>
                                    <td>{rowDimensions(row) || <span className="xan-cell-na">&mdash;</span>}</td>
                                    <td className="alert-num">{RoundOff(row.rate)}</td>
                                    <td className="alert-num">{RoundOff(row.total)}</td>
                                </tr>
                            ))}
                        </tbody>
                        <tfoot>
                            <tr>
                                <td colSpan="7">Total</td>
                                <td className="alert-num">₹{RoundOff(data.total)}</td>
                            </tr>
                        </tfoot>
                    </table>
                </div>

                <div className="alert-actions">
                    <button type="button" className="alert-download" onClick={onDownload} disabled={busy}>
                        <FileText size={14} aria-hidden="true" />
                        {busy ? "Preparing…" : "Download PDF"}
                    </button>
                    {data.company.phone ? <p className="alert-muted alert-muted-tight">Questions? Call {data.company.phone}</p> : null}
                </div>

                {/* Below the job and the download, because the customer should see what they
                    are rating before being asked to rate it. */}
                <ReviewForm jobId={data.job_id} jobcardId={data.jobcard.jobcard_id} />
            </div>
        </main>
    );
};

export default AlertJobPage;
