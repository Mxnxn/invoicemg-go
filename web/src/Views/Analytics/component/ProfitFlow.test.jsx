import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const cashflow = vi.fn();
vi.mock("../analytics_backend", () => ({ analyticsBackend: { cashflow: (...a) => cashflow(...a) } }));

import ProfitFlow from "./ProfitFlow";

const totals = {
    revenue: 1180,
    cogs: 200,
    wastage: 50,
    tax: 180,
    netProfit: 750,
    netMargin: 63.6,
    operatingMargin: 78.8,
    collectionRate: 100,
};

beforeEach(() => {
    vi.clearAllMocks();
    cashflow.mockResolvedValue({ code: 200, data: { totals } });
});

describe("ProfitFlow", () => {
    // Every step of the flow has to be on screen - a strip that shows revenue and profit but
    // hides what came out between them is the thing this replaced.
    it("shows all five steps of the flow", async () => {
        render(<ProfitFlow stoken="s" />);
        await waitFor(() => expect(screen.getByText("Revenue")).toBeDefined());
        expect(screen.getByText("Operations")).toBeDefined();
        expect(screen.getByText("Waste & Material")).toBeDefined();
        expect(screen.getByText("Tax")).toBeDefined();
        expect(screen.getByText("Net Profit")).toBeDefined();
    });

    // Tax is subtracted because revenue is tax-inclusive. If this figure ever renders as
    // anything but the tax actually collected, net profit is overstated by the whole tax bill.
    it("renders the tax that was deducted", async () => {
        render(<ProfitFlow stoken="s" />);
        await waitFor(() => expect(screen.getByText("₹180.00")).toBeDefined());
        expect(screen.getByText("₹750.00")).toBeDefined();
    });

    // A failed load must NOT render a strip of zeros - "you earned nothing" and "we could not
    // find out" are different statements and only one of them is true.
    it("says it failed rather than showing zeros", async () => {
        cashflow.mockRejectedValue(new Error("down"));
        render(<ProfitFlow stoken="s" />);
        await waitFor(() => expect(screen.getByText(/Couldn’t load the figures/)).toBeDefined());
        expect(screen.queryByText("Net Profit")).toBeNull();
    });

    // A loss is the figure most worth seeing, so it renders as a loss.
    it("shows a negative net profit as a loss", async () => {
        cashflow.mockResolvedValue({ code: 200, data: { totals: { ...totals, netProfit: -418, netMargin: -35 } } });
        render(<ProfitFlow stoken="s" />);
        await waitFor(() => expect(screen.getByText("₹-418.00")).toBeDefined());
    });
});
