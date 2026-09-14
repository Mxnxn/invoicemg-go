import { describe, expect, it } from "vitest";
import { boardColumns, canDrop } from "./jobBoard";

const ORDER = ["Created", "Printing", "Ready-to-Pickup", "Done"];
const row = (over = {}) => ({ _id: "r1", rowId: "MG/1-1", queue: "Created", employee_id: null, ...over });

describe("boardColumns", () => {
    it("takes its columns from the job's own pipeline", () => {
        const cols = boardColumns({ queueOrder: ORDER, rows: [] });
        expect(cols.map((c) => c.stage)).toEqual(ORDER);
    });

    // Where a pipeline actually lives: /jobs/rows/queue-order writes row.queueOrder and leaves
    // the job's untouched, so a board reading only the job would apply every reorder to a field
    // it never looks at.
    it("prefers the pipeline stored on the rows, which is where changes are written", () => {
        const rowOrder = ["Created", "Ready-to-Pickup", "Printing", "Done"];
        const cols = boardColumns({
            queueOrder: ORDER,
            rows: [row({ queueOrder: rowOrder })],
        });
        expect(cols.map((c) => c.stage)).toEqual(rowOrder);
    });

    it("falls back to the job's own order when no row carries one", () => {
        const cols = boardColumns({ queueOrder: ORDER, rows: [row({ queueOrder: [] })] });
        expect(cols.map((c) => c.stage)).toEqual(ORDER);
    });

    it("falls back to the company order, then to the built-in stages", () => {
        const company = ["Created", "Cutting", "Done"];
        expect(boardColumns({ rows: [] }, company).map((c) => c.stage)).toEqual(company);
        expect(boardColumns({ rows: [] }).map((c) => c.stage)).toEqual([
            "Created",
            "Printing",
            "Ready-to-Pickup",
            "Done",
        ]);
    });

    it("places a card in the column matching its stage", () => {
        const cols = boardColumns({ queueOrder: ORDER, rows: [row({ queue: "Printing" })] });
        expect(cols.find((c) => c.stage === "Printing").cards).toHaveLength(1);
        expect(cols.find((c) => c.stage === "Created").cards).toHaveLength(0);
    });

    // The one genuinely bad outcome: a card that disappears because its own history disagrees
    // with its job's. The whole point of the view is that everything on a job-id is visible.
    it("gives a card at a stage outside the pipeline its own marked column", () => {
        const cols = boardColumns({ queueOrder: ORDER, rows: [row({ queue: "Lamination" })] });
        const stray = cols[cols.length - 1];
        expect(stray.stage).toBe("Lamination");
        expect(stray.offPipeline).toBe(true);
        expect(stray.cards).toHaveLength(1);
        expect(cols.filter((c) => c.offPipeline)).toHaveLength(1);
    });

    it("gathers several strays at one unknown stage into a single column", () => {
        const cols = boardColumns({
            queueOrder: ORDER,
            rows: [row({ _id: "a", queue: "Lamination" }), row({ _id: "b", queue: "Lamination" })],
        });
        expect(cols.filter((c) => c.offPipeline)).toHaveLength(1);
        expect(cols[cols.length - 1].cards).toHaveLength(2);
    });

    it("renders every card exactly once even when cards disagree on pipeline", () => {
        const cols = boardColumns({
            queueOrder: ORDER,
            rows: [row({ _id: "a", queue: "Printing" }), row({ _id: "b", queue: "Lamination" }), row({ _id: "c" })],
        });
        const ids = cols.flatMap((c) => c.cards.map((r) => r._id));
        expect(ids.sort()).toEqual(["a", "b", "c"]);
    });

    it("survives a job with no rows, and no job at all", () => {
        expect(boardColumns({ queueOrder: ORDER }).every((c) => c.cards.length === 0)).toBe(true);
        expect(boardColumns(null)).toHaveLength(4);
    });
});

describe("canDrop", () => {
    const cols = boardColumns({ queueOrder: ORDER, rows: [] });

    // Work in progress needs someone doing it. Raising and finishing do not.
    it("refuses a working stage when nobody is assigned", () => {
        const r = canDrop(row(), "Printing", cols);
        expect(r.ok).toBe(false);
        expect(r.reason).toMatch(/assign/i);
    });

    it("allows the first and last stages with nobody assigned", () => {
        expect(canDrop(row({ queue: "Printing" }), "Created", cols).ok).toBe(true);
        expect(canDrop(row({ queue: "Printing" }), "Done", cols).ok).toBe(true);
    });

    it("allows any stage once the card has someone on it", () => {
        const assigned = row({ employee_id: "e1" });
        expect(canDrop(assigned, "Printing", cols).ok).toBe(true);
        expect(canDrop(assigned, "Ready-to-Pickup", cols).ok).toBe(true);
    });

    // A stray column is somewhere to drag OUT of, not into: it is not a stage of this pipeline.
    it("refuses a drop into an off-pipeline column", () => {
        const withStray = boardColumns({ queueOrder: ORDER, rows: [row({ queue: "Lamination" })] });
        const r = canDrop(row({ employee_id: "e1" }), "Lamination", withStray);
        expect(r.ok).toBe(false);
        expect(r.reason).toMatch(/not a stage/i);
    });

    // A stray column is appended AFTER the pipeline, so the last real stage stops being the
    // last element of the array. Done must still be treated as the end.
    it("still treats the last pipeline stage as the end when a stray column exists", () => {
        const withStray = boardColumns({ queueOrder: ORDER, rows: [row({ _id: "s", queue: "Lamination" })] });
        expect(canDrop(row({ queue: "Printing" }), "Done", withStray).ok).toBe(true);
    });

    // An admin moving a card IS the person the rule would be protecting - making them stop and
    // name a colleague before tidying a board is a check that only obstructs its own author.
    it("does not ask an admin for an assignee", () => {
        expect(canDrop(row(), "Printing", cols, { isAdmin: true }).ok).toBe(true);
        expect(canDrop(row(), "Printing", cols, { isAdmin: false }).ok).toBe(false);
    });

    // Everything else still applies to an admin: the rule waived is the assignee, not the board.
    it("still refuses an admin an off-pipeline column or an unknown stage", () => {
        const withStray = boardColumns({ queueOrder: ORDER, rows: [row({ queue: "Lamination" })] });
        expect(canDrop(row(), "Lamination", withStray, { isAdmin: true }).ok).toBe(false);
        expect(canDrop(row(), "Nonsense", cols, { isAdmin: true }).ok).toBe(false);
    });

    it("refuses a stage that is not on the board at all", () => {
        expect(canDrop(row({ employee_id: "e1" }), "Nonsense", cols).ok).toBe(false);
    });

    // Not an error - just nothing to do. An empty reason tells the caller to say nothing.
    it("is a silent no-op when the card is already there", () => {
        const r = canDrop(row({ queue: "Created" }), "Created", cols);
        expect(r.ok).toBe(false);
        expect(r.reason).toBe("");
    });

    it("is a silent no-op with no card or no stage", () => {
        expect(canDrop(null, "Printing", cols)).toEqual({ ok: false, reason: "" });
        expect(canDrop(row(), "", cols)).toEqual({ ok: false, reason: "" });
    });
});
