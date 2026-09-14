import React, { useEffect, useMemo, useState } from "react";
import { Modal, ModalBody } from "reactstrap";
import { X, CheckCircle, Lock, Unlock, MessageSquare } from "react-feather";
import { useNavigate } from "react-router-dom";
import { QUEUE_LIBRARY, buildQueueHistoryFromLog } from "./queueConstants";
import { lifecycleBackend } from "../lifecycle_backend";
import { rowTotal, rowDimensions } from "../jobMath";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import "../lifecycle.css";
import DataTable from "../../../Common/DataTable/DataTable";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import AssigneeDropdown from "./AssigneeDropdown";
import ProgressDropdown from "./ProgressDropdown";
import RowQueueDialog from "./RowQueueDialog";
import JobAlertActions from "./JobAlertActions";
import { formatAmount } from "../../../Common/money";
import Blank from "../../../Common/DataTable/Blank";

const MONTH_NAMES_FULL = [
    "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December",
];

// "08 August 2026, 8:54:54 PM"
const formatNoteTimestamp = (date) => {
    const d = new Date(date);
    const day = String(d.getDate()).padStart(2, "0");
    const month = MONTH_NAMES_FULL[d.getMonth()];
    const year = d.getFullYear();
    let hours = d.getHours();
    const ampm = hours >= 12 ? "PM" : "AM";
    hours = hours % 12 || 12;
    const minutes = String(d.getMinutes()).padStart(2, "0");
    const seconds = String(d.getSeconds()).padStart(2, "0");
    return `${day} ${month} ${year}, ${hours}:${minutes}:${seconds} ${ampm}`;
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
    { key: "total", label: "Total", currency: true },
    { key: "advance", label: "Advance", currency: true },
];

// Unified job detail view - replaces the old split between JobDetailSidebar (rows/history)
// and QueueDetailModal (notes/queue-reorder) with one tabbed modal: Entries (rows + header
// info + audit log), Notes, and Queues (compact cards, each with a "Complete up to here"
// jump button, plus the existing drag-reorder and add-custom-stage affordances).
const JobDetailModal = ({ job, onClose, onChange }) => {
    // Every row Done means the work is finished - worth saying, and nothing more. It used to
    // gate on rows not yet converted, because the user had to press a button to file them as
    // Entries; invoicing does that itself now (Helpers/ConvertJobRows.js), so the condition
    // is simply "all done" and the message carries no action.
    const rows = job.rows || [];
    const allRowsDone = rows.length > 0 && rows.every((r) => r.queue === "Done");

    const navigate = useNavigate();
    const [historyRows, setHistoryRows] = useState([]);
    const [notes, setNotes] = useState([]);
    const [noteText, setNoteText] = useState("");
    const [lockBusy, setLockBusy] = useState(false);
    const [notesOpen, setNotesOpen] = useState(false);

    const onToggleLock = async () => {
        setLockBusy(true);
        try {
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("unlocked", String(!job.lock?.unlocked));
            const res = await lifecycleBackend.unlockJob(formData);
            // The server owns what the new lock state permits, so the updated job it returns
            // is what the board and this modal re-render from.
            onChange?.(res.data);
        } catch (error) {
            // The global interceptor raises the toast.
        } finally {
            setLockBusy(false);
        }
    };

    const [editingNoteId, setEditingNoteId] = useState(null);
    const [editingText, setEditingText] = useState("");
    const [dragIndex, setDragIndex] = useState(null);
    const [settingQueue, setSettingQueue] = useState(false);
    const [people, setPeople] = useState([]);
    const [queueDialogRowId, setQueueDialogRowId] = useState(null);

    const [addQuery, setAddQuery] = useState("");
    const [addOpen, setAddOpen] = useState(false);
    useEffect(() => {
        lifecycleBackend.listNotes({ job_id: job._id }).then((res) => setNotes(res.data));
        lifecycleBackend.listHistory({ job_id: job._id }).then((res) => setHistoryRows(res.data));
        lifecycleBackend.lookupPeople().then((res) => setPeople(res.data));
    }, [job._id]);

    const employeeOptions = people.filter((p) => p.type === "Employee").map((p) => ({ id: p._id, name: p.name }));

    const onAssignRow = (row, employeeId) => {
        lifecycleBackend.assignRow({ job_id: job._id, row_id: row._id, employee_id: employeeId || "" }).then((res) => onChange(res.data));
    };

    const onRowProgressChange = (row, progress) => {
        lifecycleBackend.setRowProgress({ job_id: job._id, row_id: row._id, progress }).then((res) => onChange(res.data));
    };

    const queueHistory = useMemo(() => buildQueueHistoryFromLog(job, historyRows), [job, historyRows]);
    const durationByStage = useMemo(() => new Map(queueHistory.map((h) => [h.stage, h])), [queueHistory]);

    // A job's actual current stage might not be in queueOrder/QUEUE_STAGES if it's sitting
    // in a legacy/custom stage from before the default pipeline was narrowed - append it
    // rather than silently hide the job's real position.
    const baseQueueList = job.queueOrder && job.queueOrder.length ? job.queueOrder : [...QUEUE_LIBRARY.slice(0, 4)];
    const initialQueue = baseQueueList.includes(job.queue) ? baseQueueList : [...baseQueueList, job.queue].filter(Boolean);
    const [queueList, setQueueList] = useState(initialQueue);
    const [savedQueueList, setSavedQueueList] = useState(initialQueue);

    useEffect(() => {
        setQueueList(initialQueue);
        setSavedQueueList(initialQueue);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [job._id, job.queueOrder, job.queue]);

    const currentIndex = queueList.indexOf(job.queue);
    const doneIndex = queueList.indexOf("Done");

    const onDragStart = (index) => setDragIndex(index);
    const onDragOver = (e, index) => {
        e.preventDefault();
        if (dragIndex === null || dragIndex === index) return;
        setQueueList((prev) => {
            const next = [...prev];
            const [moved] = next.splice(dragIndex, 1);
            next.splice(index, 0, moved);
            return next;
        });
        setDragIndex(index);
    };
    const onDragEnd = () => setDragIndex(null);

    const hasUnsavedReorder = queueList.join("|") !== savedQueueList.join("|");
    const confirmReorder = () => {
        lifecycleBackend.updateQueue({ job_id: job._id, queueOrder: JSON.stringify(queueList) }).then((res) => {
            setSavedQueueList(queueList);
            onChange(res.data);
        });
    };
    const cancelReorder = () => setQueueList(savedQueueList);

    const completeUpToHere = (stage) => {
        if (stage === job.queue || settingQueue) return;
        setSettingQueue(true);
        lifecycleBackend
            .setQueue({ job_id: job._id, queue: stage })
            .then((res) => onChange(res.data))
            .finally(() => setSettingQueue(false));
    };

    const filteredOptions = QUEUE_LIBRARY.filter(
        (q) => !queueList.includes(q) && q.toLowerCase().includes(addQuery.toLowerCase())
    );
    const canCreateCustom = addQuery.trim() && !queueList.some((q) => q.toLowerCase() === addQuery.trim().toLowerCase());

    const addQueueStage = (name) => {
        const trimmed = name.trim();
        if (!trimmed || queueList.includes(trimmed)) return;
        setQueueList((prev) => [...prev, trimmed]);
        setAddQuery("");
        setAddOpen(false);
    };

    const saveNote = () => {
        if (!noteText.trim()) return;
        lifecycleBackend.createNote({ job_id: job._id, text: noteText.trim() }).then((res) => {
            setNotes((prev) => [res.data, ...prev]);
            setNoteText("");
        });
    };

    const startEditNote = (note) => {
        setEditingNoteId(note._id);
        setEditingText(note.text);
    };

    const saveEditNote = () => {
        if (!editingText.trim()) return;
        lifecycleBackend.updateNote({ note_id: editingNoteId, text: editingText.trim() }).then((res) => {
            setNotes((prev) => prev.map((n) => (n._id === res.data._id ? res.data : n)));
            setEditingNoteId(null);
            setEditingText("");
        });
    };

    const displayJob = { ...job };
    // Batch Receive (auto or manual) increments job.advance toward job.total - once it
    // covers the job, every row on it counts as paid, regardless of individual conversion
    // state (advance can land before a row's even been converted to an Entry).
    const isPaid = job.total > 0 && job.advance >= job.total;

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
        <Modal isOpen toggle={onClose} size="xl" style={{ maxWidth: "90vw", width: "90vw" }}>
            <div className="shell-card-header" style={{ borderBottom: "1px solid var(--border-default)", alignItems: "flex-start" }}>
                <div>
                    <span className="d-inline-flex align-items-center" style={{ gap: 8, flexWrap: "wrap" }}>
                        <span className="text-heading-brand">{job.challanNumber}</span>
                        {/* A state, not an announcement. It belongs beside the job-id where the
                            job is identified, and it stays as long as it is true - the banner
                            this replaces said the same thing once and then left, so anyone
                            opening the job later had no way to see it. */}
                        {allRowsDone && (
                            <span className="job-finished-tag">
                                <CheckCircle size={12} aria-hidden="true" />
                                Finished
                            </span>
                        )}
                    </span>
                    <div
                        className="d-flex align-items-center text-body-small"
                        style={{ gap: 6, marginTop: 4, marginBottom: 12, color: "var(--text-tertiary)", flexWrap: "wrap" }}
                    >
                        {job.client_id && (
                            <button type="button" className="lifecycle-link" onClick={() => navigate(`/customer/${job.client_id._id}`)}>
                                {job.client_id.clientName}
                                {job.client_id.clientFirm ? ` — ${job.client_id.clientFirm}` : ""}
                            </button>
                        )}
                        {job.client_id?.clientPhone && <span>• {job.client_id.clientPhone}</span>}
                        <span>
                            • Total: <span className="cell-mono">₹{formatAmount(job.total)}</span>
                        </span>
                    </div>

                    {/* An invoiced job is frozen. Unlocking re-opens its values and its
                        production stages so the invoice can be corrected through it; the one
                        thing it never allows is deleting the job, because restructuring an
                        invoice is done by deleting the INVOICE.
                        job.lock is computed server-side (Helpers/JobLock.js) so this control
                        and the routes cannot disagree about what is permitted. */}
                    {job.lock?.invoiced && (
                        <div className="job-lock-bar">
                            {job.lock.unlocked ? <Unlock size={14} /> : <Lock size={14} />}
                            <span className="text-body-small">
                                {job.lock.unlocked
                                    ? "Unlocked — values and production stages can be edited."
                                    : "Invoiced and locked. Unlock to correct it."}
                            </span>
                            <button
                                type="button"
                                className="shell-btn shell-btn-secondary job-lock-btn"
                                disabled={lockBusy}
                                onClick={onToggleLock}
                            >
                                {lockBusy ? "…" : job.lock.unlocked ? "Lock" : "Unlock"}
                            </button>
                        </div>
                    )}
                </div>
                <button type="button" className="slide-overlay-close" onClick={onClose} aria-label="Close">
                    <X size={18} />
                </button>
                {/* Directly under the close control, on the same edge as the panel it opens -
                    an icon, because the count tells you whether there is anything in there and
                    the label did not. */}
                <button
                    type="button"
                    className={["job-notes-toggle", notesOpen ? "is-open" : ""].filter(Boolean).join(" ")}
                    onClick={() => setNotesOpen((open) => !open)}
                    aria-expanded={notesOpen}
                    aria-label={notesOpen ? "Close notes" : "Open notes"}
                    title={notesOpen ? "Close notes" : "Notes"}
                >
                    <MessageSquare size={16} />
                    {notes.length > 0 && <span className="job-notes-count">{notes.length}</span>}
                </button>
            </div>
            <ModalBody className="job-detail-body" style={{ padding: 20, minHeight: "70vh", maxHeight: "75vh", overflowY: "auto" }}>
                <div className="job-detail-split">
                    <div className="job-detail-split-main">
                <>
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
                                        {/* RoundOff, not the raw field - an existing job still
                                            carries the unrounded float it was saved with. */}
                                        {currency ? `₹${RoundOff(displayJob[key])}` : displayJob[key] || "—"}
                                    </div>
                                </div>
                            ))}
                        </div>

                        <div className="job-rows-head">
                            <h6 className="text-uppercase text-muted ls-1 mb-0">Rows</h6>
                            {/* Delivery status and the job's own actions, grouped and ordered
                                by JobAlertActions - the board's head renders the same thing, so
                                the two views cannot drift apart on what a job-id can be told
                                to do. */}
                            <JobAlertActions job={job} onChange={onChange} />
                        </div>
                        <DataTable>
                            <thead>
                                <tr>
                                    <th scope="col">Material</th>
                                    <th scope="col">Description</th>
                                    <th scope="col">Qty</th>
                                    <th scope="col">Rate</th>
                                    <th scope="col">Total</th>
                                    <th scope="col">Status</th>
                                    <th scope="col">Employee</th>
                                    <th scope="col">Queue</th>
                                    <th scope="col">Progress</th>
                                </tr>
                            </thead>
                            <tbody>
                                {(job.rows || []).map((row, index) => (
                                    <tr key={row._id || index}>
                                        <td>{row.material || <Blank />}</td>
                                        {/* Size sits under the description rather than in its own column: it
                                            qualifies what the row IS, and the table is already nine columns wide. */}
                                        <td>
                                            <div>{row.description || <Blank />}</div>
                                            <div className="job-row-dimensions">{rowDimensions(row)}</div>
                                        </td>
                                        <td className="cell-mono">{row.qty}</td>
                                        <td className="cell-mono">₹{row.rate}</td>
                                        <td className="cell-mono">₹{RoundOff(rowTotal(row))}</td>
                                        <td>
                                            {/* "overdue" is the rose variant - Due is money owed, not a neutral workflow state. */}
                                            <StatusBadge status={isPaid ? "paid" : row.entry_id ? "pending" : "overdue"}>
                                                {isPaid ? "Paid" : row.entry_id ? "Awaiting Payment" : "Due"}
                                            </StatusBadge>
                                        </td>
                                        <td>
                                            <AssigneeDropdown
                                                value={row.employee_id?.name || ""}
                                                placeholder="Employee"
                                                options={employeeOptions}
                                                onSelect={(id) => onAssignRow(row, id)}
                                                disabled={row.queue === "Done"}
                                            />
                                        </td>
                                        <td>
                                            <button
                                                type="button"
                                                onClick={() => setQueueDialogRowId(row._id)}
                                                style={{ border: "none", background: "transparent", padding: 0, cursor: "pointer" }}
                                            >
                                                <StatusBadge status="pending">{row.queue || "Created"}</StatusBadge>
                                            </button>
                                        </td>
                                        <td>
                                            <ProgressDropdown
                                                value={row.progress || "Assign"}
                                                hasAssignee={!!row.employee_id}
                                                onSelect={(next) => onRowProgressChange(row, next)}
                                                disabled={row.queue === "Done"}
                                            />
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
                            <div style={{ maxHeight: 260, overflowY: "auto" }}>
                                <DataTable className="xan-table-static">
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
                            </div>
                        )}
                    </>
                    </div>

                {/* Notes take their own column and narrow the rows, rather than covering
                    them. They used to slide over the top, which meant the only way to read the
                    rows you were writing a note about was to close the note panel - and taking
                    a note is not a separate task from looking at the job.

                    The toggle still carries the count, so there is no need to open it to find
                    out whether anything is there. */}
                {notesOpen && (
                    <aside className="job-notes-panel">
                        <div className="job-notes-panel-head">
                            <h6 className="text-uppercase text-muted ls-1 mb-0">Notes</h6>
                        </div>
                    <div>
                        <div style={{ marginBottom: 16 }}>
                            {notes.map((n) => (
                                <div key={n._id} style={{ marginBottom: 14, paddingBottom: 14, borderBottom: "1px solid var(--border-default)" }}>
                                    <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
                                        <span className="text-body-medium">{n.authorName}</span>
                                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                            {formatNoteTimestamp(n.createdAt)}
                                        </span>
                                        {n.canEdit && editingNoteId !== n._id && (
                                            <button
                                                type="button"
                                                className="shell-btn shell-btn-sm shell-btn-secondary"
                                                style={{ marginLeft: "auto" }}
                                                onClick={() => startEditNote(n)}
                                            >
                                                Edit
                                            </button>
                                        )}
                                    </div>
                                    {editingNoteId === n._id ? (
                                        <div style={{ marginTop: 4 }}>
                                            <textarea
                                                value={editingText}
                                                onChange={(e) => setEditingText(e.target.value)}
                                                rows={2}
                                                style={{
                                                    width: "100%",
                                                    borderRadius: "var(--radius-control)",
                                                    border: "1px solid var(--border-default)",
                                                    background: "var(--bg-field)",
                                                    color: "var(--text-primary)",
                                                    padding: 8,
                                                    fontFamily: "inherit",
                                                }}
                                            />
                                            <div className="d-flex justify-content-end" style={{ gap: 6, marginTop: 4 }}>
                                                <button
                                                    type="button"
                                                    className="shell-btn shell-btn-sm shell-btn-secondary"
                                                    onClick={() => setEditingNoteId(null)}
                                                >
                                                    Cancel
                                                </button>
                                                <button type="button" className="shell-btn shell-btn-sm shell-btn-primary" onClick={saveEditNote}>
                                                    Save
                                                </button>
                                            </div>
                                        </div>
                                    ) : (
                                        <div className="text-body-regular" style={{ marginTop: 4 }}>
                                            {n.text}
                                        </div>
                                    )}
                                </div>
                            ))}
                            {notes.length === 0 && (
                                <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    No notes yet.
                                </p>
                            )}
                        </div>
                        <div>
                            <textarea
                                value={noteText}
                                onChange={(e) => setNoteText(e.target.value)}
                                placeholder="Add a note..."
                                rows={3}
                                style={{
                                    width: "100%",
                                    borderRadius: "var(--radius-control)",
                                    border: "1px solid var(--border-default)",
                                    background: "var(--bg-field)",
                                    color: "var(--text-primary)",
                                    padding: 10,
                                    fontFamily: "inherit",
                                    resize: "vertical",
                                }}
                            />
                            <div className="d-flex justify-content-end" style={{ marginTop: 8 }}>
                                <button type="button" className="shell-btn shell-btn-sm shell-btn-primary" onClick={saveNote}>
                                    Save Note
                                </button>
                            </div>
                        </div>
                    </div>
                    </aside>
                )}
                </div>
            </ModalBody>
            {queueDialogRowId &&
                (() => {
                    const dialogRow = (job.rows || []).find((r) => r._id === queueDialogRowId);
                    if (!dialogRow) return null;
                    return (
                        <RowQueueDialog
                            job={job}
                            row={dialogRow}
                            onClose={() => setQueueDialogRowId(null)}
                            onChange={onChange}
                        />
                    );
                })()}
        </Modal>
    );
};

export default JobDetailModal;
