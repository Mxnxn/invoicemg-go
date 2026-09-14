import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const setRowQueue = vi.fn();
const assignRow = vi.fn();
const lookupPeople = vi.fn();
const updateRowQueueOrder = vi.fn();
const createPerson = vi.fn();
const unlockJob = vi.fn();
vi.mock("../lifecycle_backend", () => ({
    lifecycleBackend: {
        setRowQueue: (...a) => setRowQueue(...a),
        assignRow: (...a) => assignRow(...a),
        lookupPeople: (...a) => lookupPeople(...a),
        updateJob: vi.fn(),
        updateRowQueueOrder: (...a) => updateRowQueueOrder(...a),
        unlockJob: (...a) => unlockJob(...a),
    },
}));
const notifyError = vi.fn();
vi.mock("../../../Common/person_backend", () => ({
    personBackend: { create: (...a) => createPerson(...a), setNotifyPreference: vi.fn(), list: vi.fn() },
}));
vi.mock("../../../global/toast", () => ({ notifyError: (...a) => notifyError(...a), notifySuccess: vi.fn() }));

import JobBoardOverlay from "./JobBoardOverlay";

const ORDER = ["Created", "Printing", "Ready-to-Pickup", "Done"];
const job = (rows) => ({
    _id: "j1",
    challanNumber: "MG/26-27/1",
    queueOrder: ORDER,
    rows,
    lock: { canEditQueue: true, canEditValues: true },
});
const card = (over = {}) => ({ _id: "r1", rowId: "MG-1", material: "Vinyl", queue: "Created", employee_id: "e1", ...over });

const columnOf = (stage) => document.querySelector(`[data-stage="${stage}"]`);

const drag = (cardText, stage) => {
    const chip = screen.getByText(cardText).closest("[draggable]");
    const column = columnOf(stage);
    fireEvent.dragStart(chip);
    fireEvent.dragOver(column);
    fireEvent.drop(column);
};

// jsdom has no layout, so every getBoundingClientRect is zeroes - and the FLIP is measured
// from the panel's real rect, so without this it correctly computes "no transform" and there
// is nothing to assert. Giving the panel a size makes the maths real.
const PANEL = { top: 24, left: 24, width: 1112, height: 720 };
beforeEach(() => {
    Element.prototype.getBoundingClientRect = function () {
        if (this.classList?.contains("job-board-overlay")) return { ...PANEL, right: 1136, bottom: 744 };
        return { top: 0, left: 0, width: 0, height: 0, right: 0, bottom: 0 };
    };
});

beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    setRowQueue.mockResolvedValue({ code: 200, data: { _id: "j1", rows: [] } });
    assignRow.mockResolvedValue({ code: 200, data: { _id: "j1", rows: [] } });
    updateRowQueueOrder.mockResolvedValue({ code: 200, data: { _id: "j1", rows: [] } });
    createPerson.mockResolvedValue({ code: 200, data: { _id: "new1", name: "Sunil", type: "Employee" } });
    unlockJob.mockResolvedValue({ code: 200, data: { _id: "j1", rows: [] } });
    lookupPeople.mockResolvedValue({
        code: 200,
        data: [
            { _id: "e1", name: "Ravi Kumar", type: "Employee" },
            { _id: "e2", name: "Priya Nair", type: "Employee" },
            { _id: "s1", name: "Bolt Vinyls", type: "Supplier" },
        ],
    });
});

describe("JobBoardOverlay", () => {
    // The size is what tells two cards called "Vinyl" apart.
    it("shows the size beside the product", () => {
        render(<JobBoardOverlay job={job([card({ length: "4", width: "3" })])} onClose={vi.fn()} />);
        expect(screen.getByRole("button", { name: /Vinyl.*4 x 3/ })).toBeTruthy();
    });

    // A row sold by the piece has no size, and must not print "1 x 1" - the quantity is what
    // that row is measured by.
    it("shows the quantity instead when a card has no dimensions", () => {
        render(<JobBoardOverlay job={job([card({ hasDimensions: false, qty: 7 })])} onClose={vi.fn()} />);
        expect(screen.getByRole("button", { name: /Vinyl.*Qty 7/ })).toBeTruthy();
    });

    // "1 x 1" is a dimensional row whose sides nobody filled in - the default of a default. It
    // is on almost every card in a shop that works by quantity and it says nothing.
    it("does not print 1 x 1, which is a size nobody set", () => {
        render(<JobBoardOverlay job={job([card({ qty: 4 })])} onClose={vi.fn()} />);
        expect(screen.queryByRole("button", { name: /1 x 1/ })).toBeNull();
        expect(screen.getByRole("button", { name: /Vinyl.*Qty 4/ })).toBeTruthy();
    });

    // An empty stage had to be hit almost exactly - you aimed at the heading and hoped.
    it("gives an empty column a card-sized place to drop", () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        expect(columnOf("Printing").textContent).toContain("Drop here");
        expect(columnOf("Created").textContent).not.toContain("Drop here");
    });

    it("shows what the job is worth and whether it is billed", () => {
        const billed = { ...job([card()]), total: 12400, lock: { canEditQueue: false, canEditValues: false, invoiced: true } };
        const { rerender } = render(<JobBoardOverlay job={billed} onClose={vi.fn()} />);
        expect(screen.getByText(/12,400/)).toBeTruthy();
        expect(screen.getByText("Invoiced")).toBeTruthy();

        rerender(<JobBoardOverlay job={{ ...job([card()]), total: 900 }} onClose={vi.fn()} />);
        expect(screen.getByText("Open")).toBeTruthy();
    });

    it("draws a column per stage and puts each card in its own", () => {
        render(<JobBoardOverlay job={job([card({ queue: "Printing" })])} onClose={vi.fn()} />);
        ORDER.forEach((s) => expect(screen.getByText(s)).toBeTruthy());
        expect(columnOf("Printing").textContent).toContain("Vinyl");
    });

    it("moves a card by dragging it, through the existing endpoint", async () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} onChanged={vi.fn()} />);
        drag("Vinyl", "Printing");
        await waitFor(() => expect(setRowQueue).toHaveBeenCalled());
        const sent = setRowQueue.mock.calls[0][0];
        expect(sent.get("job_id")).toBe("j1");
        expect(sent.get("row_id")).toBe("r1");
        expect(sent.get("queue")).toBe("Printing");
    });

    // The rule this board adds. Nothing reaches the server.
    //
    // An EMPLOYEE session, explicitly: an admin is not asked for an assignee, and with no role
    // in storage Common/access.js reads the session as an admin - so a test that left it unset
    // would be exercising the waiver rather than the rule.
    it("refuses a working stage when nobody is assigned, and says so", () => {
        window.localStorage.setItem("role", "employee");
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} />);
        drag("Vinyl", "Printing");
        expect(setRowQueue).not.toHaveBeenCalled();
        expect(notifyError).toHaveBeenCalledWith(expect.stringMatching(/assign/i));
    });

    it("does not ask an admin, who is the person the rule would be protecting", async () => {
        window.localStorage.setItem("role", "admin");
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} onChanged={vi.fn()} />);
        drag("Vinyl", "Printing");
        await waitFor(() => expect(setRowQueue).toHaveBeenCalled());
        expect(notifyError).not.toHaveBeenCalled();
    });

    it("allows an unassigned card to reach Done", async () => {
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} onChanged={vi.fn()} />);
        drag("Vinyl", "Done");
        await waitFor(() => expect(setRowQueue).toHaveBeenCalled());
    });

    // A card that appears to move and then silently returns on the next refresh is worse than
    // one that never moved.
    it("puts the card back when the server refuses", async () => {
        setRowQueue.mockRejectedValue({ message: "Job is locked." });
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        drag("Vinyl", "Printing");
        await waitFor(() => expect(columnOf("Created").textContent).toContain("Vinyl"));
    });

    it("shows a card at an unknown stage in its own marked column", () => {
        render(<JobBoardOverlay job={job([card({ queue: "Lamination" })])} onClose={vi.fn()} />);
        expect(screen.getByText("Lamination")).toBeTruthy();
        expect(columnOf("Lamination").textContent).toContain("Vinyl");
    });

    // Dropped where it already was is not a refusal, so nothing is said.
    it("says nothing when a card is dropped back where it was", () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        drag("Vinyl", "Created");
        expect(setRowQueue).not.toHaveBeenCalled();
        expect(notifyError).not.toHaveBeenCalled();
    });

    it("marks an unassigned card as such, so the refusal is not a surprise", () => {
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} />);
        expect(screen.getByText(/unassigned/i)).toBeTruthy();
    });

    // Gated on the lock the server already computes, so a control and the route it calls
    // cannot disagree about whether something is allowed.
    it("offers adding a card and a stage on an open job", () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        expect(screen.getByRole("button", { name: /add card/i })).toBeTruthy();
        expect(screen.getByRole("button", { name: /add stage/i })).toBeTruthy();
    });

    it("offers neither on an invoiced job", () => {
        const locked = { ...job([card()]), lock: { canEditQueue: false, canEditValues: false } };
        render(<JobBoardOverlay job={locked} onClose={vi.fn()} />);
        expect(screen.queryByRole("button", { name: /add card/i })).toBeNull();
        expect(screen.queryByRole("button", { name: /add stage/i })).toBeNull();
    });

    // Work starts where work starts - so the + is on the first column and nowhere else.
    it("offers Add card on the first column only", () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        expect(screen.getAllByRole("button", { name: /add card/i })).toHaveLength(1);
        expect(columnOf("Created").textContent).toContain("Add card");
        expect(columnOf("Printing").textContent).not.toContain("Add card");
    });

    it("closes on Escape", () => {
        const onClose = vi.fn();
        render(<JobBoardOverlay job={job([])} onClose={onClose} />);
        fireEvent.keyDown(document, { key: "Escape" });
        expect(onClose).toHaveBeenCalled();
    });

    // The expansion run backwards. Vanishing instead left the motion half-finished: it arrived
    // from somewhere and then went nowhere, which reads as the page having dropped it.
    it("shrinks back into the card it came from before it closes", () => {
        const rect = { top: 120, left: 300, width: 260, height: 128 };
        render(<JobBoardOverlay job={job([card()])} rect={rect} onClose={vi.fn()} />);
        const panel = document.querySelector(".job-board-overlay");

        fireEvent.keyDown(document, { key: "Escape" });
        // Back over the card by TRANSFORM, not by resizing: a transform is one composited move,
        // where animating width and height re-lays-out the columns every frame.
        expect(panel.className).toContain("is-closing");
        expect(panel.style.transform).toMatch(/translate\(.+\) scale\(/);
        // Measured against the panel's real rect, not a size recomputed here - the layout lives
        // in the stylesheet and expressing it twice is what lets the two drift.
        const [, sx] = panel.style.transform.match(/scale\(([\d.]+),/);
        expect(Number(sx)).toBeCloseTo(260 / PANEL.width, 3);
    });

    // Overshoot passes its target and comes back. On the way in that is a settle; on the way
    // out it shrank past the card and sprang back, which is the bounce. Arrival and departure
    // do not share an easing.
    it("does not use the overshoot easing on the way out", async () => {
        const rect = { top: 120, left: 300, width: 260, height: 128 };
        render(<JobBoardOverlay job={job([card()])} rect={rect} onClose={vi.fn()} />);
        const panel = document.querySelector(".job-board-overlay");
        // Two frames to go measuring -> placed -> opening, which is what gives the transform
        // something to animate from.
        await waitFor(() => expect(panel.className).toContain("is-opening"));

        fireEvent.keyDown(document, { key: "Escape" });
        expect(panel.className).toContain("is-closing");
        expect(panel.className).not.toContain("is-opening");
    });

    // Escape held down, or Escape and the X together, must not close twice.
    // The close now waits for the transition to finish, so onClose lands later than the press.
    it("only closes once however many times it is asked", async () => {
        const onClose = vi.fn();
        const rect = { top: 120, left: 300, width: 260, height: 128 };
        render(<JobBoardOverlay job={job([card()])} rect={rect} onClose={onClose} />);
        fireEvent.keyDown(document, { key: "Escape" });
        fireEvent.keyDown(document, { key: "Escape" });
        fireEvent.click(screen.getByRole("button", { name: /close board/i }));
        await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1), { timeout: 1500 });
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    // jsdom fires no transitionend, so without the fallback the board would hang on screen -
    // which is also what would happen in a browser under prefers-reduced-motion.
    it("still closes when no transition ever runs", async () => {
        const onClose = vi.fn();
        render(
            <JobBoardOverlay job={job([card()])} rect={{ top: 1, left: 1, width: 10, height: 10 }} onClose={onClose} />
        );
        fireEvent.keyDown(document, { key: "Escape" });
        await waitFor(() => expect(onClose).toHaveBeenCalled(), { timeout: 1500 });
    });

    // Opened without a card - there is no journey to reverse, so it just goes.
    it("closes immediately when there is nothing to shrink into", () => {
        const onClose = vi.fn();
        render(<JobBoardOverlay job={job([card()])} onClose={onClose} />);
        fireEvent.click(screen.getByRole("button", { name: /close board/i }));
        expect(onClose).toHaveBeenCalledTimes(1);
    });
});

// The rule refuses a move until somebody is on the card, so the board has to be somewhere you
// can put somebody on a card. Without this the refusal is unfixable from here: every message
// says "assign someone" and there is nowhere to do it.
describe("assigning from the board", () => {
    it("offers a picker on every card", async () => {
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        expect(screen.getByText(/unassigned/i)).toBeTruthy();
    });

    it("offers employees and not suppliers", async () => {
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        fireEvent.click(screen.getByText(/unassigned/i));
        await waitFor(() => expect(screen.getByText("Ravi Kumar")).toBeTruthy());
        expect(screen.getByText("Priya Nair")).toBeTruthy();
        expect(screen.queryByText("Bolt Vinyls")).toBeNull();
    });

    // Its own route, not part of editing a card's values - the server keeps progress in step
    // with it, which /jobs/update would not.
    it("assigns through the assign route", async () => {
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} onChanged={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        fireEvent.click(screen.getByText(/unassigned/i));
        await waitFor(() => expect(screen.getByText("Ravi Kumar")).toBeTruthy());
        fireEvent.click(screen.getByText("Ravi Kumar"));
        await waitFor(() =>
            expect(assignRow).toHaveBeenCalledWith({ job_id: "j1", row_id: "r1", employee_id: "e1" })
        );
    });

    // The name is the editor's trigger, not the whole chip: a chip that is a drag handle, a
    // click target and a dropdown container at once is three controls fighting over one press.
    it("opens the editor from the card's name", async () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        // The name now carries the size too, so an exact match no longer describes it.
        fireEvent.click(screen.getByRole("button", { name: /^Vinyl/ }));
        // Asserted on the popover's own fields: the board overlay is a dialog too, so a role
        // query would match both and prove nothing about which one opened.
        await waitFor(() => expect(screen.getByLabelText(/product/i)).toBeTruthy());
        expect(screen.getByLabelText(/quantity/i)).toBeTruthy();
    });
});

// The board is not a view with its own arrangement - it IS the order work moves through, so
// rearranging columns rearranges the pipeline.
describe("rearranging and renaming columns", () => {
    const headOf = (stage) => document.querySelector(`[data-stage="${stage}"] .job-board-column-head`);

    it("reorders the pipeline by dragging a column head", async () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} onChanged={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());

        fireEvent.dragStart(headOf("Printing"));
        await waitFor(() => expect(document.querySelector(".is-moving")).toBeTruthy());
        fireEvent.drop(document.querySelector('[data-stage="Ready-to-Pickup"]'));

        await waitFor(() => expect(updateRowQueueOrder).toHaveBeenCalled());
        const sent = JSON.parse(updateRowQueueOrder.mock.calls[0][0].queueOrder);
        expect(sent).toEqual(["Created", "Ready-to-Pickup", "Printing", "Done"]);
    });

    // The head is the handle. A draggable column would swallow the drag of every card in it.
    it("does not make the whole column draggable", async () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        expect(document.querySelector('[data-stage="Created"]').getAttribute("draggable")).toBeNull();
        expect(headOf("Created").getAttribute("draggable")).toBe("true");
    });

    it("turns a column head into an input on double click", async () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        fireEvent.doubleClick(headOf("Printing"));
        expect(screen.getByLabelText(/Rename Printing/i)).toBeTruthy();
    });

    // A stage is identified by its NAME - a card sits at "Printing", not at index 1 - so a
    // rename that left the cards behind would strand every one of them off-pipeline.
    it("carries the cards on a stage across a rename, and moves them first", async () => {
        render(
            <JobBoardOverlay
                job={job([card({ queue: "Printing" })])}
                onClose={vi.fn()}
                onChanged={vi.fn()}
            />
        );
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());

        fireEvent.doubleClick(headOf("Printing"));
        const input = screen.getByLabelText(/Rename Printing/i);
        fireEvent.change(input, { target: { value: "Print" } });
        fireEvent.keyDown(input, { key: "Enter" });

        await waitFor(() => expect(setRowQueue).toHaveBeenCalled());
        expect(setRowQueue.mock.calls[0][0].get("queue")).toBe("Print");

        await waitFor(() => expect(updateRowQueueOrder).toHaveBeenCalled());
        expect(JSON.parse(updateRowQueueOrder.mock.calls[0][0].queueOrder)).toContain("Print");
    });

    it("refuses a rename onto a stage the job already has", async () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        fireEvent.doubleClick(headOf("Printing"));
        const input = screen.getByLabelText(/Rename Printing/i);
        fireEvent.change(input, { target: { value: "Done" } });
        fireEvent.keyDown(input, { key: "Enter" });
        await waitFor(() => expect(notifyError).toHaveBeenCalledWith(expect.stringMatching(/already has a stage/i)));
        expect(updateRowQueueOrder).not.toHaveBeenCalled();
    });

    it("abandons a rename on Escape", async () => {
        render(<JobBoardOverlay job={job([card()])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        fireEvent.doubleClick(headOf("Printing"));
        fireEvent.keyDown(screen.getByLabelText(/Rename Printing/i), { key: "Escape" });
        expect(screen.queryByLabelText(/Rename Printing/i)).toBeNull();
        expect(updateRowQueueOrder).not.toHaveBeenCalled();
    });
});

// The rule refuses a move until a card has somebody on it, so a shop with no employee records
// cannot move anything at all - the picker opens, is empty, and there is nowhere to go. Being
// sent to another screen to come back and start again is what makes a rule feel like a bug.
describe("adding someone without leaving the board", () => {
    const noEmployees = () =>
        lookupPeople.mockResolvedValue({ code: 200, data: [{ _id: "s1", name: "Bolt Vinyls", type: "Supplier" }] });

    it("says what to do when there is nobody to assign", async () => {
        noEmployees();
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} />);
        await waitFor(() => expect(screen.getByText(/No employees yet/i)).toBeTruthy());
    });

    // The form, not an outright create: an employee needs an email and a password to be a
    // person who can sign in, and inventing an account with neither produces someone who
    // exists on a board and nowhere else.
    it("opens the new-employee form with the typed name already in it", async () => {
        noEmployees();
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} onChanged={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());

        fireEvent.click(screen.getByText(/unassigned/i));
        const search = await screen.findByPlaceholderText(/Search unassigned/i);
        fireEvent.change(search, { target: { value: "Sunil" } });
        fireEvent.click(screen.getByRole("button", { name: /Create "Sunil"/i }));

        await waitFor(() => expect(screen.getByLabelText(/^name$/i)).toBeTruthy());
        expect(screen.getByLabelText(/^name$/i).value).toBe("Sunil");
        expect(screen.getByLabelText(/^email$/i)).toBeTruthy();
        expect(screen.getByLabelText(/^password$/i)).toBeTruthy();
        // Nothing is created until the form is submitted.
        expect(createPerson).not.toHaveBeenCalled();
    });

    it("does nothing on an empty name", async () => {
        noEmployees();
        render(<JobBoardOverlay job={job([card({ employee_id: null })])} onClose={vi.fn()} />);
        await waitFor(() => expect(lookupPeople).toHaveBeenCalled());
        fireEvent.click(screen.getByText(/unassigned/i));
        fireEvent.click(await screen.findByRole("button", { name: /Create new/i }));
        await waitFor(() => expect(createPerson).not.toHaveBeenCalled());
    });
});

// --- A board that cannot be rearranged ----------------------------------------------------
// Part-invoiced job-ids reach the board now so their remaining work can be seen. The API
// freezes stages for the WHOLE job the moment one row is billed, so every drag here would
// travel and come back refused.
describe("a frozen job-id", () => {
    const frozenJob = {
        _id: "j1",
        challanNumber: "MG/26-27/1",
        rows: [{ _id: "r1", rowId: "MG-1", material: "Vinyl", queue: "Created" }],
        lock: { invoiced: true, unlocked: false, canEditQueue: false, canEditValues: false },
    };
    const rect = { top: 10, left: 10, width: 100, height: 80 };

    it("does not offer its cards as draggable", () => {
        const { container } = render(<JobBoardOverlay job={frozenJob} rect={rect} onClose={() => {}} />);
        const chip = container.querySelector(".job-board-chip");
        expect(chip.getAttribute("draggable")).toBe("false");
        expect(chip.className).toContain("is-frozen");
    });

    it("says why, rather than leaving it to be discovered on a refused drop", () => {
        render(<JobBoardOverlay job={frozenJob} rect={rect} onClose={() => {}} />);
        expect(screen.getByText(/unlock to edit or move its cards/i)).toBeTruthy();
    });

    // The same route the table's detail modal uses, so the two cannot disagree about what
    // unlocking means.
    it("offers unlock, and sends the opposite of what the job currently is", async () => {
        render(<JobBoardOverlay job={frozenJob} rect={rect} onClose={() => {}} />);
        fireEvent.click(screen.getByText("Unlock"));

        await waitFor(() => expect(unlockJob).toHaveBeenCalled());
        expect(unlockJob.mock.calls[0][0].get("unlocked")).toBe("true");
        expect(unlockJob.mock.calls[0][0].get("job_id")).toBe("j1");
    });

    it("offers Lock once unlocked", () => {
        const unlocked = { ...frozenJob, lock: { ...frozenJob.lock, unlocked: true, canEditValues: true, canEditQueue: true } };
        render(<JobBoardOverlay job={unlocked} rect={rect} onClose={() => {}} />);
        expect(screen.getByText("Lock")).toBeTruthy();
        expect(screen.getByText(/cards can be edited and moved/i)).toBeTruthy();
    });

    // Unlocking re-opens the stages too now. A job-id is routinely billed while a card on it
    // still has finishing to do, and refusing the move left no way to record work being done.
    it("makes the cards draggable again once unlocked", () => {
        const unlocked = {
            ...frozenJob,
            lock: { ...frozenJob.lock, unlocked: true, canEditValues: true, canEditQueue: true },
        };
        const { container } = render(<JobBoardOverlay job={unlocked} rect={rect} onClose={() => {}} />);
        const chip = container.querySelector(".job-board-chip");
        expect(chip.getAttribute("draggable")).toBe("true");
        expect(chip.className).not.toContain("is-frozen");
    });

    it("shows no lock control at all on a job-id that was never billed", () => {
        const open = { ...frozenJob, lock: { invoiced: false, canEditQueue: true, canEditValues: true } };
        render(<JobBoardOverlay job={open} rect={rect} onClose={() => {}} />);
        expect(screen.queryByText("Unlock")).toBeNull();
        expect(screen.queryByText("Lock")).toBeNull();
    });

    it("leaves an editable job-id draggable and unexplained", () => {
        const open = { ...frozenJob, lock: { invoiced: false, canEditQueue: true, canEditValues: true } };
        const { container } = render(<JobBoardOverlay job={open} rect={rect} onClose={() => {}} />);
        const chip = container.querySelector(".job-board-chip");
        expect(chip.getAttribute("draggable")).toBe("true");
        expect(chip.className).not.toContain("is-frozen");
        expect(screen.queryByText(/stages stay fixed/i)).toBeNull();
    });
});

// --- A card at a stage the job-id does not have -------------------------------------------
// JOB/26-27/000005 in the field: pipeline Created/Printing/Packing/Done, its one card standing
// at Ready-to-Pickup. boardColumns opened a fifth column for it, and a board hard-capped at
// four columns wide parked that column four pixels past its own right edge - four empty
// columns on screen and the only card invisible behind a scroll nothing advertises.
describe("a card at an off-pipeline stage", () => {
    const strayJob = {
        _id: "j5",
        challanNumber: "JOB/26-27/000005",
        queueOrder: ["Created", "Printing", "Packing", "Done"],
        rows: [{ _id: "r1", rowId: "J5-1", material: "Laminated Vinyl", queue: "Ready-to-Pickup" }],
        lock: { canEditQueue: true, canEditValues: true },
    };
    const rect = { top: 10, left: 10, width: 100, height: 80 };

    it("still renders the card, in a column of its own", () => {
        const { container } = render(<JobBoardOverlay job={strayJob} rect={rect} onClose={() => {}} />);
        const stray = container.querySelector('.job-board-column[data-stage="Ready-to-Pickup"]');
        expect(stray).toBeTruthy();
        expect(stray.classList.contains("is-off-pipeline")).toBe(true);
        expect(stray.querySelectorAll(".job-board-chip").length).toBe(1);
    });

    // The fix for the invisibility: the board is as wide as the columns it actually built.
    it("widens itself to the number of columns it built", () => {
        const { container } = render(<JobBoardOverlay job={strayJob} rect={rect} onClose={() => {}} />);
        const ov = container.querySelector(".job-board-overlay");
        expect(ov.style.getPropertyValue("--board-cols")).toBe("5");
    });

    it("stays at four for an ordinary job-id", () => {
        const plain = {
            ...strayJob,
            rows: [{ _id: "r1", rowId: "J5-1", material: "Vinyl", queue: "Printing" }],
        };
        const { container } = render(<JobBoardOverlay job={plain} rect={rect} onClose={() => {}} />);
        expect(container.querySelector(".job-board-overlay").style.getPropertyValue("--board-cols")).toBe("4");
    });

    // Width alone is not enough: a narrow screen still scrolls, so the board says it too.
    it("names the stage its card is standing at", () => {
        render(<JobBoardOverlay job={strayJob} rect={rect} onClose={() => {}} />);
        expect(screen.getByText(/1 card is at Ready-to-Pickup, which is not a stage on this job/i)).toBeTruthy();
    });

    it("says nothing when every card is on the pipeline", () => {
        const plain = {
            ...strayJob,
            rows: [{ _id: "r1", rowId: "J5-1", material: "Vinyl", queue: "Printing" }],
        };
        render(<JobBoardOverlay job={plain} rect={rect} onClose={() => {}} />);
        expect(screen.queryByText(/is not a stage on this job/i)).toBeNull();
    });

    // An empty stray column is a stage that was renamed or reordered away; there is no card to
    // account for, so there is nothing to announce.
    it("counts cards, not columns", () => {
        const two = {
            ...strayJob,
            rows: [
                { _id: "r1", rowId: "J5-1", material: "A", queue: "Ready-to-Pickup" },
                { _id: "r2", rowId: "J5-2", material: "B", queue: "Quality Check" },
            ],
        };
        render(<JobBoardOverlay job={two} rect={rect} onClose={() => {}} />);
        expect(screen.getByText(/2 cards are at stages not on this job/i)).toBeTruthy();
    });
});
