import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const detail = vi.fn();
const approve = vi.fn();
const addNote = vi.fn();
const editNote = vi.fn();
vi.mock("../purchaseOrder_backend", () => ({
    purchaseOrderBackend: {
        detail: (...a) => detail(...a),
        approve: (...a) => approve(...a),
        revoke: vi.fn(),
        addNote: (...a) => addNote(...a),
        editNote: (...a) => editNote(...a),
    },
}));
// The panel's own behaviour is what is under test, not the slide-in animation or its portal.
vi.mock("../../../Common/SlideOverlay/SlideOverlay", () => ({ default: ({ children }) => <div>{children}</div> }));

import PoDetailSidebar from "./PoDetailSidebar";

const base = {
    po: {
        _id: "po1",
        poNumber: "PO/26-27/000001",
        date: "2026-09-01",
        total: 5000,
        supplier_id: { name: "Ravi", firm: "Ravi Papers" },
        approval: { state: "approved", approvedByName: "Meera", approvedAt: "2026-09-02T10:00:00Z" },
        purchaseInvoice_id: null,
        rows: [{ _id: "r1", material: "Vinyl", description: "front panel", qty: 2, rate: 100, unit: "sheet" }],
    },
    history: [
        {
            _id: "h1",
            action: "Approval revoked",
            actorName: "Arun",
            createdAt: "2026-09-03T09:00:00Z",
            changes: [{ field: "rows[1].qty", from: "2", to: "5" }],
            detail: "Price-bearing details changed after approval.",
        },
    ],
    notes: [],
};

beforeEach(() => {
    vi.clearAllMocks();
    detail.mockResolvedValue({ code: 200, data: base });
    addNote.mockResolvedValue({ code: 200 });
    editNote.mockResolvedValue({ code: 200 });
});

describe("PoDetailSidebar", () => {
    it("renders the order and its rows", async () => {
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByText("PO/26-27/000001")).toBeTruthy());
        expect(screen.getByText("Vinyl")).toBeTruthy();
    });

    it("renders history from the structured changes, not a pre-baked sentence", async () => {
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        // Built in the component FROM { field, from, to } - the API never sends this string.
        // The row is named by its product rather than its index: "rows[1].qty" tells the
        // person deciding whether to re-approve nothing at all.
        await waitFor(() => expect(screen.getAllByText(/Vinyl · Qty: 2 → 5/).length).toBeGreaterThan(0));
        expect(screen.getByText(/Arun/)).toBeTruthy();
    });

    it("names what changed when approval lapsed, so the fix is obvious", async () => {
        detail.mockResolvedValue({
            code: 200,
            data: { ...base, po: { ...base.po, approval: { state: "draft" } } },
        });
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByText(/approval lapsed/i)).toBeTruthy());
        expect(screen.getAllByText(/Vinyl · Qty/).length).toBeGreaterThan(0);
    });

    it("offers note editing only on the viewer's own notes inside the window", async () => {
        const now = Date.now();
        detail.mockResolvedValue({
            code: 200,
            data: {
                ...base,
                notes: [
                    { _id: "n1", authorId: "u1", authorName: "Me", text: "Mine, fresh", createdAt: new Date(now - 3600000).toISOString() },
                    { _id: "n2", authorId: "u1", authorName: "Me", text: "Mine, stale", createdAt: new Date(now - 90000000).toISOString() },
                    { _id: "n3", authorId: "u2", authorName: "Someone", text: "Not mine", createdAt: new Date(now).toISOString() },
                ],
            },
        });
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByText("Mine, fresh")).toBeTruthy());
        // Exactly one editable note: own AND inside 24h. Offering an edit the server will
        // refuse is worse than not offering it.
        expect(screen.getAllByRole("button", { name: /edit note/i })).toHaveLength(1);
    });

    it("surfaces an error rather than an empty panel", async () => {
        detail.mockRejectedValue(new Error("nope"));
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByText(/could not load/i)).toBeTruthy());
    });
    it("will not submit an empty note", async () => {
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByLabelText("Add a note")).toBeTruthy());
        const button = screen.getByRole("button", { name: "Add note" });
        expect(button.disabled).toBe(true);
        fireEvent.click(button);
        expect(addNote).not.toHaveBeenCalled();
    });

    it("adds a note and reloads so the history entry appears", async () => {
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByLabelText("Add a note")).toBeTruthy());
        fireEvent.change(screen.getByLabelText("Add a note"), { target: { value: "Chase the delivery date" } });
        fireEvent.click(screen.getByRole("button", { name: "Add note" }));
        await waitFor(() => expect(addNote).toHaveBeenCalled());
        const form = addNote.mock.calls[0][0];
        expect(form.get("po_id")).toBe("po1");
        expect(form.get("text")).toBe("Chase the delivery date");
        // Two loads: the initial one, and the reload that pulls in the new note and the
        // "Note added" history entry the server writes.
        await waitFor(() => expect(detail.mock.calls.length).toBe(2));
    });

    it("edits a note in place and sends its id", async () => {
        const now = Date.now();
        detail.mockResolvedValue({
            code: 200,
            data: {
                ...base,
                notes: [{ _id: "n1", authorId: "u1", authorName: "Me", text: "Mine, fresh", createdAt: new Date(now - 3600000).toISOString() }],
            },
        });
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByText("Mine, fresh")).toBeTruthy());

        fireEvent.click(screen.getByRole("button", { name: /edit note/i }));
        const box = screen.getByLabelText("Edit note");
        expect(box.value).toBe("Mine, fresh");
        fireEvent.change(box, { target: { value: "Mine, corrected" } });
        fireEvent.click(screen.getByRole("button", { name: "Save" }));

        await waitFor(() => expect(editNote).toHaveBeenCalled());
        const form = editNote.mock.calls[0][0];
        expect(form.get("note_id")).toBe("n1");
        expect(form.get("text")).toBe("Mine, corrected");
    });

    it("closes the editor on cancel without saving", async () => {
        const now = Date.now();
        detail.mockResolvedValue({
            code: 200,
            data: {
                ...base,
                notes: [{ _id: "n1", authorId: "u1", authorName: "Me", text: "Mine, fresh", createdAt: new Date(now - 3600000).toISOString() }],
            },
        });
        render(<PoDetailSidebar poId="po1" uid="u1" onClose={() => {}} onChanged={() => {}} />);
        await waitFor(() => expect(screen.getByText("Mine, fresh")).toBeTruthy());
        fireEvent.click(screen.getByRole("button", { name: /edit note/i }));
        fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
        expect(screen.queryByLabelText("Edit note")).toBeNull();
        expect(editNote).not.toHaveBeenCalled();
    });
});
