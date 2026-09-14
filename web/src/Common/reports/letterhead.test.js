import { describe, expect, it } from "vitest";
import { headerLinesFor, wantsLogo, showsSubtitle } from "./letterhead";

const company = {
    firm: "Northwind Press",
    address: "14 Harbour Road, Ahmedabad",
    phone: "9800000000",
    gst: "24DDDDD0000D1Z5",
    url: "logo.png",
};

describe("letterhead", () => {
    it("prints the whole block when nothing is turned off", () => {
        expect(headerLinesFor(company)).toEqual([
            "Northwind Press",
            "14 Harbour Road, Ahmedabad",
            "9800000000  ·  GST 24DDDDD0000D1Z5",
        ]);
    });

    // Absent means ON. A company created before exportTemplate existed has an empty object,
    // and must keep the letterhead it has always had rather than losing it silently.
    it("treats an absent toggle as on", () => {
        expect(headerLinesFor(company, {})).toHaveLength(3);
        expect(headerLinesFor(company, { firm: undefined }).includes("Northwind Press")).toBe(true);
    });

    it("honours each toggle", () => {
        expect(headerLinesFor(company, { firm: false })).not.toContain("Northwind Press");
        expect(headerLinesFor(company, { address: false })).not.toContain("14 Harbour Road, Ahmedabad");
        expect(headerLinesFor(company, { phone: false })[2]).toBe("GST 24DDDDD0000D1Z5");
        expect(headerLinesFor(company, { gst: false })[2]).toBe("9800000000");
    });

    // Both off means no contact line at all, rather than an empty one that prints as a blank
    // row in the spreadsheet and a gap on the page.
    it("drops the contact line entirely when both halves are off", () => {
        expect(headerLinesFor(company, { phone: false, gst: false })).toHaveLength(2);
    });

    it("returns nothing for a missing company", () => {
        expect(headerLinesFor(null)).toEqual([]);
        expect(headerLinesFor(undefined, { firm: true })).toEqual([]);
    });

    // A company with a field empty must not contribute a blank line.
    it("skips empty fields", () => {
        expect(headerLinesFor({ firm: "Acme", address: "", phone: "", gst: "" })).toEqual(["Acme"]);
    });

    describe("wantsLogo", () => {
        it("needs both the toggle and a file", () => {
            expect(wantsLogo(company)).toBe(true);
            expect(wantsLogo(company, { logo: false })).toBe(false);
            expect(wantsLogo({ ...company, url: "" })).toBe(false);
            expect(wantsLogo(null)).toBe(false);
        });
    });

    describe("showsSubtitle", () => {
        it("needs both the toggle and a subtitle", () => {
            expect(showsSubtitle("Apr - Jun")).toBe(true);
            expect(showsSubtitle("Apr - Jun", { showPeriod: false })).toBe(false);
            expect(showsSubtitle("")).toBe(false);
        });
    });
});
