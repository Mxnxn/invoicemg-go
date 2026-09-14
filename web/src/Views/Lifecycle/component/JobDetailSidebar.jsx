import React, { useEffect, useMemo, useState } from "react";
import { X } from "react-feather";
import SlideOverlay from "../../../Common/SlideOverlay/SlideOverlay";
import DataTable from "../../../Common/DataTable/DataTable";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { lifecycleBackend } from "../lifecycle_backend";

// Real per-stage timeline, derived from JobHistory rows logged server-side every time a
// job's queue stage advances. historyRows are newest-first (the API's sort order); this
// walks them oldest-first to reconstruct "stage entered on X, sat for N days".
export const buildQueueHistoryFromLog = (job, historyRows) => {
    const advances = [...historyRows]
        .filter((h) => h.action === "Queue advanced")
        .sort((a, b) => new Date(a.createdAt) - new Date(b.createdAt));

    const stages = [{ stage: "Created", enteredOn: job.createdAt }];
    advances.forEach((h) => {
        const [, to] = h.detail.split(" → ");
        stages.push({ stage: to, enteredOn: h.createdAt });
    });

    return stages.map((s, idx) => {
        const isCurrent = idx === stages.length - 1;
        const enteredDate = new Date(s.enteredOn);
        const nextEntered = isCurrent ? new Date() : new Date(stages[idx + 1].enteredOn);
        const days = Math.max(1, Math.round((nextEntered.getTime() - enteredDate.getTime()) / 86400000));
        return { stage: s.stage, enteredOn: enteredDate.toISOString().slice(0, 10), days, current: isCurrent };
    });
};

// Minutes if under an hour, hours if under a day, days once it crosses 24h.
const formatDuration = (ms) => {
    if (ms === null || ms < 0) return "—";
    const minutes = Math.round(ms / 60000);
    if (minutes < 60) return `${minutes} min`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours} hr`;
    const days = Math.round(hours / 24);
    return `${days} ${days === 1 ? "day" : "days"}`;
};

const DETAIL_FIELDS = [
    { key: "receivedDate", label: "Received Date" },
    { key: "description", label: "Description" },
    { key: "qty", label: "Qty" },
    { key: "rate", label: "Rate", currency: true },
    { key: "total", label: "Total", currency: true },
    { key: "material", label: "Material" },
    { key: "employee", label: "Employee" },
    { key: "vendor", label: "Vendor" },
];

const JobDetailSidebar = ({ job, onClose }) => {
    const [historyRows, setHistoryRows] = useState([]);

    useEffect(() => {
        if (!job) return;
        lifecycleBackend.listHistory({ job_id: job._id }).then((res) => setHistoryRows(res.data));
    }, [job]);

    const queueHistory = useMemo(() => (job ? buildQueueHistoryFromLog(job, historyRows) : []), [job, historyRows]);

    if (!job) return null;

    const displayJob = {
        ...job,
        client: job.client_id?.clientName || "",
        employee: job.employee_id?.name || "",
        vendor: job.vendor_id?.name || "",
    };

    // Entries are newest-first; each row's duration is the time since the change before
    // it (the next item in the array). The oldest entry falls back to the job's creation time.
    const changeHistory = historyRows.map((h, index, arr) => {
        const time = new Date(h.createdAt).getTime();
        const older = arr[index + 1];
        const olderTime = older ? new Date(older.createdAt).getTime() : new Date(job.createdAt).getTime();
        const duration = formatDuration(time - olderTime);
        return {
            id: h._id,
            timestamp: new Date(h.createdAt).toLocaleString(),
            user: h.actorName,
            action: h.action,
            detail: h.detail,
            duration,
        };
    });

    return (
        <SlideOverlay onClose={onClose} width={560}>
            <span className="d-flex mb-3">
                <button type="button" className="slide-overlay-close ml-auto" onClick={onClose} aria-label="Close">
                    <X size={18} />
                </button>
            </span>
            <h6 className="text-uppercase text-muted ls-1 mb-0">{displayJob.challanNumber}</h6>
            <h2
                className="text-default mb-3 geb fs-24"
                style={{ textTransform: "uppercase", letterSpacing: "1px" }}
            >
                {displayJob.client}
            </h2>

            <div
                style={{
                    display: "grid",
                    gridTemplateColumns: "repeat(auto-fit, minmax(140px, 1fr))",
                    gap: 16,
                    marginBottom: 20,
                }}
            >
                {DETAIL_FIELDS.map(({ key, label, currency }) => (
                    <div key={key}>
                        <div className="text-label-caps" style={{ color: "var(--text-tertiary)", marginBottom: 4 }}>
                            {label}
                        </div>
                        <div className={currency ? "text-body-medium cell-mono" : "text-body-medium"}>
                            {currency ? `₹${displayJob[key]}` : displayJob[key] || "—"}
                        </div>
                    </div>
                ))}
            </div>

            <h6 className="text-uppercase text-muted ls-1 mb-2">Queue History</h6>
            <DataTable>
                <thead>
                    <tr>
                        <th scope="col">Queue</th>
                        <th scope="col">Entered On</th>
                        <th scope="col">Days in Queue</th>
                    </tr>
                </thead>
                <tbody>
                    {queueHistory.map((h, index) => (
                        <tr key={index}>
                            <td>
                                <StatusBadge status={h.current ? "pending" : "paid"}>{h.stage}</StatusBadge>
                            </td>
                            <td className="cell-mono">{h.enteredOn}</td>
                            <td className="cell-mono">
                                {h.days} {h.days === 1 ? "day" : "days"}
                                {h.current && " (ongoing)"}
                            </td>
                        </tr>
                    ))}
                </tbody>
            </DataTable>

            <h6 className="text-uppercase text-muted ls-1 mb-2 mt-4">Change History</h6>
            {changeHistory.length === 0 ? (
                <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                    No changes logged yet.
                </p>
            ) : (
                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">User</th>
                            <th scope="col">Change</th>
                            <th scope="col">Detail</th>
                            <th scope="col">Duration</th>
                            <th scope="col">Date/Time</th>

                        </tr>
                    </thead>
                    <tbody>
                        {changeHistory.map((h) => (
                            <tr key={h.id}>
                                <td className="text-body-medium">{h.user}</td>
                                <td>{h.action}</td>
                                <td>{h.detail}</td>
                                <td className="cell-mono">{h.duration}</td>
                                <td className="cell-mono">{h.timestamp}</td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}
        </SlideOverlay>
    );
};

export default JobDetailSidebar;
