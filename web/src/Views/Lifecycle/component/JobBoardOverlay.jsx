import React, { useEffect, useState } from "react";
import { X, Plus, Lock, Unlock } from "react-feather";

import { boardColumns, canDrop, cardSize, LAST_STAGE } from "../jobBoard";
import { jobGrandTotal } from "../jobMath";
import { lifecycleBackend } from "../lifecycle_backend";
import { notifyError } from "../../../global/toast";
import JobCardPanel from "./JobCardPanel";
import NewCardPanel from "./NewCardPanel";
import JobAlertActions from "./JobAlertActions";
import QuickCreateEmployeeModal from "./QuickCreateEmployeeModal";
import useFlipPanel from "../../../Common/useFlipPanel";
import AssigneeDropdown from "./AssigneeDropdown";
import StatusBadge from "../../../Common/DataTable/StatusBadge";
import { can } from "../../../Common/access";
import "./jobBoard.css";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

// An admin session, decided exactly the way Common/access.js decides it: anything that is not
// "employee" is an admin, INCLUDING an absent role - which is what a fresh admin login leaves.
// Written the same way round rather than testing for "admin", because a superadmin session
// stores "superadmin" and an absent role stores nothing, and both must count.
const isAdminSession = () => (window.localStorage.getItem("role") || "admin") !== "employee";


export default function JobBoardOverlay({ job, rect, onClose, onChanged }) {
    // The cards this board is showing, held locally so a drag can move one immediately and put
    // it back if the server refuses. A card that appears to move and then silently returns on
    // the next refresh is worse than one that never moved.
    const [rows, setRows] = useState(job?.rows || []);
    // The transform currently applied. Starts over the card, is cleared on the next frame to
    // play the expansion, and is put back to play it in reverse. Empty until the panel has been
    // measured, which is one frame - the CSS keeps it invisible until then.
    const { panelRef, transform, phase, close } = useFlipPanel({ rect, onClose });
    const [over, setOver] = useState(null);
    const [dragId, setDragId] = useState(null);
    // The card being edited, with the rectangle of the chip it was opened from - its panel
    // grows out of that chip the same way the board grew out of its card.
    const [editing, setEditing] = useState(null);
    const [busy, setBusy] = useState(false);
    // Who a card can be given to. Without this the assignee rule is unfixable from the board:
    // every refusal says "assign someone" and there is nowhere on the board to do it.
    const [people, setPeople] = useState([]);
    // The rectangle of the + that asked for a card, while the new-card panel is open. Same
    // shape as `editing`: the panel grows out of the control that opened it.
    const [addingRect, setAddingRect] = useState(null);
    // { name, card } while the new-employee form is open - the card so the person created can
    // be put straight onto the card that needed them.
    const [creatingFor, setCreatingFor] = useState(null);
    // Which column is being dragged, and which stage is being renamed.
    const [dragStage, setDragStage] = useState(null);
    const [renaming, setRenaming] = useState(null);
    const [renameTo, setRenameTo] = useState("");

    useEffect(() => setRows(job?.rows || []), [job]);

    useEffect(() => {
        let live = true;
        lifecycleBackend
            .lookupPeople()
            .then((res) => live && setPeople(res.data || []))
            // The interceptor toasts the failure; the picker simply has nobody in it, which
            // the empty state below says out loud rather than looking broken.
            .catch(() => {});
        return () => {
            live = false;
        };
    }, []);



    if (!job) return null;

    const columns = boardColumns({ ...job, rows });
    const clientName = job.client_id?.clientFirm || job.client_id?.clientName || "";
    // The lock the server already computes, so a control and the route it calls cannot
    // disagree about whether something is allowed. Not also gated on every card being Done: a
    // job can gain a card after the work is finished but before it is billed, and once it IS
    // billed both of these disappear together.
    const employeeOptions = people
        .filter((person) => person.type === "Employee")
        .map((person) => ({ id: person._id, name: person.name }));

    // Adding someone without leaving the board.
    //
    // The rule refuses a move until a card has somebody on it, so a shop with no employee
    // records yet cannot move anything at all - the picker opens, is empty, and there is
    // nowhere to go from there. Sending people to Configure > People to come back and start
    // again is the kind of dead end that makes a rule feel like a bug rather than a check.
    //
    // Name only. A Person needs a name and a type, and everything else about an employee -
    // phone, permissions, a portal password - is a different job done on the People screen,
    // not something to ask for mid-drag.
    // Opens the form rather than creating outright. An employee needs an email and a password
    // to be a person who can sign in, and inventing an account with neither produces someone
    // who exists on a board and nowhere else.
    const createEmployee = (name, card) => setCreatingFor({ name: String(name || "").trim(), card });

    const employeeCreated = async (created) => {
        const card = creatingFor?.card;
        setCreatingFor(null);
        if (!created?._id) return;
        setPeople((prev) => [...prev, created]);
        // Created in order to be assigned - so assign them, rather than making the press that
        // created them a press that did nothing visible.
        if (card) await assign(card, created._id);
    };

    // Assigning is its own action on its own route - it is not part of editing a card's
    // values, and the server keeps progress in step with it (Assign -> In Progress).
    const assign = async (card, employeeId) => {
        try {
            const res = await lifecycleBackend.assignRow({
                job_id: job._id,
                row_id: card._id,
                employee_id: employeeId || "",
            });
            setRows(res.data?.rows || rows);
            onChanged?.(res.data);
        } catch (error) {
            // The interceptor has already said what went wrong.
        }
    };

    // The pipeline as it stands, without any stray columns - those are not stages of this job
    // and must not be written back into its order.
    const pipelineStages = () => columns.filter((c) => !c.offPipeline).map((c) => c.stage);

    const savePipeline = async (next) => {
        setBusy(true);
        try {
            const res = await lifecycleBackend.updateRowQueueOrder({
                job_id: job._id,
                row_id: (job.rows || [])[0]?._id,
                queueOrder: JSON.stringify(next),
                all_rows: true,
            });
            onChanged?.(res.data);
        } catch (error) {
            // The interceptor has already said what went wrong.
        } finally {
            setBusy(false);
        }
    };

    // Reordering columns reorders the pipeline itself - the board is not a view with its own
    // arrangement, it IS the order work moves through. Created and Done are pinned by the
    // server (Helpers/QueueOrder.js) whatever is sent, so dropping one past the ends is
    // corrected rather than refused.
    const moveColumn = (fromStage, toStage) => {
        if (!fromStage || fromStage === toStage) return;
        const next = pipelineStages();
        const from = next.indexOf(fromStage);
        const to = next.indexOf(toStage);
        if (from === -1 || to === -1) return;
        next.splice(to, 0, next.splice(from, 1)[0]);
        savePipeline(next);
    };

    // Renaming a stage has to carry its cards with it. A stage is identified by its NAME - a
    // card sits at "Printing", not at index 1 - so renaming the column and leaving the cards
    // behind would strand every one of them at a stage the pipeline no longer contains, which
    // is exactly the off-pipeline case this board draws in amber.
    //
    // Two writes, and the cards move FIRST: if the second fails the cards are at a stage still
    // in the order, which is survivable. The other way round leaves them stranded.
    const commitRename = async (from) => {
        const to = renameTo.trim();
        setRenaming(null);
        if (!to || to === from) return;
        if (pipelineStages().includes(to)) {
            notifyError(`This job already has a stage called ${to}.`);
            return;
        }

        setBusy(true);
        try {
            const onStage = rows.filter((r) => r.queue === from);
            for (const card of onStage) {
                const formData = new FormData();
                formData.set("job_id", job._id);
                formData.set("row_id", card._id);
                formData.set("queue", to);
                await lifecycleBackend.setRowQueue(formData);
            }
            await savePipeline(pipelineStages().map((stage) => (stage === from ? to : stage)));
        } catch (error) {
            // The interceptor has already said what went wrong.
        } finally {
            setBusy(false);
        }
    };

    const [lockBusy, setLockBusy] = useState(false);

    // Unlocking an invoiced job-id so its values can be corrected. The server owns what that
    // permits (Helpers/JobLock.js) and returns the updated job, so this never decides for
    // itself what came back open.
    const toggleLock = async () => {
        if (lockBusy) return;
        setLockBusy(true);
        try {
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("unlocked", String(!job.lock?.unlocked));
            const res = await lifecycleBackend.unlockJob(formData);
            onChanged?.(res.data);
        } catch (error) {
            // The interceptor has already said what went wrong.
        } finally {
            setLockBusy(false);
        }
    };

    // Stages holding cards that the job's own pipeline does not list, counted by cards rather
    // than by column - "1 card is somewhere unexpected" is the fact; how many columns that
    // took is not.
    const strays = columns.filter((c) => c.offPipeline && c.cards.length).map((c) => c.stage);

    const canEditValues = job.lock?.canEditValues !== false;
    const canEditQueue = job.lock?.canEditQueue !== false;

    // A new card starts where work starts. The server's normalizeRow returns no queue, so a new
    // row takes the schema default - which is the first stage. A + on every column would need a
    // second call to move the card after creating it, leaving a half-made card behind whenever
    // that second call failed, and would let the board claim work happened that did not.
    //
    // It asks before it writes. It used to append a row called "New card" with no product, no
    // rate and no tax - a real line on a real job, contributing nothing to its total, to be
    // found and corrected later by whoever noticed. A card is a thing someone is charged for,
    // so what it is gets asked at the moment it is made.
    const addCard = (e) => setAddingRect(e.currentTarget.getBoundingClientRect());

    // Removing a stage is deliberately NOT offered here. It lives in RowQueueDialog behind an
    // explicit Edit toggle, guarded against removing a stage a card is standing on - and a
    // hover-revealed delete on a board column is exactly how that guard gets exercised by
    // accident.
    const addStage = async () => {
        const name = window.prompt("Name the new stage");
        if (!name || !name.trim()) return;
        setBusy(true);
        try {
            // Inserted before the last stage, never after it: the pipeline ends where it ends,
            // and a stage after Done is a stage nothing can reach.
            const pipeline = columns.filter((c) => !c.offPipeline).map((c) => c.stage);
            pipeline.splice(Math.max(pipeline.length - 1, 0), 0, name.trim());
            const res = await lifecycleBackend.updateRowQueueOrder({
                job_id: job._id,
                row_id: (job.rows || [])[0]?._id,
                queueOrder: JSON.stringify(pipeline),
                all_rows: true,
            });
            onChanged?.(res.data);
        } catch (error) {
            // The interceptor has already said what went wrong.
        } finally {
            setBusy(false);
        }
    };

    const drop = async (stage) => {
        setOver(null);
        const moving = rows.find((r) => String(r._id) === String(dragId));
        setDragId(null);
        if (!moving) return;
        // Belt and braces with draggable={canEditQueue}: a drag can also begin from a file or
        // another window, and the API would refuse this anyway.
        if (!canEditQueue) return;

        // Admins are not asked for an assignee - see canDrop. Read from the session's own
        // role rather than from a permission, because this is about who you ARE, not what you
        // have been granted.
        const verdict = canDrop(moving, stage, columns, { isAdmin: isAdminSession() });
        if (!verdict.ok) {
            // An empty reason means "nothing to do" - dropped where it already was - which
            // deserves no message. A silent refusal of anything else reads as a broken drag.
            if (verdict.reason) notifyError(verdict.reason);
            return;
        }

        const before = moving.queue;
        setRows((prev) => prev.map((r) => (r._id === moving._id ? { ...r, queue: stage } : r)));
        try {
            const formData = new FormData();
            formData.set("job_id", job._id);
            formData.set("row_id", moving._id);
            formData.set("queue", stage);
            const res = await lifecycleBackend.setRowQueue(formData);
            onChanged?.(res.data);
        } catch (error) {
            // The interceptor has already said what went wrong; this only undoes the move.
            setRows((prev) => prev.map((r) => (r._id === moving._id ? { ...r, queue: before } : r)));
        }
    };

    return (
        <>
        {/* Nothing behind the board is reachable while it is open. Without it the page
            underneath still takes clicks - you could start editing an invoice through a board
            covering it. */}
        <div className={`job-board-scrim${phase === "opening" ? " is-open" : ""}`} onClick={close} aria-hidden="true" />
        <div
            ref={panelRef}
            className={`job-board-overlay is-${phase}`}
            // The board sizes itself to the columns it built. Any fixed width is a guess about
            // how many stages a job has, and it is wrong for the first job that has more.
            style={{ transform, "--board-cols": columns.length }}
            role="dialog"
            aria-label={`Board for ${job.challanNumber}`}
        >
            <div className="job-board-overlay-head">
                <span className="job-board-overlay-meta">
                    <span className="text-heading-brand">{job.challanNumber}</span>
                    {/* Whose job it is. A board full of stages says what is happening and never
                        who it is for, which is the first thing anyone asks about a job-id. */}
                    {clientName && <span className="text-body-small">{clientName}</span>}
                    {/* What the board cannot show by arranging cards: what it is worth, and
                        whether it has already been billed - which is also why nothing on it
                        can be dragged when it has. */}
                    <span className="text-body-small">{money(job.total ?? jobGrandTotal(rows))}</span>
                    <StatusBadge status={job.lock?.invoiced ? "neutral" : "green"}>
                        {job.lock?.invoiced ? "Invoiced" : "Open"}
                    </StatusBadge>
                    {/* A card standing at a stage this job-id's pipeline does not contain.
                        boardColumns never drops it - it opens a marked column at the far
                        right - but a column at the far right is exactly what a narrow board
                        scrolls out of sight, which is how a job-id came to look empty. Naming
                        the stage here means the board says where its cards are even when it
                        cannot show them all at once. */}
                    {strays.length > 0 && (
                        <span className="text-body-small job-board-stray-note">
                            {strays.length === 1
                                ? `1 card is at ${strays[0]}, which is not a stage on this job.`
                                : `${strays.length} cards are at stages not on this job: ${strays.join(", ")}.`}
                        </span>
                    )}
                    {/* Said here rather than discovered when a card refuses to move, and
                        acted on here too. A billed job-id is frozen until someone lifts
                        the lock deliberately; lifting it re-opens both its values and its
                        stages, so the board behaves normally again and the wording does not
                        have to carve out an exception.
                        The same route the table's detail modal uses, so the two controls
                        cannot disagree about what unlocking means. */}
                    {job.lock?.invoiced && (
                        <span className="job-board-lock">
                            {job.lock.unlocked ? <Unlock size={13} /> : <Lock size={13} />}
                            <span className="text-body-small">
                                {job.lock.unlocked
                                    ? "Unlocked — cards can be edited and moved."
                                    : "Billed — unlock to edit or move its cards."}
                            </span>
                            <button
                                type="button"
                                className="shell-btn shell-btn-secondary shell-btn-sm"
                                disabled={lockBusy}
                                onClick={toggleLock}
                            >
                                {lockBusy ? "…" : job.lock.unlocked ? "Lock" : "Unlock"}
                            </button>
                        </span>
                    )}
                </span>
                {/* The same actions the table view offers, from the same component - a job
                    could be dragged to Done here and then had to be reopened in the table to
                    tell anybody about it, which is how a new view teaches people not to use
                    it. Whether each control applies is JobAlertActions' decision, not this
                    view's, so the two cannot drift apart. */}
                <JobAlertActions
                    job={{ ...job, rows }}
                    onChange={(updated) => onChanged?.(updated)}
                    className="is-board"
                />
                <button type="button" className="shell-icon-btn" aria-label="Close board" onClick={close}>
                    <X size={16} />
                </button>
            </div>

            <div className="job-board-columns">
                {columns.map((column) => (
                    <div
                        key={column.stage}
                        data-stage={column.stage}
                        className={[
                            "job-board-column",
                            column.offPipeline ? "is-off-pipeline" : "",
                            over === column.stage ? "is-over" : "",
                            dragStage === column.stage ? "is-moving" : "",
                        ]
                            .filter(Boolean)
                            .join(" ")}
                        onDragOver={(e) => {
                            e.preventDefault();
                            // A column being dragged over another reorders; a card does not.
                            if (dragStage) return;
                            setOver(column.stage);
                        }}
                        onDragLeave={() => setOver((s) => (s === column.stage ? null : s))}
                        onDrop={() => {
                            if (dragStage) {
                                if (!column.offPipeline) moveColumn(dragStage, column.stage);
                                setDragStage(null);
                                return;
                            }
                            drop(column.stage);
                        }}
                    >
                        <div
                            className="job-board-column-head"
                            // The HEAD is the handle, not the whole column - a column that is
                            // itself draggable swallows the drag of every card inside it.
                            draggable={canEditQueue && !column.offPipeline && renaming !== column.stage}
                            onDragStart={(e) => {
                                e.stopPropagation();
                                setDragStage(column.stage);
                            }}
                            onDragEnd={() => setDragStage(null)}
                            onDoubleClick={() => {
                                if (!canEditQueue || column.offPipeline) return;
                                setRenaming(column.stage);
                                setRenameTo(column.stage);
                            }}
                            title={canEditQueue && !column.offPipeline ? "Drag to reorder, double-click to rename" : undefined}
                        >
                            {renaming === column.stage ? (
                                <input
                                    className="form-control job-board-column-rename"
                                    autoFocus
                                    value={renameTo}
                                    onChange={(e) => setRenameTo(e.target.value)}
                                    onBlur={() => commitRename(column.stage)}
                                    onKeyDown={(e) => {
                                        if (e.key === "Enter") commitRename(column.stage);
                                        if (e.key === "Escape") setRenaming(null);
                                    }}
                                    aria-label={`Rename ${column.stage}`}
                                />
                            ) : (
                                <span className="text-label-caps">{column.stage}</span>
                            )}
                            <span className="text-body-small">{column.cards.length}</span>
                        </div>

                        {/* Said on the column rather than only when a drop is refused: knowing
                            why a card cannot come here beats finding out by trying. */}
                        {column.offPipeline && (
                            <span className="text-body-small job-board-column-note">Not a stage on this job</span>
                        )}

                        <div className="job-board-column-cards">
                            {column.cards.length === 0 && (
                                <div className="job-board-column-drop text-body-small">Drop here</div>
                            )}
                            {column.cards.map((c) => (
                                <div
                                    key={c._id}
                                    // Gated, like the column head and the assign picker beside
                                    // it. This was the one drag affordance left ungated - it
                                    // did not matter while a locked job-id never reached the
                                    // board, and it matters now that part-invoiced ones do:
                                    // every drop would travel to the API and come back refused.
                                    draggable={canEditQueue}
                                    className={`job-board-chip text-body-small${
                                        String(dragId) === String(c._id) ? " is-dragging" : ""
                                    }${canEditQueue ? "" : " is-frozen"}`}
                                    onDragStart={() => canEditQueue && setDragId(c._id)}
                                    onDragEnd={() => setDragId(null)}
                                >
                                    {/* The NAME opens the editor, not the whole chip. A chip
                                        that is a drag handle, a click target and the container
                                        of a dropdown is three controls fighting over one press
                                        - which is why clicking a card did nothing. */}
                                    <button
                                        type="button"
                                        className="job-board-chip-open"
                                        disabled={!canEditValues}
                                        onClick={(e) =>
                                            setEditing({ card: c, rect: e.currentTarget.closest(".job-board-chip")?.getBoundingClientRect() })
                                        }
                                        title={canEditValues ? "Edit this card" : "This job is invoiced"}
                                    >
                                        {c.material || c.description || c.rowId}
                                        {/* The size, beside the name. A card called "Vinyl" is
                                            not the same job as another card called "Vinyl" -
                                            what tells them apart is how big it is. */}
                                        <span className="job-board-dims">{cardSize(c)}</span>
                                    </button>
                                    <span className="job-board-chip-sub text-body-small">{c.rowId}</span>
                                    {/* Not on a finished card. The rule that demands an owner
                                        applies to working stages only, so asking who is doing
                                        a thing already done is a control that can never
                                        matter. */}
                                    {column.stage !== LAST_STAGE && (
                                    <div className="job-board-chip-assign" onDragStart={(e) => e.preventDefault()}>
                                        {/* Said once per card rather than left as the picker's
                                            bare "No matches", which reads as a broken search
                                            rather than as an empty address book. */}
                                        {employeeOptions.length === 0 && (
                                            <span className="text-body-small job-board-chip-sub">
                                                No employees yet — open this to add one
                                            </span>
                                        )}
                                        <AssigneeDropdown
                                            value={c.employee_id?.name || ""}
                                            placeholder="Unassigned"
                                            options={employeeOptions}
                                            onSelect={(id) => assign(c, id)}
                                            onCreateNew={(name) => createEmployee(name, c)}
                                            fullWidth
                                            variant="dashed"
                                            disabled={!canEditQueue}
                                        />
                                    </div>
                                    )}
                                </div>
                            ))}
                        </div>

                        {canEditValues && !column.offPipeline && columns[0].stage === column.stage && (
                            <button type="button" className="job-board-add" onClick={addCard} disabled={busy}>
                                <Plus size={13} /> Add card
                            </button>
                        )}
                    </div>
                ))}

                {canEditQueue && (
                    <button type="button" className="job-board-add-stage" onClick={addStage} disabled={busy}>
                        <Plus size={14} /> Add stage
                    </button>
                )}
            </div>

            {creatingFor && (
                <QuickCreateEmployeeModal
                    isOpen
                    initialName={creatingFor.name}
                    toggle={() => setCreatingFor(null)}
                    onCreated={employeeCreated}
                />
            )}

            {addingRect && (
                <NewCardPanel
                    job={{ ...job, rows }}
                    rect={addingRect}
                    onClose={() => setAddingRect(null)}
                    onCreated={(updated) => onChanged?.(updated)}
                />
            )}

            {editing && (
                <JobCardPanel
                    // The card as the BOARD currently has it, not as it was when the panel
                    // opened - assigning from inside the panel updates `rows`, and a stale copy
                    // would show the picker empty a moment after someone filled it.
                    job={{ ...job, rows }}
                    card={rows.find((r) => String(r._id) === String(editing.card._id)) || editing.card}
                    rect={editing.rect}
                    employees={employeeOptions}
                    onAssign={canEditQueue ? assign : undefined}
                    onCreateEmployee={canEditQueue ? createEmployee : undefined}
                    onClose={() => setEditing(null)}
                    onSaved={(updated) => onChanged?.(updated)}
                />
            )}
        </div>
        </>
    );
}
