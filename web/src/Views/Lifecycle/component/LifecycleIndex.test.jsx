import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

// What this page does TODAY, pinned before a second view is added to it.
//
// This file exists because LifecycleIndex is the one file the job-board work modifies, and was
// the one file in this folder with no test - sixty-three covered the helpers, the dialogs and
// the backend, none the index. If any of these fail after the board lands, that is a regression
// rather than a decision: the board is a view beside this one, not a change to it.
//
// Written against the current code and expected to pass immediately. A characterisation test
// written after a change only describes whatever the change did.

const listJobs = vi.fn();
vi.mock("../lifecycle_backend", () => ({
    lifecycleBackend: {
        listJobs: (...a) => listJobs(...a),
        createJob: vi.fn(),
        updateJob: vi.fn(),
        deleteJob: vi.fn(),
        setRowQueue: vi.fn(),
    },
}));

// A .js file containing JSX, which Vitest will not transform - mocked the way
// AnalyticsIndex.test.jsx already mocks it.
vi.mock("../../../Common/Header/LiteHeader", () => ({ default: () => null }));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));
vi.mock("../../../Common/whatsapp_backend", () => ({ whatsappBackend: { getConfig: vi.fn(() => Promise.resolve({ data: {} })) } }));
vi.mock("../../../Common/jobStore", () => ({ useJobCreated: () => {} }));
vi.mock("../../../Common/undoDelete", () => ({ useUndoDelete: () => ({ scheduleDelete: vi.fn() }) }));

const can = vi.fn(() => true);
vi.mock("../../../Common/access", () => ({
    can: (...a) => can(...a),
    hasAccess: () => true,
    ACTIONS: ["view", "create", "delete"],
}));

import LifecycleIndex from "./LifecycleIndex";

const job = (over = {}) => ({
    _id: "j1",
    challanNumber: "MG/26-27/1",
    client_id: { clientFirm: "Acme Signs", clientName: "Priya", clientPhone: "919000000001" },
    rows: [{ _id: "r1", rowId: "MG/26-27/1-1", material: "Vinyl", qty: 2, rate: 100, queue: "Created" }],
    total: 200,
    createdAt: "2026-09-01T10:00:00Z",
    lock: { invoiced: false, canEditValues: true, canEditQueue: true, canDeleteJob: true },
    ...over,
});

const renderPage = (url = "/") =>
    render(
        <MemoryRouter initialEntries={[url]}>
            <LifecycleIndex />
        </MemoryRouter>
    );

beforeEach(() => {
    vi.clearAllMocks();
    // The Table/Board choice is remembered per list now, and jsdom shares one localStorage
    // across a file - so a test that clicks Board leaves the NEXT test starting on the board,
    // looking at a page it never asked for.
    window.localStorage.clear();
    can.mockReturnValue(true);
    listJobs.mockResolvedValue({
        code: 200,
        data: [
            job(),
            job({
                _id: "j2",
                challanNumber: "MG/26-27/2",
                client_id: { clientFirm: "Bolt Print", clientName: "Ravi", clientPhone: "919000000002" },
                total: 900,
            }),
        ],
    });
});

describe("LifecycleIndex as it stands today", () => {
    it("lists the jobs it is given", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        expect(screen.getByText("MG/26-27/2")).toBeTruthy();
    });

    it("narrows the list by search", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        fireEvent.change(screen.getByPlaceholderText(/Search job-id/i), { target: { value: "Bolt" } });
        expect(screen.queryByText("MG/26-27/1")).toBeNull();
        expect(screen.getByText("MG/26-27/2")).toBeTruthy();
    });

    // Scoped to the tablist: "Invoiced" is also a status badge in the table, so an unscoped
    // query matches two elements and says nothing about the filter.
    it("offers the category filters", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        // Scoped by the strip's own container rather than by role: the board work adds a second
        // tablist to this page, and a role query would then match two elements and fail for a
        // reason that has nothing to do with the category filters.
        const tabs = within(document.querySelector(".stats-compact"));
        ["All", "In-Progress", "Quoted", "Ready For Invoice", "Invoiced"].forEach((c) =>
            expect(tabs.getByText(c)).toBeTruthy()
        );
    });

    // The create control is permissioned. Someone who may not create must not be offered it.
    it("offers creating a job only to someone who may", async () => {
        const { unmount } = renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        // The exact label, so the absence assertion below cannot pass merely by being stricter
        // than the button's real accessible name.
        expect(screen.getByRole("button", { name: /create job/i })).toBeTruthy();
        unmount();

        can.mockImplementation((feature, action) => !(feature === "lifecycle" && action === "create"));
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        expect(screen.queryByRole("button", { name: /create job/i })).toBeNull();
    });
});

// The board, as a second view. Everything above must keep passing unchanged - that is what
// says the table view did not move.
describe("the Table / Board switch", () => {
    it("opens on the table view", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        expect(screen.getByRole("tab", { name: "Table" }).getAttribute("aria-selected")).toBe("true");
        expect(screen.getByRole("tab", { name: "Board" }).getAttribute("aria-selected")).toBe("false");
        expect(document.querySelector(".stats-compact")).toBeTruthy();
    });

    // Search only on the board - the category StatCards belong to the table view.
    it("swaps the table and its category filters for the grid", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        fireEvent.click(screen.getByRole("tab", { name: "Board" }));
        expect(document.querySelector(".stats-compact")).toBeNull();
        expect(screen.getByPlaceholderText(/Search job-id or product/i)).toBeTruthy();
        expect(screen.getByText("MG/26-27/1")).toBeTruthy();
    });

    // The fixture carries invoiceState because the API does (routes/Lifecycle.js:127). The
    // board reads that now rather than lock.canEditQueue, which the API turns off as soon as
    // ONE row is billed.
    it("keeps a fully invoiced job off the board while the table still lists it", async () => {
        listJobs.mockResolvedValue({
            code: 200,
            data: [
                job(),
                job({
                    _id: "j3",
                    challanNumber: "MG/26-27/3",
                    invoiceState: "invoiced",
                    lock: { invoiced: true, canEditValues: false, canEditQueue: false, canDeleteJob: false },
                }),
            ],
        });
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/3")).toBeTruthy());
        fireEvent.click(screen.getByRole("tab", { name: "Board" }));
        expect(screen.queryByText("MG/26-27/3")).toBeNull();
        expect(screen.getByText("MG/26-27/1")).toBeTruthy();
    });

    // The point of the change: a job-id with one row billed and the rest still in production
    // belongs on the board, because those rows ARE the work. It cannot be rearranged - the API
    // freezes stages for the whole job - but it can be seen.
    it("keeps a part-invoiced job on the board", async () => {
        listJobs.mockResolvedValue({
            code: 200,
            data: [
                job({
                    _id: "j4",
                    challanNumber: "MG/26-27/4",
                    invoiceState: "partial",
                    invoicedRows: 1,
                    lock: { invoiced: true, canEditValues: false, canEditQueue: false, canDeleteJob: false },
                }),
            ],
        });
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/4")).toBeTruthy());
        fireEvent.click(screen.getByRole("tab", { name: "Board" }));
        expect(screen.getByText("MG/26-27/4")).toBeTruthy();
    });
});

// The dashboard's "still open" alert links here with ?open=1. Without the filter it landed on
// the whole list with nothing to say which job-ids the sentence had been counting.
describe("arriving from the dashboard's still-open alert", () => {
    const finished = () =>
        job({
            _id: "j3",
            challanNumber: "MG/26-27/3",
            client_id: { clientFirm: "Cobalt Media", clientName: "Sam", clientPhone: "919000000003" },
            rows: [{ _id: "r3", rowId: "MG/26-27/3-1", material: "Foam", qty: 1, rate: 50, queue: "Done" }],
        });

    beforeEach(() => {
        listJobs.mockResolvedValue({ code: 200, data: [job(), finished()] });
    });

    it("shows every job-id without the flag", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        expect(screen.getByText("MG/26-27/3")).toBeTruthy();
    });

    it("hides job-ids whose cards have all reached Done", async () => {
        renderPage("/?open=1");
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        expect(screen.queryByText("MG/26-27/3")).toBeNull();
    });

    // A shorter list with nothing on screen to say why is the worst way to arrive from a link.
    it("says the filter is on, and counts it off every job rather than the filtered list", async () => {
        renderPage("/?open=1");
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        const chip = screen.getByRole("button", { name: /still-open filter/i });
        expect(chip.textContent).toContain("Still open");
        expect(chip.textContent).toContain("1");
    });

    it("puts the finished job-id back when the chip is pressed", async () => {
        renderPage("/?open=1");
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        fireEvent.click(screen.getByRole("button", { name: /still-open filter/i }));
        await waitFor(() => expect(screen.getByText("MG/26-27/3")).toBeTruthy());
        expect(screen.queryByRole("button", { name: /still-open filter/i })).toBeNull();
    });

    // The board reads a different list from the table. Switching view must not quietly widen
    // what is being looked at back to everything.
    it("carries the filter onto the board", async () => {
        renderPage("/?open=1");
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeTruthy());
        fireEvent.click(screen.getByRole("tab", { name: "Board" }));
        expect(screen.getByText("MG/26-27/1")).toBeTruthy();
        expect(screen.queryByText("MG/26-27/3")).toBeNull();
    });
});
