import { fireEvent, render as rtlRender, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

const list = vi.fn();
vi.mock("../purchaseOrder_backend", () => ({ purchaseOrderBackend: { list: (...a) => list(...a) } }));
// LiteHeader is a .js file containing JSX, which vitest will not transform. Mocked the same
// way AnalyticsIndex.test.jsx does it - this suite is about the list, not the page chrome.
vi.mock("../../../Common/Header/LiteHeader", () => ({ default: () => null }));
// The panel has its own suite; this one is about the list.
vi.mock("./PoDetailSidebar", () => ({ default: () => <div data-testid="detail" /> }));
// The form has its own suite; here it only needs to signal that it opened.
vi.mock("./PurchaseOrderModal", () => ({ default: ({ isOpen }) => (isOpen ? <div data-testid="modal" /> : null) }));

import PurchaseOrderIndex from "./PurchaseOrderIndex";

// The list reads ?open=<id> so the navbar bell can deep-link to one order, which means it
// needs a router. MemoryRouter is how the other suites here supply one.
const render = (ui) => rtlRender(<MemoryRouter>{ui}</MemoryRouter>);

beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue({
        code: 200,
        data: [
            {
                _id: "po1",
                poNumber: "PO/26-27/000001",
                date: "2026-09-01",
                total: 5000,
                supplier_id: { name: "Ravi", firm: "Ravi Papers" },
                approval: { state: "approved" },
                purchaseInvoice_id: null,
            },
            {
                _id: "po2",
                poNumber: "PO/26-27/000002",
                date: "2026-09-02",
                total: 1200,
                supplier_id: { name: "Sunil", firm: "Sunil Inks" },
                approval: { state: "draft" },
                purchaseInvoice_id: null,
            },
            {
                // Approved AND converted: the row that proves converted outranks approved.
                _id: "po3",
                poNumber: "PO/26-27/000003",
                date: "2026-09-03",
                total: 800,
                supplier_id: { name: "Asha", firm: "Asha Board" },
                approval: { state: "approved" },
                purchaseInvoice_id: "pi1",
            },
        ],
    });
});

describe("PurchaseOrderIndex", () => {
    it("lists purchase orders with supplier and number", async () => {
        render(<PurchaseOrderIndex uid="u1" />);
        await waitFor(() => expect(screen.getByText("PO/26-27/000001")).toBeTruthy());
        expect(screen.getByText("Ravi Papers")).toBeTruthy();
        expect(screen.getByText("PO/26-27/000002")).toBeTruthy();
    });

    it("shows approval state per row", async () => {
        render(<PurchaseOrderIndex uid="u1" />);
        await waitFor(() => expect(screen.getByText("Approved")).toBeTruthy());
        expect(screen.getByText("Draft")).toBeTruthy();
    });

    it("shows converted rather than approved once billed, so nobody acts on it again", async () => {
        render(<PurchaseOrderIndex uid="u1" />);
        // po3 is approved AND converted. Converted has to win: showing "Approved" on an
        // order already billed invites someone to send or re-order against it.
        await waitFor(() => expect(screen.getByText("Invoiced")).toBeTruthy());
        expect(screen.getAllByText("Approved")).toHaveLength(1);
    });

    it("narrows the list as you search, by number or supplier", async () => {
        render(<PurchaseOrderIndex uid="u1" />);
        await waitFor(() => expect(screen.getByText("PO/26-27/000001")).toBeTruthy());

        const box = screen.getByPlaceholderText(/search order number/i);
        fireEvent.change(box, { target: { value: "Sunil" } });
        expect(screen.getByText("PO/26-27/000002")).toBeTruthy();
        expect(screen.queryByText("PO/26-27/000001")).toBeNull();

        fireEvent.change(box, { target: { value: "000003" } });
        expect(screen.getByText("PO/26-27/000003")).toBeTruthy();
        expect(screen.queryByText("PO/26-27/000002")).toBeNull();
    });

    it("says nothing matched rather than looking like there are no orders at all", async () => {
        render(<PurchaseOrderIndex uid="u1" />);
        await waitFor(() => expect(screen.getByText("PO/26-27/000001")).toBeTruthy());
        fireEvent.change(screen.getByPlaceholderText(/search order number/i), { target: { value: "zzzz" } });
        expect(screen.getByText(/no purchase orders match/i)).toBeTruthy();
    });

    it("opens the form from the New button", async () => {
        render(<PurchaseOrderIndex uid="u1" />);
        await waitFor(() => expect(screen.getByText("PO/26-27/000001")).toBeTruthy());
        expect(screen.queryByTestId("modal")).toBeNull();
        fireEvent.click(screen.getByRole("button", { name: /new purchase order/i }));
        expect(screen.getByTestId("modal")).toBeTruthy();
    });

    it("surfaces an error instead of rendering an empty list as though there were no orders", async () => {
        list.mockRejectedValue(new Error("nope"));
        render(<PurchaseOrderIndex uid="u1" />);
        await waitFor(() => expect(screen.getByText(/could not load/i)).toBeTruthy());
    });
});
