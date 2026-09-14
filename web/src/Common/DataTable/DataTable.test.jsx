import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import DataTable from "./DataTable";

const HEAD = (
    <thead>
        <tr>
            <th scope="col">Number</th>
            <th scope="col">Supplier</th>
            <th scope="col">Total</th>
        </tr>
    </thead>
);

describe("DataTable", () => {
    it("renders its children when not loading", () => {
        render(
            <DataTable>
                {HEAD}
                <tbody>
                    <tr>
                        <td>PO/1</td>
                        <td>Ravi</td>
                        <td>₹100</td>
                    </tr>
                </tbody>
            </DataTable>
        );
        expect(screen.getByText("PO/1")).toBeTruthy();
        expect(screen.queryByTestId("table-shimmer")).toBeNull();
    });

    it("shows shimmer rows while loading, and keeps the header", () => {
        const { container } = render(
            <DataTable loading columns={3}>
                {HEAD}
                <tbody>
                    <tr>
                        <td>PO/1</td>
                    </tr>
                </tbody>
            </DataTable>
        );
        // The header stays: the columns are known before the data is, and a table that
        // reflows once the rows land reads as a different table.
        expect(screen.getByText("Supplier")).toBeTruthy();
        // Real rows are not rendered alongside the placeholder.
        expect(screen.queryByText("PO/1")).toBeNull();
        expect(screen.getByTestId("table-shimmer")).toBeTruthy();
        expect(container.querySelectorAll(".xan-shimmer-cell").length).toBeGreaterThan(0);
    });

    it("lays out the placeholder to the column count it is given", () => {
        const { container } = render(
            <DataTable loading columns={3} rows={4}>
                {HEAD}
            </DataTable>
        );
        // 4 rows x 3 columns - matching the real shape stops the swap from jumping.
        expect(container.querySelectorAll(".xan-shimmer-cell").length).toBe(12);
    });

    it("counts the columns off an inline header when none is given", () => {
        const { container } = render(
            <DataTable loading rows={2}>
                {HEAD}
            </DataTable>
        );
        expect(container.querySelectorAll(".xan-shimmer-cell").length).toBe(6);
    });

    it("still renders a header it cannot count, using the given column count", () => {
        // A header wrapped in a component cannot be counted by element type - `columns` is
        // the escape hatch, and the header must still appear.
        const Head = () => HEAD;
        const { container } = render(
            <DataTable loading columns={3} rows={2}>
                <Head />
            </DataTable>
        );
        expect(screen.getByText("Supplier")).toBeTruthy();
        expect(container.querySelectorAll(".xan-shimmer-cell").length).toBe(6);
    });

    it("is announced as busy rather than as an empty table", () => {
        render(
            <DataTable loading columns={2}>
                {HEAD}
            </DataTable>
        );
        expect(screen.getByTestId("table-shimmer").closest("[aria-busy='true']")).toBeTruthy();
    });
});
