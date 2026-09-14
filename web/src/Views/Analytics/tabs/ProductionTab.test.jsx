import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getProductionWip = vi.fn();
const getProductionThroughput = vi.fn();
vi.mock("../analytics_backend", () => ({
    analyticsBackend: {
        getProductionWip: (...a) => getProductionWip(...a),
        getProductionThroughput: (...a) => getProductionThroughput(...a),
    },
}));
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

import ProductionTab from "./ProductionTab";

beforeEach(() => {
    vi.clearAllMocks();
    getProductionWip.mockResolvedValue({
        code: 200,
        data: {
            stages: [{ name: "Printing", total: 31, buckets: { "0-2d": 6, "3-7d": 12, "8-14d": 9, "15+d": 4 } }],
            holders: [{ name: "Ramesh", id: "p1", total: 18, buckets: { "0-2d": 5, "3-7d": 7, "8-14d": 4, "15+d": 2 } }],
            unassigned: { name: "Nobody assigned", total: 25, buckets: { "0-2d": 5, "3-7d": 8, "8-14d": 7, "15+d": 5 } },
            totalOpen: 72,
            oldestDays: 34.2,
            agesExact: true,
        },
    });
    getProductionThroughput.mockResolvedValue({
        code: 200,
        data: {
            trend: [{ label: "05 Jan", completed: 12 }, { label: "12 Jan", completed: 18 }],
            perPerson: [{ name: "Ramesh", completed: 42 }],
            cycleTimeDays: { median: 6.4, p90: 19.1, count: 212 },
        },
    });
});

describe("ProductionTab", () => {
    it("shows the open-card count and the oldest waiting card", async () => {
        render(<ProductionTab stoken="s" />);
        await waitFor(() => expect(screen.getByText("72")).toBeTruthy());
        expect(screen.getByText("34.2d")).toBeTruthy();
    });

    it("shows both WIP cuts, with unassigned separated out", async () => {
        render(<ProductionTab stoken="s" />);
        await waitFor(() => expect(screen.getByText("Printing")).toBeTruthy());
        expect(screen.getByText("Ramesh")).toBeTruthy();
        expect(screen.getByText("Nobody assigned")).toBeTruthy();
    });

    it("says so when ages are estimated rather than implying precision", async () => {
        getProductionWip.mockResolvedValue({
            code: 200,
            data: {
                stages: [{ name: "Printing", total: 1, buckets: { "0-2d": 1 } }],
                holders: [],
                unassigned: { name: "Nobody assigned", total: 0, buckets: {} },
                totalOpen: 1,
                oldestDays: 2,
                agesExact: false,
            },
        });
        render(<ProductionTab stoken="s" />);
        // getAllByText, not getByText: the warning appears on the stat card AND in the
        // panel subtitle, and getByText throws when a query matches more than once.
        await waitFor(() => expect(screen.getAllByText(/some ages are estimated/i).length).toBeGreaterThan(0));
    });

    it("says the invoiced/all switch does not reach this tab, rather than appearing to respond to it", async () => {
        render(<ProductionTab stoken="s" source="all" />);
        await waitFor(() => expect(screen.getByText(/invoiced\/all/i)).toBeTruthy());
    });

    it("surfaces an error instead of rendering an empty panel as though it were zero work", async () => {
        getProductionWip.mockRejectedValue(new Error("nope"));
        render(<ProductionTab stoken="s" />);
        // Both WIP panels are fed by this one call, so both must report the failure - a
        // panel that silently rendered empty would read as "no work in progress", which is
        // the opposite of the truth.
        await waitFor(() => expect(screen.getAllByText(/could not load/i)).toHaveLength(2));
    });
});
