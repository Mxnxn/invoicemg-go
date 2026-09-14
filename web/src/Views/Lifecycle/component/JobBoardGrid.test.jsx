import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import JobBoardGrid from "./JobBoardGrid";

const job = (over = {}) => ({
    _id: "j1",
    challanNumber: "MG/26-27/1",
    total: 12400,
    createdAt: "2026-09-01T10:00:00Z",
    lock: { canEditQueue: true, canEditValues: true },
    rows: [
        { _id: "r1", material: "Vinyl banner", queue: "Created" },
        { _id: "r2", material: "Foam board", queue: "Printing" },
        { _id: "r3", material: "Acrylic sheet", queue: "Printing" },
    ],
    ...over,
});

describe("JobBoardGrid", () => {
    it("shows the job-id, its total and how many jobs it has", () => {
        render(<JobBoardGrid jobs={[job()]} onOpen={vi.fn()} />);
        expect(screen.getByText("MG/26-27/1")).toBeTruthy();
        expect(screen.getByText(/12,400/)).toBeTruthy();
        expect(screen.getByText(/3 jobs/)).toBeTruthy();
    });

    // "job", not "card": the chips inside a job-id are cards to this board and jobs to the
    // shop reading it.
    it("says 1 job rather than 1 jobs", () => {
        render(<JobBoardGrid jobs={[job({ rows: [{ _id: "r1", material: "Vinyl" }] })]} onOpen={vi.fn()} />);
        expect(screen.getByText(/· 1 job$/)).toBeTruthy();
        expect(screen.queryByText(/1 jobs/)).toBeNull();
    });

    // The same pairing the board's chips use - three badges reading "Vinyl, Vinyl, Vinyl" say
    // less than three carrying their sizes.
    it("shows each product with its size", () => {
        const rows = [
            { _id: "r1", material: "Vinyl banner", length: "12", width: "4", qty: 1 },
            { _id: "r2", material: "Foam board", hasDimensions: false, qty: 9 },
        ];
        render(<JobBoardGrid jobs={[job({ rows })]} onOpen={vi.fn()} />);
        expect(screen.getByText("12 x 4")).toBeTruthy();
        expect(screen.getByText("Qty 9")).toBeTruthy();
    });

    // Searching is by name. Typing "Vinyl" should not have to compete with a size.
    it("still searches by product name", () => {
        const rows = [{ _id: "r1", material: "Vinyl banner", length: "12", width: "4" }];
        render(<JobBoardGrid jobs={[job({ rows }), job({ _id: "j2", challanNumber: "MG/2", rows: [{ _id: "x", material: "Canvas" }] })]} onOpen={vi.fn()} />);
        fireEvent.change(screen.getByPlaceholderText(/Search/i), { target: { value: "vinyl" } });
        expect(screen.getByText("MG/26-27/1")).toBeTruthy();
        expect(screen.queryByText("MG/2")).toBeNull();
    });

    // A fixed cap, not a measurement: measuring real overflow would make an equal-sized grid
    // render differently depending on how long someone's product names happen to be.
    it("shows the first three products and collapses the rest", () => {
        const rows = ["A", "B", "C", "D", "E"].map((m, i) => ({ _id: `r${i}`, material: m }));
        render(<JobBoardGrid jobs={[job({ rows })]} onOpen={vi.fn()} />);
        expect(screen.getByText("A")).toBeTruthy();
        expect(screen.getByText("C")).toBeTruthy();
        expect(screen.queryByText("D")).toBeNull();
        expect(screen.getByText("+2 more")).toBeTruthy();
    });

    it("does not collapse anything when there are exactly three", () => {
        render(<JobBoardGrid jobs={[job()]} onOpen={vi.fn()} />);
        expect(screen.queryByText(/more$/)).toBeNull();
    });

    // Drawn only when locked. An icon on every card is decoration, and the common case
    // deserves no ink.
    it("marks a frozen job and leaves an editable one unmarked", () => {
        const { rerender } = render(<JobBoardGrid jobs={[job()]} onOpen={vi.fn()} />);
        expect(screen.queryByLabelText(/locked/i)).toBeNull();
        rerender(<JobBoardGrid jobs={[job({ lock: { canEditQueue: false } })]} onOpen={vi.fn()} />);
        expect(screen.getByLabelText(/locked/i)).toBeTruthy();
    });

    it("searches the job-id and the products", () => {
        const jobs = [
            job(),
            job({ _id: "j2", challanNumber: "MG/26-27/2", rows: [{ _id: "x", material: "Canvas" }] }),
        ];
        render(<JobBoardGrid jobs={jobs} onOpen={vi.fn()} />);

        fireEvent.change(screen.getByPlaceholderText(/Search/i), { target: { value: "Canvas" } });
        expect(screen.queryByText("MG/26-27/1")).toBeNull();
        expect(screen.getByText("MG/26-27/2")).toBeTruthy();

        fireEvent.change(screen.getByPlaceholderText(/Search/i), { target: { value: "26-27/1" } });
        expect(screen.getByText("MG/26-27/1")).toBeTruthy();
    });

    // The rectangle is what lets the overlay expand out of this card rather than fade in on
    // the spot, so it has to travel with the job.
    it("hands back the job and the card's rectangle when opened", () => {
        const onOpen = vi.fn();
        render(<JobBoardGrid jobs={[job()]} onOpen={onOpen} />);
        fireEvent.click(screen.getByRole("button", { name: /MG\/26-27\/1/ }));
        expect(onOpen).toHaveBeenCalled();
        const [passedJob, rect] = onOpen.mock.calls[0];
        expect(passedJob._id).toBe("j1");
        expect(rect).toHaveProperty("top");
    });

    it("says so when there is nothing, and says something different when a search matches nothing", () => {
        const { rerender } = render(<JobBoardGrid jobs={[]} onOpen={vi.fn()} />);
        expect(screen.getByText(/No job-ids/i)).toBeTruthy();

        rerender(<JobBoardGrid jobs={[job()]} onOpen={vi.fn()} />);
        fireEvent.change(screen.getByPlaceholderText(/Search/i), { target: { value: "zzzz" } });
        expect(screen.getByText(/matches that search/i)).toBeTruthy();
    });

    it("newest first", () => {
        const jobs = [
            job({ _id: "old", challanNumber: "MG/26-27/OLD", createdAt: "2026-01-01T00:00:00Z" }),
            job({ _id: "new", challanNumber: "MG/26-27/NEW", createdAt: "2026-09-01T00:00:00Z" }),
        ];
        render(<JobBoardGrid jobs={jobs} onOpen={vi.fn()} />);
        const rendered = screen.getAllByRole("button").map((b) => b.textContent);
        expect(rendered[0]).toContain("MG/26-27/NEW");
    });
});

// --- Billing from the board ------------------------------------------------------------
// The board had no answer for "this one is ready, invoice it" - the one thing the table could
// do that it could not.
describe("selecting job-ids to invoice", () => {
    const selectionStub = (selectedIds = []) => {
        const set = new Set(selectedIds);
        return {
            isSelected: (id) => set.has(id),
            toggle: vi.fn(),
            count: set.size,
            clear: vi.fn(),
        };
    };

    const ready = { _id: "a", challanNumber: "MG/1", rows: [], readyForInvoice: true };
    const busy = { _id: "b", challanNumber: "MG/2", rows: [], readyForInvoice: false };

    it("gives a box only to a job-id that can actually be billed", () => {
        render(
            <JobBoardGrid
                jobs={[ready, busy]}
                selection={selectionStub()}
                isInvoiceable={(j) => Boolean(j.readyForInvoice)}
            />
        );
        expect(screen.getByLabelText("Select MG/1")).toBeTruthy();
        expect(screen.queryByLabelText("Select MG/2")).toBeNull();
    });

    it("offers no boxes at all when the view passes no selection", () => {
        render(<JobBoardGrid jobs={[ready, busy]} />);
        expect(screen.queryByLabelText("Select MG/1")).toBeNull();
    });

    it("toggles the job it belongs to", () => {
        const selection = selectionStub();
        render(<JobBoardGrid jobs={[ready]} selection={selection} isInvoiceable={() => true} />);
        fireEvent.click(screen.getByLabelText("Select MG/1"));
        expect(selection.toggle).toHaveBeenCalledWith("a");
    });

    // The box is a sibling of the card, not a child: the card is a <button>, and a checkbox
    // inside one is neither valid nor clickable - the button eats the event.
    it("keeps the checkbox outside the card button", () => {
        const { container } = render(
            <JobBoardGrid jobs={[ready]} selection={selectionStub()} isInvoiceable={() => true} />
        );
        const box = screen.getByLabelText("Select MG/1");
        expect(box.closest("button.job-board-card")).toBeNull();
        expect(container.querySelector(".job-board-card-wrap")).toBeTruthy();
    });

    it("marks the wrapper so the whole card can show as selected", () => {
        const { container } = render(
            <JobBoardGrid jobs={[ready]} selection={selectionStub(["a"])} isInvoiceable={() => true} />
        );
        expect(container.querySelector(".job-board-card-wrap.is-selected")).toBeTruthy();
    });

    // Opening a job-id and selecting it for billing are different intents.
    it("does not open the board when the box is ticked", () => {
        const onOpen = vi.fn();
        render(
            <JobBoardGrid jobs={[ready]} selection={selectionStub()} isInvoiceable={() => true} onOpen={onOpen} />
        );
        fireEvent.click(screen.getByLabelText("Select MG/1"));
        expect(onOpen).not.toHaveBeenCalled();
    });
});

// --- Raising a job-id from the board ----------------------------------------------------
describe("the create button", () => {
    const job = { _id: "a", challanNumber: "MG/1", rows: [] };

    it("opens the view's own create modal", () => {
        const onCreate = vi.fn();
        render(<JobBoardGrid jobs={[job]} onCreate={onCreate} />);
        fireEvent.click(screen.getByText("Create Job"));
        expect(onCreate).toHaveBeenCalled();
    });

    // Permission is the caller's to decide - it passes no handler when the user may not
    // create, exactly as the table's own button is gated.
    it("is absent when the view offers no handler", () => {
        render(<JobBoardGrid jobs={[job]} />);
        expect(screen.queryByText("Create Job")).toBeNull();
    });

    it("sits on the search row", () => {
        const { container } = render(<JobBoardGrid jobs={[job]} onCreate={() => {}} />);
        const head = container.querySelector(".job-board-head");
        expect(head.querySelector(".job-board-create")).toBeTruthy();
        expect(head.querySelector("input")).toBeTruthy();
    });

    // An empty board is exactly when someone needs it most.
    it("stays visible when nothing matches the search", () => {
        render(<JobBoardGrid jobs={[]} onCreate={() => {}} />);
        expect(screen.getByText("Create Job")).toBeTruthy();
    });
});

// --- Board paging and the ready edge ------------------------------------------------------
describe("board paging", () => {
    const many = (n) =>
        Array.from({ length: n }, (_, i) => ({
            _id: `j${i}`,
            challanNumber: `MG/${i}`,
            rows: [],
            createdAt: new Date(2026, 0, n - i).toISOString(),
        }));

    // Fixed at 15 for this view rather than Appearance's rows-per-page, which is about table
    // rows. If the setting leaked back in, this count would follow it.
    it("shows 15 cards to a page", () => {
        const { container } = render(<JobBoardGrid jobs={many(20)} />);
        expect(container.querySelectorAll(".job-board-card").length).toBe(15);
    });

    it("does not page at all below 15", () => {
        const { container } = render(<JobBoardGrid jobs={many(9)} />);
        expect(container.querySelectorAll(".job-board-card").length).toBe(9);
    });

    it("keeps the pager clear of the last row of cards", () => {
        const { container } = render(<JobBoardGrid jobs={many(20)} />);
        expect(container.querySelector(".job-board-pagination")).toBeTruthy();
    });
});

describe("the ready-for-invoice edge", () => {
    const mk = (over) => ({ _id: "a", challanNumber: "MG/1", rows: [], ...over });

    it("marks a job-id that is ready to bill", () => {
        const { container } = render(<JobBoardGrid jobs={[mk({ readyForInvoice: true })]} />);
        expect(container.querySelector(".job-board-card-wrap.is-ready")).toBeTruthy();
    });

    it("leaves an unfinished job-id unmarked", () => {
        const { container } = render(<JobBoardGrid jobs={[mk({ readyForInvoice: false })]} />);
        expect(container.querySelector(".job-board-card-wrap.is-ready")).toBeNull();
    });

    // Already billed is not "ready to bill" - the edge would be inviting an action that is
    // already done.
    it("leaves an invoiced job-id unmarked even if the flag lingers", () => {
        const { container } = render(
            <JobBoardGrid jobs={[mk({ readyForInvoice: true, invoiceState: "invoiced" })]} />
        );
        expect(container.querySelector(".job-board-card-wrap.is-ready")).toBeNull();
    });

    it("can be both ready and selected", () => {
        const selection = { isSelected: () => true, toggle: () => {}, count: 1, clear: () => {} };
        const { container } = render(
            <JobBoardGrid jobs={[mk({ readyForInvoice: true })]} selection={selection} isInvoiceable={() => true} />
        );
        const wrap = container.querySelector(".job-board-card-wrap");
        expect(wrap.classList.contains("is-ready")).toBe(true);
        expect(wrap.classList.contains("is-selected")).toBe(true);
    });
});

// --- What the board carries ---------------------------------------------------------------
// The filter moved from lock.canEditQueue to "not fully invoiced". canEditQueue is false the
// moment ONE row is billed, so the old rule hid job-ids whose remaining rows were still the
// work in front of everyone.
describe("part-invoiced job-ids", () => {
    const partial = {
        _id: "p",
        challanNumber: "MG/PART",
        invoiceState: "partial",
        invoicedRows: 2,
        rows: [{}, {}, {}, {}, {}, {}],
        lock: { canEditQueue: false },
    };

    it("says how much is billed rather than merely that it is locked", () => {
        render(<JobBoardGrid jobs={[partial]} />);
        expect(screen.getByText("2/6 billed")).toBeTruthy();
    });

    it("marks it locked so the frozen stages are not a surprise", () => {
        const { container } = render(<JobBoardGrid jobs={[partial]} />);
        expect(container.querySelector(".job-board-card-lock")).toBeTruthy();
    });

    // Frozen is not the same as billable: it has un-billed rows, but its stages cannot move.
    it("is not offered an invoice checkbox by a view that says it is not billable", () => {
        const selection = { isSelected: () => false, toggle: () => {}, count: 0, clear: () => {} };
        render(<JobBoardGrid jobs={[partial]} selection={selection} isInvoiceable={() => false} />);
        expect(screen.queryByLabelText("Select MG/PART")).toBeNull();
    });

    it("gets no ready-to-bill edge", () => {
        const { container } = render(<JobBoardGrid jobs={[partial]} />);
        expect(container.querySelector(".job-board-card-wrap.is-ready")).toBeNull();
    });

    it("still shows an unlocked job-id without a lock", () => {
        const open = { _id: "o", challanNumber: "MG/OPEN", rows: [{}], lock: { canEditQueue: true } };
        const { container } = render(<JobBoardGrid jobs={[open]} />);
        expect(container.querySelector(".job-board-card-lock")).toBeNull();
    });
});
