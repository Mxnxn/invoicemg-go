import { QUEUE_STAGES } from "./component/queueConstants";
import { rowDimensions } from "./jobMath";

// Mirrors LAST_STAGE in the API's Helpers/QueueOrder.js, which pins both ends of a pipeline on
// every write. Repeated rather than imported because that module is server-side.
export const LAST_STAGE = "Done";

/**
 * The board's columns, and which card sits in each.
 *
 * The pipeline is resolved the way the rest of the app resolves it: the job's own order, else
 * the company default, else the built-in stages.
 *
 * A card at a stage the pipeline does not contain gets a column of its own at the far right,
 * marked. It is never dropped. A card that vanishes because its own history disagrees with its
 * job's is the one outcome this view cannot have - the point of it is that everything on a
 * job-id is visible at once - and marking it makes the rare broken case findable rather than
 * invisible.
 */
export function boardColumns(job, companyOrder) {
    // Row first, then the job, then the company, then the built-in stages - the same order
    // RowQueueDialog resolves in, and it has to be, because that is where a pipeline is
    // actually stored. /jobs/rows/queue-order writes row.queueOrder and leaves job.queueOrder
    // untouched, so a board that read only the job would apply every reorder to a field it
    // never looks at and appear to do nothing.
    //
    // The first row is enough: the board only ever writes with all_rows, so every card on a
    // job-id shares one order. A job whose rows genuinely disagree still renders - the
    // stragglers come out as off-pipeline columns, which is the case this board draws in amber.
    const fromRows = (job?.rows || []).map((r) => r.queueOrder).find((o) => Array.isArray(o) && o.length);
    const pipeline =
        fromRows ||
        (job && Array.isArray(job.queueOrder) && job.queueOrder.length && job.queueOrder) ||
        (Array.isArray(companyOrder) && companyOrder.length && companyOrder) ||
        QUEUE_STAGES;

    // Done last, whatever order arrives. The server pins it (Helpers/QueueOrder.js) on every
    // write, but a pipeline can reach here from an older document or from a rename that has not
    // round-tripped yet - and a board with Done in the middle says work finishes halfway
    // through itself. Pinned on display as well as on write, so the two cannot disagree.
    const ordered = [...pipeline].sort((a, b) => (a === LAST_STAGE) - (b === LAST_STAGE));

    const columns = ordered.map((stage) => ({ stage, offPipeline: false, cards: [] }));
    const byStage = new Map(columns.map((c) => [c.stage, c]));

    (job?.rows || []).forEach((card) => {
        const home = byStage.get(card.queue);
        if (home) {
            home.cards.push(card);
            return;
        }
        // The first card at an unknown stage opens a column for it and registers it, so the
        // next card at that stage finds it as `home` above and joins it rather than opening a
        // second column for the same name.
        const stray = { stage: card.queue, offPipeline: true, cards: [card] };
        columns.push(stray);
        byStage.set(card.queue, stray);
    });

    return columns;
}

/**
 * Whether this card may be dropped on this stage, and what to say when it may not.
 *
 * The assignee rule lives here and only here: a card moving into a WORKING stage needs somebody
 * on it. A working stage is one that is neither the first nor the last of the PIPELINE - work
 * in progress needs a person, raising and finishing do not.
 *
 * Enforced in the board rather than on the server because cards are advanced elsewhere today
 * with nobody assigned, and a server rule would start refusing a workflow that currently works.
 *
 * A reason of "" means "nothing to do" rather than "refused" - the caller shows no message.
 */
export function canDrop(row, stage, columns, { isAdmin = false } = {}) {
    if (!row || !stage) return { ok: false, reason: "" };
    // Dropped where it already was. Not an error, so nothing is said.
    if (row.queue === stage) return { ok: false, reason: "" };

    const all = columns || [];
    const index = all.findIndex((c) => c.stage === stage);
    if (index === -1) return { ok: false, reason: "That stage is not on this board." };

    // Somewhere to drag out of, not into: a stray column is not a stage of this pipeline.
    if (all[index].offPipeline) return { ok: false, reason: `${stage} is not a stage on this job.` };

    // Counted against the PIPELINE, not the array. Stray columns are appended after it, so
    // using all.length here would stop the real last stage being the last one and start
    // demanding an assignee to finish a card.
    const pipelineCount = all.filter((c) => !c.offPipeline).length;
    const working = index > 0 && index < pipelineCount - 1;

    // An admin is not asked. The rule exists so work in progress has someone accountable for
    // it, and an admin moving a card is already that someone - they are the person the rule
    // would otherwise be protecting. Making them stop and name a colleague before they can
    // tidy a board is a check that only ever gets in the way of the person who imposed it.
    if (working && !row.employee_id && !isAdmin) {
        return {
            ok: false,
            reason: `Assign someone to ${row.rowId || "this card"} before moving it to ${stage}.`,
        };
    }
    return { ok: true, reason: "" };
}

/**
 * How a card is measured, in a few characters: its size, or its quantity when it has no size.
 *
 * Shared by the board's chips and the grid's product badges, because a card described one way
 * in one place and another way two clicks later is two descriptions of the same thing.
 *
 * rowDimensions deliberately returns "" for a by-quantity row rather than "1 x 1" - printing a
 * size a row does not have is the bug it exists to avoid - so the quantity stands in.
 *
 * A bare "1 x 1" is dropped for the same reason it should never have been printed: it is a
 * dimensional row whose sides nobody filled in, it is on almost every card in a shop that works
 * by quantity, and it says nothing. A row genuinely one unit square loses nothing by being
 * described by its count instead.
 */
export const cardSize = (row) => {
    const dims = rowDimensions(row);
    const qty = Number(row?.qty || 0);
    if (dims && dims !== "1 x 1") return qty > 1 ? `${dims} · ${qty}` : dims;
    return qty ? `Qty ${qty}` : "";
};
