import { describe, expect, it } from "vitest";
import { buildSheetModel, imageExtensionForLogo, sheetNameFor } from "./exportWorkbook";

const columns = [
    { key: "date", label: "Date" },
    { key: "party", label: "Party" },
    { key: "amount", label: "Amount", numeric: true },
];
const rows = [
    ["2026-01-02", "Sharma Packaging", 48200],
    ["2026-01-03", "A very much longer party name than the others", 1120000],
];
const company = {
    firm: "Manan Graphics",
    address: "12 Press Lane, Ahmedabad",
    phone: "9825507071",
    gst: "24ABCDE1234F1Z5",
    url: "logo.png",
};
const template = { logo: true, firm: true, address: true, phone: true, gst: true };

describe("buildSheetModel", () => {
    it("puts the company on the sheet", () => {
        const model = buildSheetModel({ title: "Ledger", columns, rows, company, template });
        expect(model.headerLines).toContain("Manan Graphics");
        expect(model.headerLines).toContain("12 Press Lane, Ahmedabad");
        expect(model.headerLines.some((l) => l.includes("24ABCDE1234F1Z5"))).toBe(true);
    });

    it("omits the fields the template turns off", () => {
        const model = buildSheetModel({
            title: "Ledger",
            columns,
            rows,
            company,
            template: { logo: false, firm: true, address: false, phone: false, gst: false },
        });
        expect(model.headerLines).toContain("Manan Graphics");
        expect(model.headerLines).not.toContain("12 Press Lane, Ahmedabad");
        expect(model.headerLines.some((l) => l.includes("24ABCDE1234F1Z5"))).toBe(false);
        expect(model.logo).toBeNull();
    });

    it("renders a text header for a company with no logo, rather than nothing to draw", () => {
        const model = buildSheetModel({ title: "Ledger", columns, rows, company: { ...company, url: "" }, template });
        expect(model.logo).toBeNull();
        expect(model.headerLines).toContain("Manan Graphics");
    });

    it("survives having no company at all", () => {
        const model = buildSheetModel({ title: "Ledger", columns, rows, company: null, template });
        expect(model.headerLines).toEqual([]);
        expect(model.logo).toBeNull();
        // The report still exports; it simply has no letterhead.
        expect(model.startRow).toBeGreaterThan(0);
    });

    it("caps column widths so a long cell does not render as ####", () => {
        const model = buildSheetModel({ title: "Ledger", columns, rows, company, template });
        expect(model.columnWidths).toHaveLength(3);
        model.columnWidths.forEach((w) => {
            expect(w).toBeGreaterThanOrEqual(10);
            expect(w).toBeLessThanOrEqual(48);
        });
        // The long party name widens its own column beyond the date column.
        expect(model.columnWidths[1]).toBeGreaterThan(model.columnWidths[0]);
    });

    it("starts the data below the header, leaving room for what is actually shown", () => {
        const withHeader = buildSheetModel({ title: "Ledger", columns, rows, company, template });
        const without = buildSheetModel({
            title: "Ledger",
            columns,
            rows,
            company,
            template: { logo: false, firm: false, address: false, phone: false, gst: false },
        });
        expect(withHeader.startRow).toBeGreaterThan(without.startRow);
    });

    it("only reserves a totals row when a foot is supplied", () => {
        const withFoot = buildSheetModel({ title: "Ledger", columns, rows, company, template, foot: ["", "Total", 1168200] });
        const withoutFoot = buildSheetModel({ title: "Ledger", columns, rows, company, template });
        expect(withFoot.hasFoot).toBe(true);
        expect(withoutFoot.hasFoot).toBe(false);
    });

    it("right-aligns numeric columns and left-aligns everything else", () => {
        const model = buildSheetModel({ title: "Ledger", columns, rows, company, template });
        expect(model.alignments).toEqual(["left", "left", "right"]);
    });

    it("reserves a row per summary item, pushing the table down", () => {
        const summary = [
            { label: "Total Billed", value: "₹1,20,000" },
            { label: "Total Received", value: "₹80,000" },
        ];
        const withoutSummary = buildSheetModel({ title: "Ledger", columns, rows, company, template });
        const withSummary = buildSheetModel({ title: "Ledger", columns, rows, company, template, summary });
        expect(withSummary.summaryRows).toBe(2);
        expect(withSummary.startRow).toBe(withoutSummary.startRow + 2);
    });

    it("omitting summary changes nothing", () => {
        const withDefault = buildSheetModel({ title: "Ledger", columns, rows, company, template });
        const withEmptyArray = buildSheetModel({ title: "Ledger", columns, rows, company, template, summary: [] });
        expect(withEmptyArray).toEqual(withDefault);
        expect(withDefault.summaryRows).toBe(0);
    });

    // Pinned row numbers, not just "greater than" - a wrong offset here would silently
    // overwrite the subtitle with the summary block, or the summary with the header row, and
    // no earlier test would notice. This company has a 3-line letterhead plus a logo, so
    // letterheadRows is LOGO_ROWS (max(3, 6)) for all four cases below - the logo is taller
    // than the text beside it, so it is the logo that sets the height.
    describe("row positions across ±subtitle × ±summary", () => {
        const twoItemSummary = [
            { label: "Total Billed", value: "₹1,20,000" },
            { label: "Total Received", value: "₹80,000" },
        ];

        it("neither subtitle nor summary: title sits right after the letterhead gap", () => {
            const model = buildSheetModel({ title: "Ledger", columns, rows, company, template });
            expect(model.startRow).toBe(10);
            expect(model.titleRow).toBe(8);
            expect(model.subtitleRow).toBeNull();
            expect(model.summaryStartRow).toBeNull();
        });

        it("subtitle only: title unmoved, subtitle right below it", () => {
            const model = buildSheetModel({ title: "Ledger", subtitle: "Sharma Packaging", columns, rows, company, template });
            expect(model.startRow).toBe(11);
            expect(model.titleRow).toBe(8);
            expect(model.subtitleRow).toBe(9);
            expect(model.summaryStartRow).toBeNull();
        });

        it("summary only: title unmoved, summary block right below it", () => {
            const model = buildSheetModel({ title: "Ledger", columns, rows, company, template, summary: twoItemSummary });
            expect(model.startRow).toBe(12);
            expect(model.titleRow).toBe(8);
            expect(model.subtitleRow).toBeNull();
            expect(model.summaryStartRow).toBe(9);
        });

        it("subtitle and summary together: title still unmoved, subtitle then summary cascade below it", () => {
            const model = buildSheetModel({
                title: "Ledger",
                subtitle: "Sharma Packaging",
                columns,
                rows,
                company,
                template,
                summary: twoItemSummary,
            });
            expect(model.startRow).toBe(13);
            expect(model.titleRow).toBe(8);
            expect(model.subtitleRow).toBe(9);
            expect(model.summaryStartRow).toBe(10);
        });
    });

    describe("showPeriod template flag", () => {
        it("suppresses the subtitle row when the toggle is off", () => {
            const withPeriod = buildSheetModel({ title: "Ledger", subtitle: "Sharma Packaging", columns, rows, company, template });
            const withoutPeriod = buildSheetModel({
                title: "Ledger",
                subtitle: "Sharma Packaging",
                columns,
                rows,
                company,
                template: { ...template, showPeriod: false },
            });
            expect(withPeriod.subtitleRow).not.toBeNull();
            expect(withoutPeriod.subtitleRow).toBeNull();
            // No reserved row for it either - the table climbs back up to where it would sit
            // with no subtitle at all, not just an empty gap where one would have been.
            const noSubtitleAtAll = buildSheetModel({ title: "Ledger", columns, rows, company, template });
            expect(withoutPeriod.startRow).toBe(noSubtitleAtAll.startRow);
        });

        it("showPeriod defaults to on when the template omits it", () => {
            const model = buildSheetModel({ title: "Ledger", subtitle: "Sharma Packaging", columns, rows, company, template: {} });
            expect(model.subtitleRow).not.toBeNull();
        });
    });
});

describe("imageExtensionForLogo", () => {
    // ExcelJS's addImage renders only png/jpeg/gif. A company's logo can be webp or avif
    // (the API's ImageUpload.js accepts both) - those, and anything unrecognised, must resolve
    // to null so the caller skips the image instead of mislabelling it.
    it("maps supported extensions to what ExcelJS accepts", () => {
        expect(imageExtensionForLogo("https://api/uploads/logo.png")).toBe("png");
        expect(imageExtensionForLogo("https://api/uploads/logo.jpg")).toBe("jpeg");
        expect(imageExtensionForLogo("https://api/uploads/logo.jpeg")).toBe("jpeg");
        expect(imageExtensionForLogo("https://api/uploads/logo.gif")).toBe("gif");
        expect(imageExtensionForLogo("https://api/uploads/LOGO.PNG")).toBe("png");
    });

    it("resolves an unsupported or missing extension to null rather than guessing jpeg", () => {
        expect(imageExtensionForLogo("https://api/uploads/logo.webp")).toBeNull();
        expect(imageExtensionForLogo("https://api/uploads/logo.avif")).toBeNull();
        expect(imageExtensionForLogo("https://api/uploads/logo")).toBeNull();
        expect(imageExtensionForLogo("")).toBeNull();
        expect(imageExtensionForLogo(undefined)).toBeNull();
    });

    it("ignores a query string after the extension", () => {
        expect(imageExtensionForLogo("https://api/uploads/logo.png?v=2")).toBe("png");
        expect(imageExtensionForLogo("https://api/uploads/logo.webp?v=2")).toBeNull();
    });
});

describe("sheetNameFor", () => {
    it("strips characters Excel rejects in a sheet name", () => {
        expect(sheetNameFor("A:B\\C/D?E*F[G]H")).toBe("ABCDEFGH");
    });

    it("caps the name at 31 characters", () => {
        const long = "A".repeat(50);
        expect(sheetNameFor(long)).toHaveLength(31);
    });

    it("falls back to Report when there is no title", () => {
        expect(sheetNameFor("")).toBe("Report");
        expect(sheetNameFor(undefined)).toBe("Report");
    });
});
