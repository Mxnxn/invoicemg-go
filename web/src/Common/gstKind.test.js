import { describe, it, expect } from "vitest";
import { rowIsIgst, hasIgst, invoiceGstKind, filterInvoicesByGst, countInvoicesByGst } from "./gstKind";

const intra = { cgst: 9, sgst: 9, igst: 0 };
const inter = { cgst: 0, sgst: 0, igst: 18 };
const zero = { cgst: 0, sgst: 0, igst: 0 };

describe("rowIsIgst", () => {
    it("is true only when IGST is actually charged", () => {
        expect(rowIsIgst(inter)).toBe(true);
        expect(rowIsIgst(intra)).toBe(false);
        expect(rowIsIgst(zero)).toBe(false);
    });

    it("survives missing or string values", () => {
        expect(rowIsIgst({})).toBe(false);
        expect(rowIsIgst(undefined)).toBe(false);
        expect(rowIsIgst({ igst: "18" })).toBe(true);
    });
});

describe("invoiceGstKind", () => {
    it("classifies an intrastate invoice as gst", () => {
        expect(invoiceGstKind({ entries: [intra, intra] })).toBe("gst");
    });

    it("classifies an interstate invoice as igst", () => {
        expect(invoiceGstKind({ entries: [inter] })).toBe("igst");
    });

    // A mixed invoice should not hide in the GST list - the IGST line is the one that needs
    // filing attention.
    it("counts a mixed invoice as igst", () => {
        expect(invoiceGstKind({ entries: [intra, inter] })).toBe("igst");
    });

    it("treats a zero-tax or empty invoice as gst rather than throwing", () => {
        expect(invoiceGstKind({ entries: [zero] })).toBe("gst");
        expect(invoiceGstKind({ entries: [] })).toBe("gst");
        expect(invoiceGstKind({})).toBe("gst");
    });
});

describe("hasIgst", () => {
    it("is true if any row carries IGST", () => {
        expect(hasIgst([intra, intra, inter])).toBe(true);
        expect(hasIgst([intra, intra])).toBe(false);
        expect(hasIgst([])).toBe(false);
    });
});

describe("filterInvoicesByGst / countInvoicesByGst", () => {
    const list = [
        { _id: 1, entries: [intra] },
        { _id: 2, entries: [inter] },
        { _id: 3, entries: [intra, inter] },
        { _id: 4, entries: [] },
    ];

    it("all returns the list untouched", () => {
        expect(filterInvoicesByGst(list, "all")).toHaveLength(4);
    });

    it("splits the two kinds without losing any", () => {
        const gst = filterInvoicesByGst(list, "gst");
        const igst = filterInvoicesByGst(list, "igst");
        expect(gst.map((i) => i._id)).toEqual([1, 4]);
        expect(igst.map((i) => i._id)).toEqual([2, 3]);
        expect(gst.length + igst.length).toBe(list.length);
    });

    it("counts match the filters", () => {
        expect(countInvoicesByGst(list)).toEqual({ all: 4, gst: 2, igst: 2 });
    });
});
