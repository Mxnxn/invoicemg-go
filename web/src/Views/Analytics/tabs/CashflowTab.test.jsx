import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const cashflow = vi.fn();
const getAging = vi.fn();
const getPayables = vi.fn();
const getAvgPaymentTime = vi.fn();
const getPayoutWeekday = vi.fn();
vi.mock("../analytics_backend", () => ({
    analyticsBackend: {
        cashflow: (...a) => cashflow(...a),
        getAging: (...a) => getAging(...a),
        getPayables: (...a) => getPayables(...a),
        getAvgPaymentTime: (...a) => getAvgPaymentTime(...a),
        getPayoutWeekday: (...a) => getPayoutWeekday(...a),
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

import CashflowTab from "./CashflowTab";

beforeEach(() => {
    vi.clearAllMocks();
    cashflow.mockResolvedValue({ code: 200, data: { series: [{ month: "2026-01", cashIn: 500, cashOut: 200 }] } });
    getAging.mockResolvedValue({
        code: 200,
        data: [
            { label: "0-30 days", amount: 500, count: 2 },
            { label: "31-60 days", amount: 400, count: 1 },
        ],
        totalOutstanding: 900,
    });
    getPayables.mockResolvedValue({ code: 200, totalPayable: 250, bySupplier: [] });
    getAvgPaymentTime.mockResolvedValue({ code: 200, data: { avgDays: 12, count: 4 } });
    getPayoutWeekday.mockResolvedValue({
        code: 200,
        data: [{ label: "Mon", amount: 300 }, { label: "Tue", amount: 0 }],
        periodLabel: "January 2026",
    });
});

describe("CashflowTab", () => {
    it("shows both sides of the money, not just receivables", async () => {
        render(<CashflowTab stoken="s" />);
        await waitFor(() => expect(screen.getByText("Outstanding")).toBeDefined());
        expect(screen.getByText("Payables")).toBeDefined();
        expect(screen.getByText("Net position")).toBeDefined();
    });

    it("computes net position from receivables minus payables", async () => {
        render(<CashflowTab stoken="s" />);
        // 900 outstanding - 250 payable
        await waitFor(() => expect(screen.getByText("₹650")).toBeDefined());
    });

    it("counts everything past 30 days as overdue", async () => {
        render(<CashflowTab stoken="s" />);
        // Wait on the amount, which needs the fetch; the bucket label may not.
        await waitFor(() => expect(screen.getByText("₹400")).toBeDefined());
        expect(screen.getByText("Overdue")).toBeDefined();
    });

    it("renders the aging buckets", async () => {
        render(<CashflowTab stoken="s" />);
        await waitFor(() => expect(screen.getAllByTestId("bar").length).toBeGreaterThan(0));
    });

    it("reads the average payment time from the shape the route actually returns", async () => {
        // The route answers { data: { avgDays } }. Mocking a bare `days` here once made
        // this pass while the real KPI rendered a dash.
        render(<CashflowTab stoken="s" />);
        await waitFor(() => expect(screen.getByText("12")).toBeDefined());
    });

    it("asks payout-weekday for a period, which it refuses to answer without", async () => {
        render(<CashflowTab stoken="s" />);
        await waitFor(() => expect(getPayoutWeekday).toHaveBeenCalled());
        const sent = getPayoutWeekday.mock.calls[0][0];
        expect(sent.get("year")).toBeTruthy();
        expect(sent.get("month")).toBeTruthy();
    });

    it("says the cash figures ignore the invoiced/all switch", async () => {
        // Uninvoiced work has not been billed, let alone paid, so it must never move a
        // cash line. Saying so beats appearing to respond to the toggle.
        render(<CashflowTab stoken="s" source="all" />);
        await waitFor(() => expect(screen.getByText(/always invoiced/i)).toBeDefined());
    });
});
