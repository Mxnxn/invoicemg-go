import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { DashboardMockup } from "./DashboardMockup";
import { ActivityFeed } from "./ActivityFeed";
import { IntegrationsBeam } from "./IntegrationsBeam";

afterEach(() => {
    vi.useRealTimers();
});

describe("landing demo widgets", () => {
    it("DashboardMockup shows invoice-shaped content, not lorem ipsum", () => {
        render(<DashboardMockup />);
        // Exact, not /invoices/i - the mocked address bar also says "invoices".
        expect(screen.getByText("Invoices")).toBeDefined();
        expect(screen.getByText("Sharma Packaging")).toBeDefined();
    });

    it("ActivityFeed shows job lifecycle events", () => {
        const { container } = render(<ActivityFeed />);
        expect(container.textContent).toMatch(/Challan|Invoice|Printing/);
    });

    it("IntegrationsBeam labels both the inputs and the outputs", () => {
        const { container } = render(<IntegrationsBeam />);
        expect(container.querySelectorAll("svg").length).toBeGreaterThan(0);
        expect(container.textContent).toContain("InvoiceMG");
    });
});

describe("DashboardMockup job preview", () => {
    it("exposes every invoice row as a button so it is keyboard reachable", () => {
        render(<DashboardMockup />);
        const rows = screen.getAllByRole("button", { name: /^Open job / });
        expect(rows).toHaveLength(4);
    });

    it("opens the job panel for the row that was clicked", () => {
        render(<DashboardMockup />);
        fireEvent.click(screen.getByRole("button", { name: /Open job .* Vertex Labels/ }));

        const panel = screen.getByRole("dialog");
        expect(panel.textContent).toContain("Vertex Labels");
        expect(panel.textContent).toContain("₹1,12,000");
    });

    it("shows the job's lifecycle stages with the current one marked", () => {
        render(<DashboardMockup />);
        fireEvent.click(screen.getAllByRole("button", { name: /^Open job / })[0]);

        const panel = screen.getByRole("dialog");
        expect(panel.textContent).toContain("Received");
        expect(panel.textContent).toContain("Dispatched");
        expect(panel.querySelector('[data-stage-state="current"]')).not.toBeNull();
        expect(panel.querySelectorAll('[data-stage-state="done"]').length).toBeGreaterThan(0);
    });

    it("shows two job rows at different stages, each with an assignee", () => {
        render(<DashboardMockup />);
        fireEvent.click(screen.getAllByRole("button", { name: /^Open job / })[0]);

        const jobRows = screen.getByRole("dialog").querySelectorAll("[data-job-row]");
        expect(jobRows).toHaveLength(2);

        // Different stages, not the same badge twice.
        const stages = [...jobRows].map((r) => r.textContent.match(/Printing|Plate|Design|Cutting|Packing|Lamination|Received|Dispatched/)[0]);
        expect(new Set(stages).size).toBe(2);

        // Every row names the person who has it.
        [...jobRows].forEach((r) => expect(r.textContent).toMatch(/[A-Z][a-z]+ [A-Z]\./));
    });

    it("outlines the row before its panel opens", async () => {
        render(<DashboardMockup />);

        // The first beat is the lead-in: armed, but nothing open yet.
        const armed = document.querySelector('[data-row-state="armed"]');
        expect(armed).not.toBeNull();
        expect(screen.queryByRole("dialog")).toBeNull();

        // ...and that same row is the one that then opens.
        const armedJob = armed.getAttribute("aria-label").replace(/^Open job /, "");
        const panel = await screen.findByRole("dialog", {}, { timeout: 2000 });
        expect(armedJob).toContain(panel.getAttribute("aria-label").replace(/^Job /, ""));
    });

    // The panel leaves through an exit animation, so it stays mounted for a beat
    // after the close - these wait for removal rather than asserting synchronously.
    it("closes on Escape", async () => {
        render(<DashboardMockup />);
        fireEvent.click(screen.getAllByRole("button", { name: /^Open job / })[0]);
        expect(screen.getByRole("dialog")).toBeDefined();

        fireEvent.keyDown(window, { key: "Escape" });
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    });

    it("closes when the scrim behind the panel is clicked", async () => {
        render(<DashboardMockup />);
        fireEvent.click(screen.getAllByRole("button", { name: /^Open job / })[0]);

        fireEvent.click(screen.getByTestId("job-preview-scrim"));
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    });

    it("cycles through the rows on its own, with no interaction at all", async () => {
        render(<DashboardMockup />);

        // Starts closed so the table reads first.
        expect(screen.queryByRole("dialog")).toBeNull();

        // Real timers, because motion drives its exit animations off rAF and a
        // fake clock leaves a closing panel mounted forever.
        const first = await screen.findByRole("dialog", {}, { timeout: 2000 });
        const firstJob = first.getAttribute("aria-label");

        await waitFor(
            () => {
                const panel = screen.queryByRole("dialog");
                expect(panel).not.toBeNull();
                expect(panel.getAttribute("aria-label")).not.toBe(firstJob);
            },
            { timeout: 6000 }
        );
    });

    it("stops cycling once a row has been clicked", () => {
        vi.useFakeTimers();
        render(<DashboardMockup />);

        fireEvent.click(screen.getAllByRole("button", { name: /^Open job / })[2]);
        const chosen = screen.getByRole("dialog").textContent;

        act(() => vi.advanceTimersByTime(12000));
        expect(screen.getByRole("dialog").textContent).toBe(chosen);
    });
});
