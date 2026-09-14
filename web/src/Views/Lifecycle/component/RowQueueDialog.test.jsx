import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const notifyError = vi.fn();
vi.mock("../../../global/toast", () => ({
    notifySuccess: vi.fn(),
    notifyError: (...a) => notifyError(...a),
}));
vi.mock("../lifecycle_backend", () => ({
    lifecycleBackend: {
        setDefaultQueueOrder: vi.fn(),
        updateRowQueueOrder: vi.fn(),
        setRowQueue: vi.fn(),
    },
}));

import RowQueueDialog from "./RowQueueDialog";

const ORDER = ["Created", "Printing", "Ready-to-Pickup", "Done"];

// Two cards on one job-id, deliberately at different stages: that is the case where "use this
// order for the whole job-id" and "this card only" disagree about what may be removed.
const makeJob = () => ({
    _id: "j1",
    queueOrder: ORDER,
    rows: [
        { _id: "r1", rowId: "MG/1-1", queue: "Created", queueOrder: ORDER },
        { _id: "r2", rowId: "MG/1-2", queue: "Printing", queueOrder: ORDER },
    ],
});

// Removing is behind the Edit toggle - a stray click while reaching for "Complete up to here"
// must not re-plan live production - so every test has to unlock first.
const openEditing = (rowIndex = 0) => {
    const job = makeJob();
    render(<RowQueueDialog job={job} row={job.rows[rowIndex]} onClose={vi.fn()} onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: /Locked|Edit/i }));
};

// One Remove button per non-pinned stage, in pipeline order: Printing, then Ready-to-Pickup.
// Created and Done have none - they cannot be removed at all.
const removeStage = (stage) => {
    const order = ["Printing", "Ready-to-Pickup"];
    const buttons = screen.getAllByRole("button", { name: /Remove stage/i });
    fireEvent.click(buttons[order.indexOf(stage)]);
};

// The pipeline, read off the draggable cards. Not getByText: a stage that has been removed
// reappears in the "add a stage" search list below, so matching on text alone would report a
// removal as having failed.
const pipelineCards = () => Array.from(document.querySelectorAll("[draggable]"));

const stageCard = (stage) =>
    pipelineCards().find((el) => el.textContent.includes(stage));

const pipeline = () =>
    pipelineCards().map((el) => el.querySelector("span:nth-of-type(2)")?.textContent?.trim());

const uncheckApplyToAll = () =>
    fireEvent.click(screen.getByRole("checkbox", { name: /whole job-id/i }));

beforeEach(() => {
    vi.clearAllMocks();
});

describe("RowQueueDialog stage removal", () => {
    // The guard must not block ordinary re-planning, which is what the editor is for.
    it("removes a stage no card is standing on", () => {
        openEditing(0);
        expect(pipeline()).toContain("Ready-to-Pickup");
        removeStage("Ready-to-Pickup");
        expect(notifyError).not.toHaveBeenCalled();
        expect(pipeline()).not.toContain("Ready-to-Pickup");
    });

    // Card r2 is on Printing. With the order about to be copied to every card, removing
    // Printing would leave r2 indexed at -1 in its own pipeline - unreachable by "complete up
    // to here", and mis-bucketed on the board.
    it("refuses a stage another card is standing on, and says how many", () => {
        openEditing(0);
        removeStage("Printing");
        expect(pipeline()).toContain("Printing");
        expect(notifyError).toHaveBeenCalledWith("Can't remove Printing - one card is on it.");
    });

    // The shake has to be on the stage that was refused, and only on that one.
    it("shakes the stage that was refused", () => {
        openEditing(0);
        removeStage("Printing");
        expect(stageCard("Printing").className).toContain("queue-stage-refused");
        expect(stageCard("Ready-to-Pickup").className || "").not.toContain("queue-stage-refused");
        expect(stageCard("Created").className || "").not.toContain("queue-stage-refused");
    });

    // Scoped to what is actually about to be written. With the copy turned off, only this
    // card's pipeline changes - and this card is on Created, not Printing, so the removal is
    // safe and refusing it would block a legitimate edit.
    it("allows the same removal once the order is no longer copied to every card", () => {
        openEditing(0);
        uncheckApplyToAll();
        removeStage("Printing");
        expect(notifyError).not.toHaveBeenCalled();
        expect(pipeline()).not.toContain("Printing");
    });

    // The card in front of you counts too - "this card only" is not a way around the guard.
    it("refuses removing the stage this very card is standing on", () => {
        openEditing(1);
        uncheckApplyToAll();
        removeStage("Printing");
        expect(pipeline()).toContain("Printing");
        expect(notifyError).toHaveBeenCalledWith("Can't remove Printing - one card is on it.");
    });
});
