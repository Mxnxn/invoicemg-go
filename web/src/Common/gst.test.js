import { describe, it, expect } from "vitest";
import { splitGst, igstOnly, combinedGst } from "./gst";

describe("splitGst", () => {
	it("halves the product rate across CGST and SGST", () => {
		expect(splitGst(18)).toEqual({ cgst: 9, sgst: 9 });
		expect(splitGst(28)).toEqual({ cgst: 14, sgst: 14 });
	});

	it("keeps odd slabs exact rather than rounding to whole numbers", () => {
		expect(splitGst(5)).toEqual({ cgst: 2.5, sgst: 2.5 });
	});

	it("handles zero-rated and junk without producing NaN", () => {
		expect(splitGst(0)).toEqual({ cgst: 0, sgst: 0 });
		expect(splitGst(undefined)).toEqual({ cgst: 0, sgst: 0 });
		expect(splitGst("18")).toEqual({ cgst: 9, sgst: 9 });
		expect(splitGst("nonsense")).toEqual({ cgst: 0, sgst: 0 });
	});

	it("does not touch igst - spreading it onto a row must not clear an interstate rate", () => {
		expect("igst" in splitGst(18)).toBe(false);
	});

	it("always sums back to the product's rate", () => {
		[0, 5, 12, 18, 28].forEach((rate) => {
			expect(combinedGst(splitGst(rate))).toBe(rate);
		});
	});
});

describe("igstOnly", () => {
	it("puts the whole rate on IGST for an interstate sale", () => {
		expect(igstOnly(18)).toEqual({ cgst: 0, sgst: 0, igst: 18 });
		expect(combinedGst(igstOnly(28))).toBe(28);
	});
});
