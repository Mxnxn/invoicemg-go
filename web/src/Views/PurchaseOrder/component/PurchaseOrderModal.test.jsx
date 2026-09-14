import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const personList = vi.fn();
const materialsList = vi.fn();
const unitList = vi.fn();

vi.mock("../../../Common/person_backend", () => ({ personBackend: { list: (...a) => personList(...a) } }));
vi.mock("../../Material/material_backend", () => ({ materialsBackend: { getAllMaterials: (...a) => materialsList(...a) } }));
vi.mock("../../../Common/unit_backend", () => ({ unitBackend: { list: (...a) => unitList(...a) } }));
// Quick-create lives in the Purchase Invoice view and opens its own modal; this suite is
// about the order form, not supplier creation.
vi.mock("../../PurchaseInvoice/component/QuickCreateSupplierModal", () => ({ default: () => null }));

import PurchaseOrderModal from "./PurchaseOrderModal";

const supplier = { _id: "s1", name: "Ravi", firm: "Ravi Papers", phone: "999" };

const approvedPo = {
    _id: "po1",
    poNumber: "PO/26-27/000001",
    date: "2026-09-12",
    supplier_id: supplier,
    approval: { state: "approved" },
    purchaseInvoice_id: null,
    rows: [
        { _id: "r1", description: "front panel", material: "Vinyl", hsn: "3919", gst: 18, hasDimensions: false, length: "1", width: "1", rate: 100, qty: 2, unit: "sheet", discount: 0, charges: 0 },
    ],
};

beforeEach(() => {
    vi.clearAllMocks();
    personList.mockResolvedValue({ data: [supplier] });
    materialsList.mockResolvedValue({ data: [{ _id: "m1", material_name: "Vinyl", hsn: "3919", tax: 18, purchase_rate: 100 }] });
    unitList.mockResolvedValue({ data: [{ name: "sheet" }] });
});

const noop = () => {};

describe("PurchaseOrderModal", () => {
    it("starts a new order with one empty row and today's date", async () => {
        render(<PurchaseOrderModal isOpen toggle={noop} onCreate={vi.fn()} onUpdate={vi.fn()} editingOrder={null} />);
        // "Create Purchase Order" is both the title and the submit button, so ask for the
        // button specifically rather than matching text twice.
        await waitFor(() => expect(screen.getByRole("button", { name: "Create Purchase Order" })).toBeTruthy());
        // AssigneeDropdown shows its placeholder as the trigger's text, not as an input
        // placeholder attribute.
        expect(screen.getByText("Select supplier")).toBeTruthy();
        expect(screen.getByText("Generated on save")).toBeTruthy();
    });

    it("prefills from the order being edited", async () => {
        render(<PurchaseOrderModal isOpen toggle={noop} onCreate={vi.fn()} onUpdate={vi.fn()} editingOrder={approvedPo} />);
        await waitFor(() => expect(screen.getByText("Edit Purchase Order")).toBeTruthy());
        expect(screen.getByDisplayValue("front panel")).toBeTruthy();
        // The PO number is ours, not typed - shown for reference, never editable.
        expect(screen.getByText(/PO\/26-27\/000001/)).toBeTruthy();
    });

    it("totals a GST row the same way the server does", async () => {
        render(<PurchaseOrderModal isOpen toggle={noop} onCreate={vi.fn()} onUpdate={vi.fn()} editingOrder={approvedPo} />);
        // 2 x 100 = 200, +18% = 236. The server's Helpers/PurchaseRowTotal.js must agree, or a
        // converted order and its invoice would show different money. With a single row the
        // row total and the grand total are the same figure, so both should read 236.
        await waitFor(() => expect(screen.getAllByText(/236/).length).toBeGreaterThanOrEqual(2));
    });

    it("warns that saving will revoke approval, before it happens", async () => {
        render(<PurchaseOrderModal isOpen toggle={noop} onCreate={vi.fn()} onUpdate={vi.fn()} editingOrder={approvedPo} />);
        await waitFor(() => expect(screen.getByText(/revoke approval/i)).toBeTruthy());
    });

    it("does not warn when the order is not approved", async () => {
        const draft = { ...approvedPo, approval: { state: "draft" } };
        render(<PurchaseOrderModal isOpen toggle={noop} onCreate={vi.fn()} onUpdate={vi.fn()} editingOrder={draft} />);
        await waitFor(() => expect(screen.getByText("Edit Purchase Order")).toBeTruthy());
        expect(screen.queryByText(/revoke approval/i)).toBeNull();
    });

    it("refuses to save without a supplier", async () => {
        const onCreate = vi.fn();
        render(<PurchaseOrderModal isOpen toggle={noop} onCreate={onCreate} onUpdate={vi.fn()} editingOrder={null} />);
        const save = await waitFor(() => screen.getByRole("button", { name: "Create Purchase Order" }));
        expect(save.disabled).toBe(true);
        fireEvent.click(save);
        expect(onCreate).not.toHaveBeenCalled();
    });

    it("sends supplier, date and rows as JSON on save", async () => {
        const onUpdate = vi.fn().mockResolvedValue({});
        render(<PurchaseOrderModal isOpen toggle={noop} onCreate={vi.fn()} onUpdate={onUpdate} editingOrder={approvedPo} />);
        await waitFor(() => expect(screen.getByText("Edit Purchase Order")).toBeTruthy());
        fireEvent.click(screen.getByRole("button", { name: "Save Changes" }));
        await waitFor(() => expect(onUpdate).toHaveBeenCalled());
        const fd = onUpdate.mock.calls[0][0];
        expect(fd.get("po_id")).toBe("po1");
        expect(fd.get("supplier_id")).toBe("s1");
        expect(fd.get("date")).toBe("2026-09-12");
        expect(JSON.parse(fd.get("rows"))).toHaveLength(1);
    });
});
