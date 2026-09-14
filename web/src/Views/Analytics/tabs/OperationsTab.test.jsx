import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getUnbilled = vi.fn();
const getAvgPendingTime = vi.fn();
vi.mock("../analytics_backend", () => ({
    analyticsBackend: {
        getUnbilled: (...a) => getUnbilled(...a),
        getAvgPendingTime: (...a) => getAvgPendingTime(...a),
    },
}));
vi.mock("../component/BottleneckMetrics", () => ({ default: () => <div data-testid="bottleneck" /> }));
// Stubbed like BottleneckMetrics beside it: this file is about unbilled work, and a panel
// that fetches its own reviews would otherwise need its endpoint mocked here too - coupling
// these tests to a feature they do not describe. ReviewsPanel has its own suite.
vi.mock("../component/ReviewsPanel", () => ({ default: () => <div data-testid="reviews" /> }));
vi.mock("recharts", () => ({
    ResponsiveContainer: ({ children }) => <div>{children}</div>,
    BarChart: ({ children }) => <div data-testid="bar">{children}</div>,
    Bar: () => null,
    XAxis: () => null,
    YAxis: () => null,
    CartesianGrid: () => null,
    Tooltip: () => null,
    Legend: () => null,
}));

import OperationsTab from "./OperationsTab";

beforeEach(() => {
    vi.clearAllMocks();
    getUnbilled.mockResolvedValue({
        code: 200,
        count: 4,
        totalValue: 12000,
        avgAgeDays: 9.5,
        topClients: [{ clientFirm: "Acme", clientName: "A Ltd", value: 8000 }],
    });
    // The route answers { data: { avgDays, count } }, not a bare `days`.
    getAvgPendingTime.mockResolvedValue({ code: 200, data: { avgDays: 3, count: 7 } });
});

describe("OperationsTab", () => {
    it("surfaces unbilled work, which nothing showed before", async () => {
        render(<OperationsTab stoken="s" />);
        // Wait on the FIGURE, not the label - the label renders before the fetch resolves,
        // so waiting on it leaves the money assertion racing the promise.
        await waitFor(() => expect(screen.getByText("₹12,000")).toBeDefined());
        expect(screen.getByText("Unbilled value")).toBeDefined();
    });

    it("shows how old the unbilled work is", async () => {
        render(<OperationsTab stoken="s" />);
        await waitFor(() => expect(screen.getByText("9.5")).toBeDefined());
    });

    it("reads pending time from the shape the route actually returns", async () => {
        render(<OperationsTab stoken="s" />);
        await waitFor(() => expect(screen.getByText("3")).toBeDefined());
    });

    it("names the customers holding unbilled work", async () => {
        render(<OperationsTab stoken="s" />);
        await waitFor(() => expect(screen.getAllByRole("button", { name: /Table/i }).length).toBeGreaterThan(0));
        fireEvent.click(screen.getAllByRole("button", { name: /Table/i })[0]);
        expect(screen.getByText("Acme")).toBeDefined();
    });

    it("keeps the existing bottleneck panel", async () => {
        render(<OperationsTab stoken="s" />);
        await waitFor(() => expect(screen.getByTestId("bottleneck")).toBeDefined());
    });
});
