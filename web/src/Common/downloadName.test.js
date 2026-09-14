import { describe, it, expect } from "vitest";
import { firmSlug, isoDate, downloadName } from "./downloadName";

describe("firmSlug", () => {
	it("joins words without separators, so the underscore stays the date separator", () => {
		expect(firmSlug("Plain Firm")).toBe("PlainFirm");
		expect(firmSlug("manan graphics")).toBe("MananGraphics");
	});

	it("drops characters a filesystem would object to", () => {
		expect(firmSlug("A/B\C:D*E?F")).toBe("ABCDEF");
		expect(firmSlug("Acme & Co. (Pvt) Ltd")).toBe("AcmeCoPvtLtd");
	});

	it("falls back rather than producing an empty name", () => {
		expect(firmSlug("")).toBe("Firm");
		expect(firmSlug("///")).toBe("Firm");
		expect(firmSlug(null)).toBe("Firm");
	});

	it("caps the length so a downloads list stays readable", () => {
		expect(firmSlug("A".repeat(80)).length).toBe(40);
	});
});

describe("downloadName", () => {
	it("is date_firm.ext", () => {
		expect(downloadName({ firm: "Plain Firm", ext: "pdf", date: "2026-08-24" })).toBe("2026-08-24_PlainFirm.pdf");
		expect(downloadName({ firm: "Plain Firm", ext: "xlsx", date: "2026-08-24" })).toBe("2026-08-24_PlainFirm.xlsx");
	});

	it("adds a suffix so two same-day exports do not overwrite each other", () => {
		expect(downloadName({ firm: "Acme", ext: "pdf", date: "2026-08-24", suffix: "Bank Report" })).toBe(
			"2026-08-24_Acme_BankReport.pdf"
		);
	});

	it("tolerates a leading dot on the extension", () => {
		expect(downloadName({ firm: "Acme", ext: ".pdf", date: "2026-08-24" })).toBe("2026-08-24_Acme.pdf");
	});

	it("falls back to today for a missing or unparseable date", () => {
		const today = new Date().toISOString().slice(0, 10);
		expect(downloadName({ firm: "Acme", ext: "pdf" })).toBe(`${today}_Acme.pdf`);
		expect(isoDate("not a date")).toBe(today);
	});
});
