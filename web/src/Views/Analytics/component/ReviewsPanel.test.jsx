import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getReviews = vi.fn();
vi.mock("../analytics_backend", () => ({ analyticsBackend: { getReviews: (...a) => getReviews(...a) } }));
vi.mock("recharts", () => ({
    ResponsiveContainer: ({ children }) => <div>{children}</div>,
    BarChart: ({ children }) => <div data-testid="bar">{children}</div>,
    Bar: () => null,
    XAxis: () => null,
    YAxis: () => null,
    CartesianGrid: () => null,
    Tooltip: () => null,
    Legend: () => null,
    Cell: () => null,
}));

import ReviewsPanel from "./ReviewsPanel";

const payload = {
    count: 3,
    averages: { quality: 4.3, speed: 3.7, communication: 4, satisfaction: 4, overall: 4 },
    counts: { quality: 3, speed: 3, communication: 3, satisfaction: 3, overall: 3 },
    distribution: { 1: 0, 2: 0, 3: 1, 4: 1, 5: 1 },
    recent: [{ _id: "r1", job: "NWP/26-27/00001", customer: "Northwind Signage", scores: { overall: 5 }, comment: "Great work" }],
};

beforeEach(() => {
    vi.clearAllMocks();
    getReviews.mockResolvedValue({ code: 200, data: payload });
});

describe("ReviewsPanel", () => {
    it("shows every dimension with its average", async () => {
        render(<ReviewsPanel stoken="s" />);
        await waitFor(() => expect(screen.getByText("4.3")).toBeDefined());
        for (const l of ["Quality", "Speed", "Communication", "Satisfaction", "Overall"]) {
            expect(screen.getByText(l)).toBeDefined();
        }
    });

    // An average without its sample size invites the wrong conclusion - 4.5 from two reviews
    // is not the claim it is from two hundred.
    it("states how many reviews the averages come from", async () => {
        render(<ReviewsPanel stoken="s" />);
        await waitFor(() => expect(screen.getByText("From 3 reviews")).toBeDefined());
    });

    it("does not say 'reviews' for a single one", async () => {
        getReviews.mockResolvedValue({ code: 200, data: { ...payload, count: 1 } });
        render(<ReviewsPanel stoken="s" />);
        await waitFor(() => expect(screen.getByText("From 1 review")).toBeDefined());
    });

    // Who said it - an average of 2.8 on communication is only actionable with a name on it.
    it("names the customer behind a recent review", async () => {
        render(<ReviewsPanel stoken="s" />);
        await waitFor(() => expect(screen.getByText("Northwind Signage")).toBeDefined());
        expect(screen.getByText(/Great work/)).toBeDefined();
    });

    // No reviews is normal for a company that just switched this on, and must read as empty
    // rather than as a wall of zeros implying everyone rated them 0.
    it("reads as empty rather than as zeros when nothing is collected", async () => {
        getReviews.mockResolvedValue({ code: 200, data: { count: 0, averages: {}, distribution: {}, recent: [] } });
        render(<ReviewsPanel stoken="s" />);
        await waitFor(() => expect(getReviews).toHaveBeenCalled());
        expect(screen.queryByText("Northwind Signage")).toBeNull();
    });
});
