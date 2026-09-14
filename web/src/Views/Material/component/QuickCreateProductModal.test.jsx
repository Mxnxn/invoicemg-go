import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const addNewMaterial = vi.fn();
const editMaterial = vi.fn();
vi.mock("../material_backend", () => ({
    materialsBackend: {
        addNewMaterial: (...a) => addNewMaterial(...a),
        editMaterial: (...a) => editMaterial(...a),
    },
}));

const list = vi.fn();
vi.mock("../../../Common/unit_backend", () => ({ unitBackend: { list: (...a) => list(...a) } }));

import QuickCreateProductModal from "./QuickCreateProductModal";

const fill = (label, value) => fireEvent.change(screen.getByLabelText(label), { target: { value } });

beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue({ data: [{ _id: "u1", name: "SQ. FT" }, { _id: "u2", name: "PCS" }] });
    addNewMaterial.mockResolvedValue({ data: { _id: "m1", material_name: "Vinyl" } });
    editMaterial.mockResolvedValue({ data: { _id: "m1", material_name: "Vinyl Matte" } });
});

const openCreate = (props = {}) =>
    render(<QuickCreateProductModal isOpen toggle={() => {}} onCreated={() => {}} onUpdated={() => {}} {...props} />);

describe("creating", () => {
    it("offers the units the company has configured", async () => {
        openCreate();
        await waitFor(() => expect(screen.getByRole("option", { name: "SQ. FT" })).toBeDefined());
    });

    // The bug this test exists for. Configure > Products made `unit` required, but this
    // dialog - the one the job card opens - never had the field, so a product created mid-job
    // was saved with no unit and then sat under "Not counted" on the inventory report.
    it("will not submit without a unit", async () => {
        openCreate();
        await waitFor(() => expect(screen.getByRole("option", { name: "SQ. FT" })).toBeDefined());

        fill(/Product name/i, "Vinyl");
        fill(/Sale rate/i, "120");
        fill(/Purchase rate/i, "80");
        fill(/HSN/i, "3919");

        const submit = screen.getByRole("button", { name: /Add product/i });
        expect(submit.disabled).toBe(true);
        fireEvent.click(submit);
        expect(addNewMaterial).not.toHaveBeenCalled();
    });

    it("sends the unit along with everything else", async () => {
        const onCreated = vi.fn();
        openCreate({ onCreated });
        await waitFor(() => expect(screen.getByRole("option", { name: "SQ. FT" })).toBeDefined());

        fill(/Product name/i, "Vinyl");
        fill(/Sale rate/i, "120");
        fill(/Purchase rate/i, "80");
        fill(/HSN/i, "3919");
        fill(/Unit/i, "SQ. FT");

        fireEvent.click(screen.getByRole("button", { name: /Add product/i }));

        await waitFor(() => expect(addNewMaterial).toHaveBeenCalled());
        const posted = addNewMaterial.mock.calls[0][0];
        expect(posted.get("material_name")).toBe("Vinyl");
        expect(posted.get("unit")).toBe("SQ. FT");
        expect(posted.get("hsn")).toBe("3919");
        expect(editMaterial).not.toHaveBeenCalled();
        await waitFor(() => expect(onCreated).toHaveBeenCalled());
    });
});

describe("editing", () => {
    const product = {
        id: "m1",
        material_name: "Vinyl",
        material_rate: 120,
        purchase_rate: 80,
        hsn: "3919",
        tax: 18,
        unit: "SQ. FT",
        priceHistory: [{ changed_at: "2026-04-01", material_rate: 100, purchase_rate: 70 }],
    };

    it("opens with the product's own values", async () => {
        openCreate({ product });
        await waitFor(() => expect(screen.getByLabelText(/Product name/i).value).toBe("Vinyl"));
        expect(screen.getByLabelText(/HSN/i).value).toBe("3919");
        expect(screen.getByLabelText(/Unit/i).value).toBe("SQ. FT");
    });

    // One dialog serving both is the whole point; sending a create for an existing product
    // would silently duplicate it.
    it("updates rather than creates, and carries the id", async () => {
        const onUpdated = vi.fn();
        openCreate({ product, onUpdated });
        await waitFor(() => expect(screen.getByLabelText(/Product name/i).value).toBe("Vinyl"));

        fill(/Product name/i, "Vinyl Matte");
        fireEvent.click(screen.getByRole("button", { name: /Save changes/i }));

        await waitFor(() => expect(editMaterial).toHaveBeenCalled());
        const posted = editMaterial.mock.calls[0][0];
        expect(posted.get("material_id")).toBe("m1");
        expect(posted.get("material_name")).toBe("Vinyl Matte");
        expect(addNewMaterial).not.toHaveBeenCalled();
        await waitFor(() => expect(onUpdated).toHaveBeenCalled());
    });

    it("shows the price history it has", async () => {
        openCreate({ product });
        await waitFor(() => expect(screen.getByText(/Price history/i)).toBeDefined());
    });

    it("does not show a price history section when creating", async () => {
        openCreate();
        await waitFor(() => expect(screen.getByRole("option", { name: "SQ. FT" })).toBeDefined());
        expect(screen.queryByText(/Price history/i)).toBeNull();
    });
});
