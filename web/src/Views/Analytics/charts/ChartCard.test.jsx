import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ChartCard from "./ChartCard";

const columns = [
    { key: "label", label: "Month" },
    { key: "billed", label: "Billed" },
];
const rows = [
    { label: "Jan", billed: 100 },
    { label: "Feb", billed: 200 },
];

describe("ChartCard", () => {
    it("names the chart", () => {
        render(
            <ChartCard title="Revenue" columns={columns} rows={rows}>
                <div />
            </ChartCard>
        );
        expect(screen.getByText("Revenue")).toBeDefined();
    });

    it("says it is loading rather than showing an empty frame", () => {
        render(
            <ChartCard title="Revenue" loading columns={columns} rows={rows}>
                <div data-testid="plot" />
            </ChartCard>
        );
        expect(screen.getByText(/Loading/)).toBeDefined();
        expect(screen.queryByTestId("plot")).toBeNull();
    });

    it("distinguishes no data from a failure", () => {
        const { rerender } = render(
            <ChartCard title="R" empty columns={columns} rows={[]}>
                <div data-testid="plot" />
            </ChartCard>
        );
        expect(screen.getByText(/Nothing to show/)).toBeDefined();

        rerender(
            <ChartCard title="R" error="Could not load" columns={columns} rows={[]}>
                <div data-testid="plot" />
            </ChartCard>
        );
        expect(screen.getByText("Could not load")).toBeDefined();
        expect(screen.queryByTestId("plot")).toBeNull();
    });

    it("offers a table view carrying the same numbers as the chart", () => {
        // Mandatory, not optional: three light-mode hues sit under 3:1, so the
        // numbers must be readable without relying on colour.
        render(
            <ChartCard title="Revenue" columns={columns} rows={rows}>
                <div data-testid="plot" />
            </ChartCard>
        );
        fireEvent.click(screen.getByRole("button", { name: /Table/i }));

        expect(screen.getByRole("table")).toBeDefined();
        expect(screen.getByText("Jan")).toBeDefined();
        expect(screen.getByText("200")).toBeDefined();
    });
});
