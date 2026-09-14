import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getRevenue = vi.fn();
const cashflow = vi.fn();
vi.mock("../analytics_backend", () => ({
    analyticsBackend: {
        getRevenue: (...a) => getRevenue(...a),
        cashflow: (...a) => cashflow(...a),
    },
}));
vi.mock("recharts", () => ({
    ResponsiveContainer: ({ children }) => <div>{children}</div>,
    LineChart: ({ children }) => <div data-testid="line">{children}</div>,
    BarChart: ({ children }) => <div data-testid="bar">{children}</div>,
    Line: () => null,
    Bar: () => null,
    XAxis: () => null,
    YAxis: () => null,
    CartesianGrid: () => null,
    Tooltip: () => null,
    Legend: () => null,
}));

import RevenueTab from "./RevenueTab";

beforeEach(() => {
    vi.clearAllMocks();
    getRevenue.mockResolvedValue({
        code: 200,
        data: [
            { label: "Jan", billed: 100, collected: 60 },
            { label: "Feb", billed: 100, collected: 80 },
        ],
    });
    cashflow.mockResolvedValue({
        code: 200,
        data: { series: [], bestMargins: [{ name: "Art card", margin: 30 }], worstMargins: [] },
    });
});

describe("RevenueTab", () => {
    it("shows the revenue KPIs", async () => {
        render(<RevenueTab stoken="s" />);
        await waitFor(() => expect(screen.getByText("Billed")).toBeDefined());
        expect(screen.getByText("Collected")).toBeDefined();
        // 140 collected of 200 billed
        await waitFor(() => expect(screen.getByText("70%")).toBeDefined());
    });

    it("plots revenue over time", async () => {
        render(<RevenueTab stoken="s" />);
        await waitFor(() => expect(screen.getByTestId("line")).toBeDefined());
    });

    it("sends the invoiced/all source to the endpoint that honours it", async () => {
        render(<RevenueTab stoken="s" source="all" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        expect(getRevenue.mock.calls[0][0].get("source")).toBe("all");
    });

    it("relabels under 'all', because production value is not billed revenue", async () => {
        render(<RevenueTab stoken="s" source="all" />);
        await waitFor(() => expect(screen.getByText("Production value")).toBeDefined());
        expect(screen.queryByText("Billed")).toBeNull();
    });

    it("survives an endpoint that fails, saying so rather than showing an empty chart", async () => {
        getRevenue.mockRejectedValue(new Error("nope"));
        render(<RevenueTab stoken="s" />);
        await waitFor(() => expect(screen.getByText(/Could not load revenue/)).toBeDefined());
    });
});

describe("RevenueTab request contract", () => {
    // The route REQUIRES `period` - routes/Analytics.js returns
    // { code: 422, message: "Invalid request." } without it, and has since the tab was built.
    // Every mock in this file resolves happily, which is exactly why nothing caught it: the
    // test asserted against a stub that did not share the real endpoint's contract.
    it("sends the period the endpoint requires", async () => {
        render(<RevenueTab stoken="s" source="invoiced" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        const form = getRevenue.mock.calls[0][0];
        expect(form.get("period")).toBeTruthy();
        expect(["weekly", "monthly", "yearly"]).toContain(form.get("period"));
    });

    // The Invoiced/All switch still has to reach it - this is the one endpoint that honours it.
    it("still sends the source", async () => {
        render(<RevenueTab stoken="s" source="all" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        expect(getRevenue.mock.calls[0][0].get("source")).toBe("all");
    });
});

describe("RevenueTab period selector", () => {
    // The old RevenueCard had weekly/monthly/yearly and the tab rewrite dropped it, leaving
    // Revenue pinned to 36 trailing months with no way to ask a different question.
    it("offers all three periods", async () => {
        render(<RevenueTab stoken="s" source="invoiced" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        for (const label of ["Weekly", "Monthly", "Yearly"]) {
            expect(screen.getByRole("tab", { name: label })).toBeDefined();
        }
    });

    it("starts on monthly", async () => {
        render(<RevenueTab stoken="s" source="invoiced" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        expect(getRevenue.mock.calls[0][0].get("period")).toBe("monthly");
        expect(screen.getByRole("tab", { name: "Monthly" }).getAttribute("aria-selected")).toBe("true");
    });

    it("re-fetches with the chosen period", async () => {
        render(<RevenueTab stoken="s" source="invoiced" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        getRevenue.mockClear();
        fireEvent.click(screen.getByRole("tab", { name: "Yearly" }));
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        expect(getRevenue.mock.calls[0][0].get("period")).toBe("yearly");
    });

    // Switching period must clear any drill-down. Carrying focusYear from a monthly drill
    // into a yearly request asks the route for "the months of 2025" while the tab claims to
    // be showing years.
    it("clears the drill-down when the period changes", async () => {
        render(<RevenueTab stoken="s" source="invoiced" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        getRevenue.mockClear();
        fireEvent.click(screen.getByRole("tab", { name: "Weekly" }));
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        const form = getRevenue.mock.calls[0][0];
        expect(form.get("focusYear")).toBeNull();
        expect(form.get("focusMonth")).toBeNull();
    });

    // The source switch has to survive a period change - they are independent questions.
    it("keeps the invoiced/all source when the period changes", async () => {
        render(<RevenueTab stoken="s" source="all" />);
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        getRevenue.mockClear();
        fireEvent.click(screen.getByRole("tab", { name: "Yearly" }));
        await waitFor(() => expect(getRevenue).toHaveBeenCalled());
        expect(getRevenue.mock.calls[0][0].get("source")).toBe("all");
    });
});
