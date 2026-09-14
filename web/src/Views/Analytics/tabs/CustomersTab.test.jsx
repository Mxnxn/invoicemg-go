import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getTopSales = vi.fn();
const getTopPaid = vi.fn();
vi.mock("../analytics_backend", () => ({
    analyticsBackend: {
        getTopSales: (...a) => getTopSales(...a),
        getTopPaid: (...a) => getTopPaid(...a),
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

import CustomersTab from "./CustomersTab";

// The real shapes: top-sales carries `amount`, top-paid carries `paid`, and the display
// name is clientFirm falling back to clientName.
const sales = [
    { clientId: "1", clientName: "A Ltd", clientFirm: "Acme", amount: 100 },
    { clientId: "2", clientName: "V Ltd", clientFirm: "Vertex", amount: 60 },
    { clientId: "3", clientName: "N Ltd", clientFirm: "", amount: 40 },
];

beforeEach(() => {
    vi.clearAllMocks();
    getTopSales.mockResolvedValue({ code: 200, data: sales });
    getTopPaid.mockResolvedValue({ code: 200, data: [{ clientId: "1", clientFirm: "Acme", paid: 90 }] });
});

describe("CustomersTab", () => {
    it("counts the customers who bought", async () => {
        render(<CustomersTab stoken="s" />);
        // Wait on the COUNT, not on the label. "Active customers" is static markup that is
        // present on the very first render, so gating on it waits for nothing and the next
        // assertion races the promise that sets the data - which is why this failed
        // intermittently depending on which files ran alongside it.
        await waitFor(() => expect(screen.getByText("3")).toBeDefined());
        expect(screen.getByText("Active customers")).toBeDefined();
    });

    it("reports how concentrated the revenue is", async () => {
        render(<CustomersTab stoken="s" />);
        // Three customers, so the top five are all of it.
        // Same reason: the percentage is derived from the fetched rows, the heading is not.
        await waitFor(() => expect(screen.getByText("100%")).toBeDefined());
        expect(screen.getByText("Top-5 concentration")).toBeDefined();
    });

    it("falls back to the client name when a firm is blank", async () => {
        render(<CustomersTab stoken="s" />);
        await waitFor(() => expect(screen.getAllByRole("button", { name: /Table/i }).length).toBeGreaterThan(0));

        // The names live on the chart's axis, which the recharts mock does not render -
        // the table view is where they are assertable, and is the accessible path anyway.
        fireEvent.click(screen.getAllByRole("button", { name: /Table/i })[0]);

        expect(screen.getByText("Acme")).toBeDefined();
        // Row 3 has no firm, so its name has to stand in rather than render blank.
        expect(screen.getByText("N Ltd")).toBeDefined();
    });

    it("charts the top customers", async () => {
        render(<CustomersTab stoken="s" />);
        await waitFor(() => expect(screen.getAllByTestId("bar").length).toBeGreaterThan(0));
    });

    it("says so when an endpoint fails", async () => {
        getTopSales.mockRejectedValue(new Error("nope"));
        render(<CustomersTab stoken="s" />);
        await waitFor(() => expect(screen.getByText(/Could not load customers/)).toBeDefined());
    });
});
