import React, { useEffect, useState } from "react";
import { Modal, ModalHeader, ModalBody } from "reactstrap";
import { Move, Check, X, AlertTriangle, Lock, Unlock, Plus, Search, Trash2, Star } from "react-feather";
import { QUEUE_STAGES, QUEUE_LIBRARY } from "./queueConstants";
import { lifecycleBackend } from "../lifecycle_backend";
import { notifySuccess, notifyError } from "../../../global/toast";

// This job-card's own pipeline: which stages it moves through, in what order.
//
// Locked by default. Reordering is a deliberate act on live production - a stray drag while
// reaching for "Complete up to here" would silently re-plan work someone is in the middle of
// doing - so dragging, removing and adding are all behind an Edit toggle, and the changes are
// staged until Saved.
//
// Created and Done never move. The whole lifecycle keys off them: "complete up to here" walks
// the array forwards, the customer alert fires on queue === "Done", and the board buckets jobs
// by position. They cannot be dragged, dropped past, or removed - and the server normalises
// the order anyway (Helpers/QueueOrder.js), because a stale tab must not be able to post a
// pipeline that marks a job finished halfway through itself.
const PINNED = ["Created", "Done"];
const isPinned = (stage) => PINNED.includes(stage);

const RowQueueDialog = ({ job, row, onClose, onChange }) => {
    // Row first, then the job it belongs to, then the built-in stages. A card with no order of
    // its own should follow its job rather than jumping back to the app-wide default.
    const baseList =
        (row.queueOrder && row.queueOrder.length && row.queueOrder) ||
        (job.queueOrder && job.queueOrder.length && job.queueOrder) || [...QUEUE_STAGES];
    const initialQueue = baseList.includes(row.queue) ? [...baseList] : [...baseList, row.queue].filter(Boolean);

    const [queueList, setQueueList] = useState(initialQueue);
    const [savedQueueList, setSavedQueueList] = useState(initialQueue);
    // Whether saving a rearranged pipeline copies it to the job-id's other cards. On by
    // default: they are one order going through one shop, so rearranging the stages here and
    // then repeating the identical drag on each sibling is the work this removes.
    //
    // The ORDER only. Each card keeps the stage it is actually at - "Complete up to here"
    // stays per-row, because two cards can genuinely be at different points and moving them
    // together would overwrite where the work really is.
    const [applyToAll, setApplyToAll] = useState(true);
    // Everything except the card being looked at.
    const siblingCount = (job.rows || []).filter((r) => String(r._id) !== String(row._id)).length;
    const [dragIndex, setDragIndex] = useState(null);
    // The stage whose removal was just refused, held only long enough to shake. Cleared on
    // animationend, and by a timer as well: under prefers-reduced-motion there is no animation
    // and so no animationend, and without the timer the red edge would stay on that stage for
    // good - a refusal of one press turning into a permanent property of the stage.
    const [refusedStage, setRefusedStage] = useState(null);
    useEffect(() => {
        if (!refusedStage) return undefined;
        const t = setTimeout(() => setRefusedStage(null), 600);
        return () => clearTimeout(t);
    }, [refusedStage]);
    const [settingQueue, setSettingQueue] = useState(false);
    const [editing, setEditing] = useState(false);
    const [addQuery, setAddQuery] = useState("");
    const [savingDefault, setSavingDefault] = useState(false);

    const currentIndex = queueList.indexOf(row.queue);
    const doneIndex = queueList.indexOf("Done");

    // Where a dragged stage is allowed to land: after Created, before Done. Without the clamp
    // a drop at either end would push a pin out of position.
    const clampIndex = (index) => {
        const first = queueList.indexOf("Created");
        const last = queueList.indexOf("Done");
        const low = first === -1 ? 0 : first + 1;
        const high = last === -1 ? queueList.length - 1 : last - 1;
        return Math.min(Math.max(index, low), Math.max(low, high));
    };

    const onDragStart = (index) => {
        if (!editing || isPinned(queueList[index])) return;
        setDragIndex(index);
    };
    const onDragOver = (e, index) => {
        if (!editing || dragIndex === null) return;
        e.preventDefault();
        const target = clampIndex(index);
        if (dragIndex === target) return;
        setQueueList((prev) => {
            const next = [...prev];
            const [moved] = next.splice(dragIndex, 1);
            next.splice(target, 0, moved);
            return next;
        });
        setDragIndex(target);
    };
    const onDragEnd = () => setDragIndex(null);

    // Stages not already in this pipeline, matching what has been typed. The typed text itself
    // is offered when it matches nothing, so a stage this company has never used is one press
    // away rather than needing to exist first.
    const trimmedQuery = addQuery.trim();
    const suggestions = QUEUE_LIBRARY.filter(
        (q) => !queueList.some((s) => s.toLowerCase() === q.toLowerCase()) && q.toLowerCase().includes(trimmedQuery.toLowerCase())
    );
    const canCreateCustom = Boolean(trimmedQuery) && !queueList.some((s) => s.toLowerCase() === trimmedQuery.toLowerCase());

    const addStage = (name) => {
        const stage = String(name || "").trim();
        if (!stage || queueList.some((s) => s.toLowerCase() === stage.toLowerCase())) return;
        // Always before Done - a stage after it would never be reached.
        setQueueList((prev) => {
            const next = [...prev];
            const last = next.indexOf("Done");
            next.splice(last === -1 ? next.length : last, 0, stage);
            return next;
        });
        setAddQuery("");
    };

    // Which cards would be written by a save, so the check matches what is actually about to
    // happen: with "apply to all" on that is every card on the job-id, with it off just this
    // one. Checking the whole job either way would refuse removals that are perfectly safe.
    const affectedRows = () =>
        applyToAll ? (job.rows || []) : (job.rows || []).filter((r) => String(r._id) === String(row._id));

    const removeStage = (stage) => {
        if (isPinned(stage)) return;
        // A card may not have the stage it is standing on taken out from under it: position in
        // the pipeline is what "complete up to here" and the board both read, so a card whose
        // stage is missing from its own order indexes at -1 and can never be completed.
        //
        // Refused here as well as on the server (routes/Lifecycle.js) so it is answered at the
        // press rather than after a save that looked like it worked. The server is the
        // guarantee; this is the part a person sees.
        const standing = affectedRows().filter((r) => r.queue === stage);
        if (standing.length) {
            setRefusedStage(stage);
            notifyError(
                standing.length === 1
                    ? `Can't remove ${stage} - one card is on it.`
                    : `Can't remove ${stage} - ${standing.length} cards are on it.`
            );
            return;
        }
        setQueueList((prev) => prev.filter((s) => s !== stage));
    };

    const saveAsDefault = async () => {
        setSavingDefault(true);
        try {
            const formData = new FormData();
            formData.set("queueOrder", JSON.stringify(queueList));
            const res = await lifecycleBackend.setDefaultQueueOrder(formData);
            notifySuccess(res.message || "Saved as the default for new jobs.");
        } catch (error) {
            notifyError(error?.message || "Couldn't save that as the default.");
        } finally {
            setSavingDefault(false);
        }
    };

    const hasUnsavedReorder = queueList.join("|") !== savedQueueList.join("|");
    const confirmReorder = () => {
        lifecycleBackend
            .updateRowQueueOrder({
                job_id: job._id,
                row_id: row._id,
                queueOrder: JSON.stringify(queueList),
                all_rows: applyToAll,
            })
            .then((res) => {
                setSavedQueueList(queueList);
                onChange(res.data);
            });
    };
    const cancelReorder = () => setQueueList(savedQueueList);

    const completeUpToHere = (stage) => {
        if (stage === row.queue || settingQueue) return;
        setSettingQueue(true);
        lifecycleBackend
            .setRowQueue({ job_id: job._id, row_id: row._id, queue: stage })
            .then((res) => onChange(res.data))
            .finally(() => setSettingQueue(false));
    };

    return (
        <Modal isOpen toggle={onClose}>
            <ModalHeader toggle={onClose}>{row.material || row.description || "Row"} — Queue</ModalHeader>
            <ModalBody>
                {/* The gate. Locked, this is a status list you can advance; unlocked, it is a
                    plan you can rearrange. Saying which one it currently is beats leaving the
                    user to discover it by dragging something by accident. */}
                <div className="d-flex align-items-center" style={{ gap: 8, marginBottom: 12 }}>
                    <span className="text-body-small" style={{ color: "var(--text-secondary)", flex: 1 }}>
                        {editing ? "Drag to reorder. Created and Done stay put." : "Locked — press Edit to rearrange the stages."}
                    </span>
                    <button
                        type="button"
                        className="shell-btn shell-btn-sm shell-btn-secondary d-flex align-items-center"
                        style={{ gap: 6 }}
                        aria-pressed={editing}
                        onClick={() => {
                            // Leaving edit mode drops staged changes rather than keeping them
                            // pending invisibly - Save is the only way they take effect.
                            if (editing) setQueueList(savedQueueList);
                            setEditing((on) => !on);
                        }}
                    >
                        {editing ? <Unlock size={13} /> : <Lock size={13} />}
                        {editing ? "Done editing" : "Edit"}
                    </button>
                </div>
                {queueList.map((stage, index) => {
                    const passed = currentIndex >= 0 && index < currentIndex;
                    const active = index === currentIndex;
                    const strandedAfterDone = doneIndex !== -1 && index > doneIndex;
                    return (
                        <div
                            key={stage}
                            className={refusedStage === stage ? "queue-stage-refused" : undefined}
                            onAnimationEnd={() => refusedStage === stage && setRefusedStage(null)}
                            draggable={editing && !isPinned(stage)}
                            onDragStart={() => onDragStart(index)}
                            onDragOver={(e) => onDragOver(e, index)}
                            onDragEnd={onDragEnd}
                            style={{
                                padding: "8px 10px",
                                marginBottom: 6,
                                borderRadius: "var(--radius-control)",
                                border: strandedAfterDone ? "1px solid var(--status-red-text)" : "1px solid var(--border-default)",
                                background: strandedAfterDone
                                    ? "var(--xan-red-bg, rgba(220, 38, 38, 0.08))"
                                    : active
                                    ? "var(--xan-blue-bg)"
                                    : "var(--bg-raised)",
                                cursor: editing && !isPinned(stage) ? "grab" : "default",
                                opacity: dragIndex === index ? 0.5 : 1,
                            }}
                        >
                            <div className="d-flex align-items-center" style={{ gap: 8 }}>
                                {strandedAfterDone ? (
                                    <AlertTriangle size={13} style={{ color: "var(--status-red-text)", flexShrink: 0 }} />
                                ) : isPinned(stage) ? (
                                    // Says "this one is fixed" rather than offering a handle
                                    // that does nothing when pulled.
                                    <Lock size={13} style={{ color: "var(--text-tertiary)", flexShrink: 0 }} />
                                ) : (
                                    <Move
                                        size={13}
                                        style={{ color: editing ? "var(--text-secondary)" : "var(--text-tertiary)", flexShrink: 0 }}
                                    />
                                )}
                                <span
                                    style={{
                                        width: 18,
                                        height: 18,
                                        borderRadius: "50%",
                                        flexShrink: 0,
                                        display: "flex",
                                        alignItems: "center",
                                        justifyContent: "center",
                                        fontSize: 10,
                                        background: strandedAfterDone
                                            ? "var(--status-red-text)"
                                            : passed
                                            ? "var(--xan-emerald)"
                                            : active
                                            ? "var(--xan-blue)"
                                            : "var(--bg-field)",
                                        color: strandedAfterDone || passed || active ? "#fff" : "var(--text-tertiary)",
                                    }}
                                >
                                    {passed && !strandedAfterDone ? <Check size={12} /> : index + 1}
                                </span>
                                <span
                                    style={{
                                        fontSize: 13,
                                        color: strandedAfterDone ? "var(--status-red-text)" : active ? "var(--xan-blue)" : "var(--text-primary)",
                                        fontWeight: active ? 600 : 400,
                                        flex: 1,
                                    }}
                                >
                                    {stage}
                                </span>
                            </div>
                            {editing ? (
                                // Advancing production and re-planning it are different jobs,
                                // so they never share a button. A pin offers nothing here.
                                !isPinned(stage) && (
                                    <button
                                        type="button"
                                        className="shell-btn shell-btn-sm shell-btn-secondary d-flex align-items-center"
                                        style={{ marginTop: 6, width: "100%", gap: 6, justifyContent: "center" }}
                                        onClick={() => removeStage(stage)}
                                    >
                                        <Trash2 size={13} />
                                        Remove stage
                                    </button>
                                )
                            ) : active ? (
                                <div className="text-body-small" style={{ marginTop: 4, color: "var(--xan-blue)" }}>
                                    Current stage
                                </div>
                            ) : (
                                <button
                                    type="button"
                                    className="shell-btn shell-btn-sm shell-btn-secondary"
                                    style={{ marginTop: 6, width: "100%" }}
                                    disabled={settingQueue}
                                    onClick={() => completeUpToHere(stage)}
                                >
                                    Complete up to here
                                </button>
                            )}
                        </div>
                    );
                })}

                {editing && (
                    <div style={{ marginTop: 12 }}>
                        <div className="d-flex align-items-center" style={{ gap: 6, marginBottom: 6 }}>
                            <Search size={13} style={{ color: "var(--icon-muted)", flexShrink: 0 }} />
                            <input
                                className="form-control form-control-alternative nn"
                                value={addQuery}
                                onChange={(e) => setAddQuery(e.target.value)}
                                placeholder="Search or name a new stage"
                                aria-label="Search or name a new stage"
                            />
                        </div>
                        <div className="d-flex" style={{ gap: 6, flexWrap: "wrap" }}>
                            {suggestions.map((q) => (
                                <button
                                    key={q}
                                    type="button"
                                    className="shell-btn shell-btn-sm shell-btn-secondary"
                                    onClick={() => addStage(q)}
                                >
                                    {q}
                                </button>
                            ))}
                            {/* A stage this company has never used is one press away rather
                                than needing to be created somewhere else first. */}
                            {canCreateCustom && (
                                <button
                                    type="button"
                                    className="shell-btn shell-btn-sm shell-btn-primary d-flex align-items-center"
                                    style={{ gap: 6 }}
                                    onClick={() => addStage(trimmedQuery)}
                                >
                                    <Plus size={13} />
                                    Create “{trimmedQuery}”
                                </button>
                            )}
                            {!suggestions.length && !canCreateCustom && (
                                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                    Every stage is already in this pipeline.
                                </span>
                            )}
                        </div>
                    </div>
                )}

                {hasUnsavedReorder && (
                    <div
                        className="d-flex align-items-center"
                        style={{
                            gap: 8,
                            marginTop: 12,
                            padding: "8px 12px",
                            borderRadius: "var(--radius-control)",
                            background: "var(--xan-amber-bg)",
                            border: "1px solid var(--xan-amber)",
                        }}
                    >
                        <span className="text-body-small" style={{ color: "var(--xan-amber)", flex: 1 }}>
                            Queue order changed — not saved yet
                        </span>
                        <button
                            type="button"
                            onClick={cancelReorder}
                            aria-label="Cancel reorder"
                            className="shell-icon-btn"
                            style={{ width: 28, height: 28, color: "var(--status-red-text)" }}
                        >
                            <X size={16} />
                        </button>
                        <button
                            type="button"
                            onClick={confirmReorder}
                            aria-label="Save reorder"
                            className="shell-icon-btn"
                            style={{ width: 28, height: 28, color: "var(--xan-emerald)" }}
                        >
                            <Check size={16} />
                        </button>
                    </div>
                )}

                {/* Applies to jobs raised from now on, never to jobs already in production -
                    rewriting a live pipeline would move work between stages under the hands of
                    whoever is doing it, and there is no undo. Offered only while editing, and
                    the server refuses it for anyone but an admin: it changes how every job this
                    company raises behaves, which is not a per-operator preference. */}
                {editing && (
                    <button
                        type="button"
                        className="shell-btn shell-btn-sm shell-btn-secondary d-flex align-items-center"
                        style={{ marginTop: 10, width: "100%", gap: 6, justifyContent: "center" }}
                        disabled={savingDefault || hasUnsavedReorder}
                        title={
                            hasUnsavedReorder
                                ? "Save this order first, then it can become the default"
                                : "New jobs will start with this pipeline"
                        }
                        onClick={saveAsDefault}
                    >
                        <Star size={13} />
                        {savingDefault ? "Saving…" : "Make this the default for new jobs"}
                    </button>
                )}
                {/* Only shown while a rearrangement is actually pending, and only when there
                    is a sibling to copy it to. Elsewhere it would be a checkbox with nothing
                    to act on, which teaches people not to read the ones that matter. */}
                {siblingCount > 0 && editing && (
                    <label className="row-queue-all">
                        <input
                            type="checkbox"
                            checked={applyToAll}
                            onChange={(e) => setApplyToAll(e.target.checked)}
                        />
                        <span>
                            Use this order for the whole job-id
                            <span className="row-queue-all-hint">
                                {applyToAll
                                    ? `Saving copies these stages to all ${siblingCount + 1} cards. Each keeps the stage it is at.`
                                    : `Saving changes this card's stages only; the other ${siblingCount} keep theirs.`}
                            </span>
                        </span>
                    </label>
                )}
            </ModalBody>
        </Modal>
    );
};

export default RowQueueDialog;
