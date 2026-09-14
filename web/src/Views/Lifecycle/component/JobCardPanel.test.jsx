import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const updateJob = vi.fn();
vi.mock("../lifecycle_backend", () => ({ lifecycleBackend: { updateJob: (...a) => updateJob(...a) } }));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import JobCardPanel from "./JobCardPanel";

const card = { _id: "r1", rowId: "MG-1", material: "Vinyl", qty: 2, rate: 100, queue: "Printing", employee_id: "e1" };
const other = { _id: "r2", rowId: "MG-2", material: "Board", qty: 1, rate: 50, queue: "Created", employee_id: "e2" };
const job = { _id: "j1", challanNumber: "MG/26-27/1", rows: [card, other] };

// The chip it grew out of. Without one the panel simply appears, which is the no-origin path.
const RECT = { top: 200, left: 100, width: 240, height: 90 };

beforeEach(() => {
    vi.clearAllMocks();
    updateJob.mockResolvedValue({ code: 200, data: job });
});

describe("JobCardPanel", () => {
    it("opens on the card's own values", () => {
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />);
        expect(screen.getByLabelText(/product/i).value).toBe("Vinyl");
        expect(screen.getByLabelText(/quantity/i).value).toBe("2");
        expect(screen.getByLabelText(/rate/i).value).toBe("100");
    });

    // Dragging owns the stage and the card's own picker owns the assignee. Offering either here
    // would be two controls for one value, which is two controls that can disagree.
    it("does not offer the stage or the assignee", () => {
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />);
        expect(screen.queryByLabelText(/stage/i)).toBeNull();
        expect(screen.queryByLabelText(/assign/i)).toBeNull();
    });

    it("sends every row, with only this one changed", async () => {
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />);
        fireEvent.change(screen.getByLabelText(/quantity/i), { target: { value: "5" } });
        fireEvent.click(screen.getByRole("button", { name: /save/i }));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const sent = JSON.parse(updateJob.mock.calls[0][0].get("rows"));
        expect(sent).toHaveLength(2);
        expect(sent.find((r) => r._id === "r1").qty).toBe("5");
        // The untouched row travels exactly as it was.
        expect(sent.find((r) => r._id === "r2").material).toBe("Board");
    });

    // The property this whole component leans on: the server's normalizeRow carries only the
    // commercial fields, so stage and assignee survive the merge. Sending them unchanged is
    // what keeps that true from this end.
    it("sends the edited card's stage and assignee back unchanged", async () => {
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />);
        fireEvent.change(screen.getByLabelText(/rate/i), { target: { value: "250" } });
        fireEvent.click(screen.getByRole("button", { name: /save/i }));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const edited = JSON.parse(updateJob.mock.calls[0][0].get("rows")).find((r) => r._id === "r1");
        expect(edited.queue).toBe("Printing");
        expect(edited.employee_id).toBe("e1");
    });

    it("refuses to save an empty product", () => {
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />);
        fireEvent.change(screen.getByLabelText(/product/i), { target: { value: "  " } });
        fireEvent.click(screen.getByRole("button", { name: /save/i }));
        expect(updateJob).not.toHaveBeenCalled();
        expect(screen.getByText(/needs a product/i)).toBeTruthy();
    });

    // Closing shrinks back into the chip first, so onClose lands after the motion rather than
    // on the press.
    it("closes without saving", async () => {
        const onClose = vi.fn();
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={onClose} onSaved={vi.fn()} />);
        fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
        await waitFor(() => expect(onClose).toHaveBeenCalled(), { timeout: 1500 });
        expect(updateJob).not.toHaveBeenCalled();
    });

    // It grows out of the chip, the same motion the board makes. A modal arriving from nowhere
    // covers the thing you were looking at and has no evident relationship to it.
    it("grows out of the chip it was opened from", () => {
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />);
        const panel = document.querySelector(".job-card-panel");
        expect(panel).toBeTruthy();
        expect(panel.className).toMatch(/is-(measuring|placed|opening)/);
    });

    // Portalled to body: the board is a transformed element while it animates, and a fixed
    // descendant of a transformed ancestor is positioned against that ancestor rather than the
    // viewport - so nested inside, this panel would be dragged around by the board's motion.
    it("renders outside the board rather than inside it", () => {
        const host = document.createElement("div");
        host.className = "job-board-overlay";
        document.body.appendChild(host);
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />, {
            container: host,
        });
        expect(host.querySelector(".job-card-panel")).toBeNull();
        expect(document.body.querySelector(".job-card-panel")).toBeTruthy();
    });

    it("closes on Escape", async () => {
        const onClose = vi.fn();
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={onClose} onSaved={vi.fn()} />);
        fireEvent.keyDown(document, { key: "Escape" });
        await waitFor(() => expect(onClose).toHaveBeenCalled(), { timeout: 1500 });
    });
});

// The assignee decides whether a card can move at all, so it is first - asking for it after
// eight commercial fields is asking for it last and finding out first.
describe("JobCardPanel assigning", () => {
    const employees = [{ id: "e1", name: "Ravi Kumar" }, { id: "e2", name: "Priya Nair" }];

    it("offers the assignee above the commercial fields", () => {
        render(
            <JobCardPanel job={job} card={card} rect={RECT} employees={employees} onAssign={vi.fn()} onClose={vi.fn()} onSaved={vi.fn()} />
        );
        const labels = [...document.querySelectorAll("label")].map((l) => l.textContent);
        expect(labels[0]).toMatch(/assigned to/i);
    });

    // It cannot ride along with Save: /jobs/update's normalizeRow carries only the commercial
    // fields, so an assignee in that payload is silently dropped.
    it("assigns immediately rather than on Save", async () => {
        const onAssign = vi.fn();
        render(
            <JobCardPanel job={job} card={card} rect={RECT} employees={employees} onAssign={onAssign} onClose={vi.fn()} onSaved={vi.fn()} />
        );
        fireEvent.click(screen.getByText(/unassigned/i));
        fireEvent.click(await screen.findByText("Ravi Kumar"));
        expect(onAssign).toHaveBeenCalledWith(card, "e1");
        expect(updateJob).not.toHaveBeenCalled();
    });

    it("says why it matters, where someone is already looking", () => {
        render(
            <JobCardPanel job={job} card={card} rect={RECT} employees={employees} onAssign={vi.fn()} onClose={vi.fn()} onSaved={vi.fn()} />
        );
        expect(screen.getByText(/before it can move into a working stage/i)).toBeTruthy();
    });

    // The stage stays with dragging - a select here would be a second way to do the thing the
    // board exists to do.
    it("still does not offer the stage", () => {
        render(
            <JobCardPanel job={job} card={card} rect={RECT} employees={employees} onAssign={vi.fn()} onClose={vi.fn()} onSaved={vi.fn()} />
        );
        expect(screen.queryByLabelText(/stage/i)).toBeNull();
    });

    // An invoiced job passes no handler, and the section goes with it rather than sitting there
    // inert.
    it("omits the section entirely when assigning is not allowed", () => {
        render(<JobCardPanel job={job} card={card} rect={RECT} onClose={vi.fn()} onSaved={vi.fn()} />);
        expect(screen.queryByText(/assigned to/i)).toBeNull();
    });
});

// --- Measurement and tax --------------------------------------------------------------
// Both were missing from this panel: a card entered the wrong way round could not be corrected
// without going back to the table, and its GST could not be seen here at all.
describe("measurement and tax", () => {
    const sentRow = (id = "r1") => {
        const rows = JSON.parse(updateJob.mock.calls[0][0].get("rows"));
        return rows.find((r) => r._id === id);
    };

    const intrastate = {
        _id: "j1",
        challanNumber: "MG/26-27/1",
        rows: [{ ...card, hasDimensions: true, length: "4", width: "3", cgst: 9, sgst: 9, igst: 0 }, other],
    };

    it("shows size fields for a card measured by size, and hides them when switched", () => {
        render(<JobCardPanel job={intrastate} card={intrastate.rows[0]} rect={RECT} onClose={() => {}} />);
        expect(screen.getByLabelText("Length")).toBeTruthy();

        fireEvent.click(screen.getByText("By quantity"));
        expect(screen.queryByLabelText("Length")).toBeNull();
        expect(screen.queryByLabelText("Width")).toBeNull();
    });

    it("reads the row's existing tax back as one number", () => {
        render(<JobCardPanel job={intrastate} card={intrastate.rows[0]} rect={RECT} onClose={() => {}} />);
        expect(screen.getByLabelText("GST %").value).toBe("18");
    });

    it("labels the field IGST on an interstate job, and never shows both", () => {
        const igstJob = { _id: "j1", rows: [{ ...card, igst: 18, cgst: 0, sgst: 0 }] };
        render(<JobCardPanel job={igstJob} card={igstJob.rows[0]} rect={RECT} onClose={() => {}} />);
        expect(screen.getByLabelText("IGST %").value).toBe("18");
        expect(screen.queryByLabelText("GST %")).toBeNull();
    });

    it("splits an edited rate back across cgst and sgst", async () => {
        render(<JobCardPanel job={intrastate} card={intrastate.rows[0]} rect={RECT} onClose={() => {}} />);
        fireEvent.change(screen.getByLabelText("GST %"), { target: { value: "12" } });
        fireEvent.click(screen.getByText("Save"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const row = sentRow();
        expect(row.cgst).toBe(6);
        expect(row.sgst).toBe(6);
        expect(row.igst).toBe(0);
    });

    it("puts an edited rate wholly on igst for an interstate job", async () => {
        const igstJob = { _id: "j1", rows: [{ ...card, igst: 18, cgst: 0, sgst: 0 }] };
        render(<JobCardPanel job={igstJob} card={igstJob.rows[0]} rect={RECT} onClose={() => {}} />);
        fireEvent.change(screen.getByLabelText("IGST %"), { target: { value: "5" } });
        fireEvent.click(screen.getByText("Save"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const row = sentRow();
        expect(row.igst).toBe(5);
        expect(row.cgst).toBe(0);
        expect(row.sgst).toBe(0);
    });

    // The factors rowAmount multiplies by - 0 would total the line to nothing.
    it("sends 1 for the dimensions when a card is switched to by-quantity", async () => {
        render(<JobCardPanel job={intrastate} card={intrastate.rows[0]} rect={RECT} onClose={() => {}} />);
        fireEvent.click(screen.getByText("By quantity"));
        fireEvent.click(screen.getByText("Save"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const row = sentRow();
        expect(row.hasDimensions).toBe(false);
        expect(row.length).toBe("1");
        expect(row.width).toBe("1");
    });

    // The single entered number is a display convenience; it must not reach the row, where
    // nothing reads it and a stray field would be carried forward for ever.
    it("never writes the entered number itself onto the row", async () => {
        render(<JobCardPanel job={intrastate} card={intrastate.rows[0]} rect={RECT} onClose={() => {}} />);
        fireEvent.click(screen.getByText("Save"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        expect("tax" in sentRow()).toBe(false);
    });

    it("leaves every other row untouched", async () => {
        render(<JobCardPanel job={intrastate} card={intrastate.rows[0]} rect={RECT} onClose={() => {}} />);
        fireEvent.change(screen.getByLabelText("GST %"), { target: { value: "12" } });
        fireEvent.click(screen.getByText("Save"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        expect(sentRow("r2")).toEqual(other);
    });
});

// --- Removing a card ----------------------------------------------------------------------
// canDeleteRow was computed by the API and consumed by nothing: there was no way to take a
// card off a job-id from anywhere in the app.
describe("removing a card", () => {
    const twoRowJob = {
        _id: "j1",
        challanNumber: "MG/26-27/1",
        rows: [card, other],
        lock: { canDeleteRow: true, canEditValues: true },
    };

    it("is offered when the server says the row may go", () => {
        render(<JobCardPanel job={twoRowJob} card={card} rect={RECT} onClose={() => {}} />);
        expect(screen.getByText("Remove")).toBeTruthy();
    });

    it("is not offered on a locked job-id", () => {
        const locked = { ...twoRowJob, lock: { canDeleteRow: false } };
        render(<JobCardPanel job={locked} card={card} rect={RECT} onClose={() => {}} />);
        expect(screen.queryByText("Remove")).toBeNull();
    });

    // One press arms it, a second commits. A native confirm blocks the page and says nothing
    // the label cannot.
    it("asks before it removes", () => {
        render(<JobCardPanel job={twoRowJob} card={card} rect={RECT} onClose={() => {}} />);
        fireEvent.click(screen.getByText("Remove"));
        expect(screen.getByText("Remove for good?")).toBeTruthy();
        expect(updateJob).not.toHaveBeenCalled();
    });

    it("sends every row except the one removed", async () => {
        render(<JobCardPanel job={twoRowJob} card={card} rect={RECT} onClose={() => {}} />);
        fireEvent.click(screen.getByText("Remove"));
        fireEvent.click(screen.getByText("Remove for good?"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const rows = JSON.parse(updateJob.mock.calls[0][0].get("rows"));
        expect(rows).toHaveLength(1);
        expect(rows[0]._id).toBe("r2");
    });

    it("relays the server's refusal rather than closing as though it worked", async () => {
        updateJob.mockRejectedValueOnce({ message: "A job needs at least one row." });
        const onClose = vi.fn();
        render(<JobCardPanel job={twoRowJob} card={card} rect={RECT} onClose={onClose} />);
        fireEvent.click(screen.getByText("Remove"));
        fireEvent.click(screen.getByText("Remove for good?"));

        await screen.findByText("A job needs at least one row.");
        expect(onClose).not.toHaveBeenCalled();
        // and it disarms, so the next press is deliberate too
        expect(screen.getByText("Remove")).toBeTruthy();
    });
});
