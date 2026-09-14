import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

const get = vi.fn();
vi.mock("axios", () => ({ default: { get: (...a) => get(...a) } }));

import PrePurchaseOrder from "./PrePurchaseOrder";

const live = {
    closed: false,
    order: {
        poNumber: "PO/26-27/000001",
        date: "2026-09-12",
        total: 236,
        supplier: { name: "Ravi", firm: "Ravi Papers" },
        rows: [
            { description: "front panel", material: "Vinyl", hsn: "3919", gst: 18, hasDimensions: false, length: "1", width: "1", qty: 2, rate: 100, unit: "sheet", discount: 0, charges: 0 },
        ],
    },
    company: { firm: "Mahalaxmi Graphics", address: "5 Ring Rd", phone: "888", gst: "27BBB", url: "", documentFont: "lato", exportTemplate: {} },
};

const renderPage = () =>
    render(
        <MemoryRouter initialEntries={["/PO/s1/po1"]}>
            <Routes>
                <Route path="/PO/:supplier_id/:po_id" element={<PrePurchaseOrder />} />
            </Routes>
        </MemoryRouter>
    );

beforeEach(() => {
    vi.clearAllMocks();
    get.mockResolvedValue({ data: { code: 200, status: true, data: live } });
});

describe("PrePurchaseOrder", () => {
    it("shows the order, the supplier and the rows", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("PO/26-27/000001")).toBeTruthy());
        expect(screen.getByText(/Ravi Papers/)).toBeTruthy();
        expect(screen.getByText(/Vinyl/)).toBeTruthy();
        expect(screen.getByText(/Mahalaxmi Graphics/)).toBeTruthy();
    });

    it("offers both downloads on a live order", async () => {
        renderPage();
        await waitFor(() => expect(screen.getByText("PO/26-27/000001")).toBeTruthy());
        expect(screen.getByRole("button", { name: /pdf/i })).toBeTruthy();
        expect(screen.getByRole("button", { name: /excel/i })).toBeTruthy();
    });

    it("shows the closed message and serves NO commercial detail once converted", async () => {
        get.mockResolvedValue({ data: { code: 200, status: true, data: { closed: true } } });
        renderPage();
        await waitFor(() => expect(screen.getByText(/no longer current/i)).toBeTruthy());
        // The whole point of expiry: nothing about the order stays reachable.
        expect(screen.queryByText("PO/26-27/000001")).toBeNull();
        expect(screen.queryByText(/Vinyl/)).toBeNull();
        expect(screen.queryByRole("button", { name: /pdf/i })).toBeNull();
    });

    it("shows a not-found state when the id pair does not match", async () => {
        get.mockResolvedValue({ data: { code: 404, status: false, message: "Purchase order not found." } });
        renderPage();
        await waitFor(() => expect(screen.getByText(/not found/i)).toBeTruthy());
    });

    it("shows a readable message when the request fails outright", async () => {
        get.mockRejectedValue(new Error("offline"));
        renderPage();
        await waitFor(() => expect(screen.getByText(/could not load/i)).toBeTruthy());
    });
});
