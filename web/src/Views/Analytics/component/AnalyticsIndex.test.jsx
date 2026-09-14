import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../tabs/RevenueTab", () => ({ default: ({ source }) => <div data-testid="revenue" data-source={source} /> }));
vi.mock("../tabs/CustomersTab", () => ({ default: () => <div data-testid="customers" /> }));
vi.mock("../tabs/CashflowTab", () => ({ default: () => <div data-testid="cashflow" /> }));
vi.mock("../tabs/OperationsTab", () => ({ default: () => <div data-testid="operations" /> }));
vi.mock("../tabs/ProductionTab", () => ({ default: () => <div data-testid="production" /> }));
vi.mock("../../../Common/Header/LiteHeader", () => ({ default: () => null }));

import AnalyticsIndex from "./AnalyticsIndex";

describe("AnalyticsIndex", () => {
    it("offers one tab per category", () => {
        const { container } = render(<AnalyticsIndex uid="u" />);
        const labels = [...container.querySelectorAll('[role="tab"]')].map((t) => t.textContent.trim());
        expect(labels).toEqual(["Revenue", "Customers", "Cashflow", "Operations", "Production"]);
    });

    it("opens on Revenue", () => {
        render(<AnalyticsIndex uid="u" />);
        expect(screen.getByTestId("revenue")).toBeDefined();
        expect(screen.queryByTestId("cashflow")).toBeNull();
    });

    it("shows only the chosen category, so nothing borrows another's charts", () => {
        render(<AnalyticsIndex uid="u" />);
        fireEvent.click(screen.getByRole("tab", { name: "Cashflow" }));
        expect(screen.getByTestId("cashflow")).toBeDefined();
        expect(screen.queryByTestId("revenue")).toBeNull();
    });

    it("passes the navbar's invoiced/all source down to the tab", () => {
        // The control already exists in AppNavbar; the bug it had was reaching
        // only one card. The shell is what makes it reach a whole tab.
        render(<AnalyticsIndex uid="u" source="all" />);
        expect(screen.getByTestId("revenue").getAttribute("data-source")).toBe("all");
    });

    it("defaults to invoiced, the figures that reconcile with the books", () => {
        render(<AnalyticsIndex uid="u" />);
        expect(screen.getByTestId("revenue").getAttribute("data-source")).toBe("invoiced");
    });
});
