import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import StageAgingBars from "./StageAgingBars";

const rows = [
    { name: "Printing", total: 31, buckets: { "0-2d": 6, "3-7d": 12, "8-14d": 9, "15+d": 4 } },
    { name: "Created", total: 14, buckets: { "0-2d": 9, "3-7d": 4, "8-14d": 1, "15+d": 0 } },
];

describe("StageAgingBars", () => {
    it("labels every row with its name and total", () => {
        render(<StageAgingBars rows={rows} />);
        expect(screen.getByText("Printing")).toBeTruthy();
        expect(screen.getByText("31")).toBeTruthy();
        expect(screen.getByText("Created")).toBeTruthy();
        expect(screen.getByText("14")).toBeTruthy();
    });

    it("renders a legend, so age is never carried by colour alone", () => {
        render(<StageAgingBars rows={rows} />);
        for (const label of ["0-2d", "3-7d", "8-14d", "15+d"]) {
            expect(screen.getAllByText(label).length).toBeGreaterThan(0);
        }
    });

    it("omits zero-count segments rather than drawing a zero-width sliver", () => {
        const { container } = render(<StageAgingBars rows={[rows[1]]} />);
        expect(container.querySelectorAll(".aging-seg").length).toBe(3);
    });

    it("separates the footer row, because unassigned work is not a person's backlog", () => {
        render(
            <StageAgingBars
                rows={rows}
                footerRow={{ name: "Nobody assigned", total: 25, buckets: { "0-2d": 5, "3-7d": 8, "8-14d": 7, "15+d": 5 } }}
            />
        );
        expect(screen.getByText("Nobody assigned")).toBeTruthy();
        expect(screen.getByTestId("aging-footer")).toBeTruthy();
    });

    it("scales bars against the largest row including the footer, so the footer cannot overflow", () => {
        const { container } = render(
            <StageAgingBars rows={[rows[1]]} footerRow={{ name: "Nobody assigned", total: 28, buckets: { "0-2d": 28 } }} />
        );
        const widths = [...container.querySelectorAll(".aging-seg")].map((el) => parseFloat(el.style.width));
        expect(Math.max(...widths)).toBeLessThanOrEqual(100);
    });
});
