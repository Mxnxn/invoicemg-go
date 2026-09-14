import React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const updateJob = vi.fn();
const MATERIALS = [{ _id: "m1", material_name: "Flex Banner", material_rate: 40, tax: 18 }];

vi.mock("../lifecycle_backend", () => ({
    lifecycleBackend: {
        updateJob: (...a) => updateJob(...a),
        lookupMaterials: () => Promise.resolve({ data: MATERIALS }),
    },
}));
vi.mock("../../../global/toast", () => ({ notifySuccess: () => {} }));

import NewCardPanel from "./NewCardPanel";

const rect = { top: 100, left: 100, width: 40, height: 20 };

// The row the panel actually posted, read back out of the FormData.
const sentRow = () => {
    const rows = JSON.parse(updateJob.mock.calls[0][0].get("rows"));
    return rows[rows.length - 1];
};

const fill = (label, value) => fireEvent.change(screen.getByLabelText(label), { target: { value } });

// Pick the seeded material the way a person does: open the trigger, click the option.
const pickProduct = async () => {
    fireEvent.click(await screen.findByRole("button", { name: /Product/ }));
    fireEvent.click(await screen.findByText("Flex Banner"));
};

const open = (rows) =>
    render(<NewCardPanel job={{ _id: "j1", rows }} rect={rect} onClose={() => {}} onCreated={() => {}} />);

describe("NewCardPanel", () => {
    beforeEach(() => {
        updateJob.mockReset();
        updateJob.mockResolvedValue({ data: { _id: "j1", rows: [] } });
    });

    it("carries the product's own rate and tax across when one is picked", async () => {
        open([{ cgst: 9, sgst: 9 }]);
        await pickProduct();

        expect(screen.getByLabelText("Rate").value).toBe("40");
        expect(screen.getByLabelText("GST %").value).toBe("18");
    });

    it("splits the tax across cgst and sgst on an intrastate job", async () => {
        open([{ cgst: 9, sgst: 9 }]);
        await pickProduct();
        fill("Length", "2");
        fill("Width", "3");
        fireEvent.click(screen.getByText("Add card"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const row = sentRow();
        expect(row.cgst).toBe(9);
        expect(row.sgst).toBe(9);
        expect(row.igst).toBe(0);
    });

    it("puts the whole rate on igst when the job is already interstate", async () => {
        open([{ igst: 18 }]);
        await pickProduct();
        fill("Length", "2");
        fill("Width", "3");
        fireEvent.click(screen.getByText("Add card"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const row = sentRow();
        expect(row.igst).toBe(18);
        expect(row.cgst).toBe(0);
        expect(row.sgst).toBe(0);
    });

    it("asks the regime only when the job has no rows to answer it", () => {
        open([{ igst: 18 }]);
        expect(screen.getByLabelText("IGST %")).toBeTruthy();
        expect(screen.queryByText(/Interstate sale/)).toBeNull();
    });

    it("offers the regime on a job with no rows, and honours it", async () => {
        open([]);
        fireEvent.click(screen.getByText(/Interstate sale/));
        await pickProduct();
        fill("Length", "2");
        fill("Width", "3");
        fireEvent.click(screen.getByText("Add card"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        expect(sentRow().igst).toBe(18);
    });

    it("shows length and width only when the card is measured by size", () => {
        open([]);
        expect(screen.getByLabelText("Length")).toBeTruthy();

        fireEvent.click(screen.getByText("By quantity"));
        expect(screen.queryByLabelText("Length")).toBeNull();
        expect(screen.queryByLabelText("Width")).toBeNull();
        expect(screen.getByLabelText("Quantity")).toBeTruthy();
    });

    // The factors rowAmount multiplies by. A by-quantity card that sent 0 for length and width
    // would total nothing however it was priced.
    it("sends 1 for the dimensions of a card counted by the piece", async () => {
        open([{ cgst: 9, sgst: 9 }]);
        fireEvent.click(screen.getByText("By quantity"));
        await pickProduct();
        fill("Quantity", "5");
        fireEvent.click(screen.getByText("Add card"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const row = sentRow();
        expect(row.hasDimensions).toBe(false);
        expect(row.length).toBe("1");
        expect(row.width).toBe("1");
        expect(row.qty).toBe(5);
    });

    it("refuses a card with no product rather than creating a blank line", async () => {
        open([]);
        fireEvent.click(screen.getByText("Add card"));

        await screen.findByText("A card needs a product.");
        expect(updateJob).not.toHaveBeenCalled();
    });

    it("refuses a card measured by size with no size", async () => {
        open([{ cgst: 9, sgst: 9 }]);
        await pickProduct();
        fireEvent.click(screen.getByText("Add card"));

        await screen.findByText("A card measured by size needs both a length and a width.");
        expect(updateJob).not.toHaveBeenCalled();
    });

    it("keeps every existing row and appends the new one", async () => {
        open([{ _id: "r1", material: "Old", cgst: 9, sgst: 9 }]);
        await pickProduct();
        fill("Length", "2");
        fill("Width", "3");
        fireEvent.click(screen.getByText("Add card"));

        await waitFor(() => expect(updateJob).toHaveBeenCalled());
        const rows = JSON.parse(updateJob.mock.calls[0][0].get("rows"));
        expect(rows).toHaveLength(2);
        expect(rows[0].material).toBe("Old");
        expect(rows[1].material).toBe("Flex Banner");
    });
});
