import { describe, it, expect } from "vitest";
import {
	clampScale, fontStack, readAppearance, writeAppearance, applyAppearance,
	DEFAULT_APPEARANCE, SCALE_MIN, SCALE_MAX, FONT_OPTIONS, FONT_GROUPS, SCALE_ROLES,
	scaleToPx, pxToScale, pxRange,
	ROWS_PER_PAGE_OPTIONS, clampRowsPerPage,
} from "./appearance";

const fakeStorage = (initial = {}) => {
	const map = { ...initial };
	return {
		getItem: (k) => (k in map ? map[k] : null),
		setItem: (k, v) => { map[k] = String(v); },
		_map: map,
	};
};

describe("clampScale", () => {
	it("keeps a sensible value", () => {
		expect(clampScale(1)).toBe(1);
		expect(clampScale(1.15)).toBe(1.15);
	});

	it("clamps to the bounds the shell survives", () => {
		expect(clampScale(0.2)).toBe(SCALE_MIN);
		expect(clampScale(9)).toBe(SCALE_MAX);
	});

	it("falls back rather than writing NaN into a CSS variable", () => {
		expect(clampScale("nonsense")).toBe(DEFAULT_APPEARANCE.scale);
		expect(clampScale(undefined)).toBe(DEFAULT_APPEARANCE.scale);
	});
});

describe("readAppearance", () => {
	it("defaults when nothing is stored", () => {
		expect(readAppearance(fakeStorage())).toEqual(DEFAULT_APPEARANCE);
	});

	it("rejects a font that is not on the list", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "comic-sans", scale: 1 }) });
		expect(readAppearance(s).font).toBe(DEFAULT_APPEARANCE.font);
	});

	it("survives corrupt storage", () => {
		expect(readAppearance(fakeStorage({ appearance: "{not json" }))).toEqual(DEFAULT_APPEARANCE);
	});

	it("clamps a stored scale that is out of range", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { heading: 99, body: 1, tableHead: 1, tableRow: 1 } }) });
		expect(readAppearance(s).scales.heading).toBe(SCALE_MAX);
	});

	// The single slider predates the per-role split. Someone who had set 120% should keep it
	// across all three rather than being silently reset to 100%.
	it("carries a legacy single scale onto every role", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scale: 1.2 }) });
		expect(readAppearance(s).scales).toEqual({ heading: 1.2, body: 1.2, tableHead: 1.2, tableRow: 1.2, button: 1.2, cardTitle: 1.2, card: 1.2 });
	});
});

describe("writeAppearance", () => {
	it("round-trips through storage", () => {
		const s = fakeStorage();
		writeAppearance(
			{ font: "outfit", scales: { heading: 1.1, body: 0.9, tableHead: 1.3, tableRow: 0.95, button: 1.05, cardTitle: 1.2, card: 1.15 }, tracking: 0.25 },
			s
		);
		// accent/gradient/rowsPerPage fall back to the defaults here - this call omits them,
		// which is also what a preference stored before those settings existed looks like.
		expect(readAppearance(s)).toEqual({
			font: "outfit",
			scales: { heading: 1.1, body: 0.9, tableHead: 1.3, tableRow: 0.95, button: 1.05, cardTitle: 1.2, card: 1.15 },
			tracking: 0.25,
			accent: "blue",
			gradient: "none",
			rowsPerPage: 25,
		});
	});

	it("round-trips a chosen accent and gradient", () => {
		const s = fakeStorage();
		writeAppearance({ ...DEFAULT_APPEARANCE, accent: "violet", gradient: "ocean" }, s);
		const back = readAppearance(s);
		expect(back.accent).toBe("violet");
		expect(back.gradient).toBe("ocean");
	});

	it("falls back to the default accent when the stored id is unknown", () => {
		const s = fakeStorage();
		s.setItem("appearance", JSON.stringify({ font: "inter", accent: "chartreuse", gradient: "nope" }));
		const back = readAppearance(s);
		expect(back.accent).toBe("blue");
		expect(back.gradient).toBe("none");
	});

	it("does not throw when storage refuses to write", () => {
		const hostile = { getItem: () => null, setItem: () => { throw new Error("private mode"); } };
		expect(() => writeAppearance({ font: "inter", scales: { heading: 1, body: 1, tableHead: 1, tableRow: 1 } }, hostile)).not.toThrow();
	});
});

describe("applyAppearance", () => {
	it("sets a variable per role, so each sizes independently", () => {
		const root = document.createElement("div");
		applyAppearance({ font: "inter", scales: { heading: 1.2, body: 0.9, tableHead: 1.3, tableRow: 0.95 } }, root);
		expect(root.style.getPropertyValue("--font-scale-heading")).toBe("1.2");
		expect(root.style.getPropertyValue("--font-scale-body")).toBe("0.9");
		// camelCase ids become kebab-case variables, which is what the stylesheets read.
		expect(root.style.getPropertyValue("--font-scale-table-head")).toBe("1.3");
		expect(root.style.getPropertyValue("--font-scale-table-row")).toBe("0.95");
		expect(root.style.getPropertyValue("--font-family")).toContain("Inter");
		expect(root.style.getPropertyValue("--font-outfit")).toContain("Inter");
		// Tables read --font-mono; without this they stayed pinned to JetBrains Mono while
		// the rest of the app changed.
		expect(root.style.getPropertyValue("--font-mono")).toContain("Inter");
	});

	it("uses defaults for a missing appearance", () => {
		const root = document.createElement("div");
		applyAppearance(undefined, root);
		expect(root.style.getPropertyValue("--font-scale-heading")).toBe("1");
		expect(root.style.getPropertyValue("--font-scale-body")).toBe("1");
		expect(root.style.getPropertyValue("--font-family")).toBe(fontStack(DEFAULT_APPEARANCE.font));
	});

	it("every offered font has a real stack", () => {
		FONT_OPTIONS.forEach((f) => expect(fontStack(f.id)).toBe(f.stack));
	});
});

describe("letter spacing", () => {
	it("defaults to the value the tables shipped with", () => {
		expect(readAppearance(fakeStorage()).tracking).toBe(0.1);
	});

	// Anyone who stored a preference before this setting existed had the fixed 0.1px; absent
	// must read as that rather than 0, or their tables would tighten on upgrade.
	it("treats a stored value from before this setting as the default", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { heading: 1, body: 1, tableHead: 1, tableRow: 1 } }) });
		expect(readAppearance(s).tracking).toBe(0.1);
	});

	it("clamps beyond the readable range", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", tracking: 9 }) });
		expect(readAppearance(s).tracking).toBe(1);
	});

	it("drives the variable the table reads", () => {
		const root = document.createElement("div");
		applyAppearance({ font: "inter", scales: { heading: 1, body: 1, tableHead: 1, tableRow: 1 }, tracking: 0.4 }, root);
		expect(root.style.getPropertyValue("--letter-spacing-table")).toBe("0.4px");
	});
});

describe("table head/row split", () => {
	// One "table" control became two. A stored preference must land on both, or someone who
	// had set 120% would find their headers and rows back at 100%.
	it("carries a stored table scale onto headers and rows", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { heading: 1, body: 1, table: 1.2 } }) });
		const { scales } = readAppearance(s);
		expect(scales.tableHead).toBe(1.2);
		expect(scales.tableRow).toBe(1.2);
	});

	it("prefers an explicit head/row value over the old combined one", () => {
		const s = fakeStorage({
			appearance: JSON.stringify({ font: "inter", scales: { heading: 1, body: 1, table: 1.2, tableRow: 0.9 } }),
		});
		const { scales } = readAppearance(s);
		expect(scales.tableHead).toBe(1.2);
		expect(scales.tableRow).toBe(0.9);
	});
});

describe("px conversion", () => {
	// The box is in px because that is what a person means by "font size". A role covers
	// several sizes, so it converts against the representative element the preview shows.
	it("round-trips a size through the multiplier", () => {
		expect(scaleToPx("body", 1)).toBe(14);
		expect(pxToScale("body", 14)).toBe(1);
		expect(scaleToPx("body", pxToScale("body", 16))).toBe(16);
	});

	it("uses each role's own base", () => {
		expect(scaleToPx("heading", 1)).toBe(23.5);
		expect(scaleToPx("tableHead", 1)).toBe(11);
		expect(scaleToPx("tableRow", 1)).toBe(13);
	});

	// Typing a size the shell cannot survive lands on the nearest one it can, rather than
	// being accepted and quietly ignored.
	it("clamps to the role's range", () => {
		const { min, max } = pxRange("tableRow");
		expect(scaleToPx("tableRow", pxToScale("tableRow", 900))).toBe(max);
		expect(scaleToPx("tableRow", pxToScale("tableRow", 1))).toBe(min);
	});

	it("ignores junk rather than collapsing to zero", () => {
		expect(pxToScale("body", "abc")).toBe(1);
		expect(pxToScale("body", 0)).toBe(1);
		expect(pxToScale("body", -5)).toBe(1);
	});
});

describe("button size", () => {
	it("defaults to 13px, smaller than body text", () => {
		expect(scaleToPx("button", 1)).toBe(13);
		expect(scaleToPx("body", 1)).toBe(14);
	});

	// .shell-btn reads an absolute px variable, not a multiplier - it is resolved here so the
	// stylesheet does not multiply it by the body scale a second time.
	it("resolves to an absolute px variable", () => {
		const root = document.createElement("div");
		applyAppearance({ font: "inter", scales: { ...DEFAULT_APPEARANCE.scales, button: 1.2 } }, root);
		expect(root.style.getPropertyValue("--button-font-size")).toBe("15.6px");
	});
});

describe("rowsPerPage", () => {
	it("offers exactly the sizes the settings screen lists, All first", () => {
		expect(ROWS_PER_PAGE_OPTIONS.map((o) => o.value)).toEqual([0, 10, 15, 25, 50, 100, 200]);
	});

	it("keeps a value that is on the list", () => {
		expect(clampRowsPerPage(50)).toBe(50);
		// 0 is All, not "no rows" - it has to survive rather than being treated as falsy.
		expect(clampRowsPerPage(0)).toBe(0);
	});

	it("snaps a value that is not offered back to the default", () => {
		// Reaches this from localStorage, which an older release or a person with devtools
		// can leave anything at all in.
		expect(clampRowsPerPage(37)).toBe(25);
		expect(clampRowsPerPage(-5)).toBe(25);
		expect(clampRowsPerPage("banana")).toBe(25);
		expect(clampRowsPerPage(undefined)).toBe(25);
	});

	it("round-trips through storage", () => {
		const s = fakeStorage();
		writeAppearance({ ...DEFAULT_APPEARANCE, rowsPerPage: 100 }, s);
		expect(readAppearance(s).rowsPerPage).toBe(100);
	});

	it("gives a preference stored before this setting the default rather than All", () => {
		const s = fakeStorage();
		s.setItem("appearance", JSON.stringify({ font: "inter" }));
		expect(readAppearance(s).rowsPerPage).toBe(25);
	});
});

// The nine document families are offered for the interface too, and the faces behind them are
// declared in styles/uiFonts.css against the .ttf files the PDF renderer uses. Nothing at build
// time connects those three lists, so this is what notices when one of them moves.
describe("the document families offered for the interface", () => {
	const DOCUMENT_FAMILIES = [
		["poppins", "Poppins"],
		["roboto", "Roboto"],
		["sora", "Sora"],
		["josefin-sans", "Josefin Sans"],
		["pliant", "Pliant"],
		["geist", "Geist"],
		["archivo", "Archivo"],
		["fira-sans", "Fira Sans"],
		["dm-mono", "DM Mono"],
	];

	it("offers every one of them", () => {
		DOCUMENT_FAMILIES.forEach(([id]) => {
			expect(FONT_OPTIONS.find((f) => f.id === id), id).toBeTruthy();
		});
	});

	// The stack is what reaches CSS. A family name that does not match the @font-face falls
	// back silently - the font appears to be chosen and nothing changes on screen.
	it("names the family the @font-face declares, first in the stack", () => {
		DOCUMENT_FAMILIES.forEach(([id, family]) => {
			expect(fontStack(id).startsWith(`"${family}"`), id).toBe(true);
		});
	});

	it("gives every option a group, so none falls out of the picker", () => {
		FONT_OPTIONS.forEach((f) => expect(typeof f.group, f.id).toBe("string"));
		expect(FONT_GROUPS.flatMap((g) => g.fonts)).toHaveLength(FONT_OPTIONS.length);
	});

	it("keeps the groups in the order the options are declared", () => {
		expect(FONT_GROUPS.map((g) => g.label)).toEqual(["Interface", "Document families"]);
	});
});

// --- the Cards role ------------------------------------------------------------------------
// Cards sized off the BODY scale until they got a control of their own, so the migration has
// to inherit it: without that, everybody who had tuned Body would watch every card on every
// screen resize the first time they loaded a new build.
describe("the Cards scale", () => {
	it("inherits a stored body scale rather than resetting to 100%", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { body: 1.15 } }) });
		expect(readAppearance(s).scales.card).toBe(1.15);
	});

	it("keeps a card scale that was set deliberately", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { body: 1.15, card: 0.9 } }) });
		expect(readAppearance(s).scales.card).toBe(0.9);
	});

	it("is 100% for someone who has never set anything", () => {
		expect(readAppearance(fakeStorage()).scales.card).toBe(1);
	});

	it("writes its own CSS variable", () => {
		const root = document.createElement("div");
		applyAppearance({ font: "inter", scales: { card: 1.25 } }, root);
		expect(root.style.getPropertyValue("--font-scale-card")).toBe("1.25");
	});

	it("is offered as two controls - what a card is called, and what it says", () => {
		expect(SCALE_ROLES.map((r) => r.id)).toContain("cardTitle");
		expect(SCALE_ROLES.map((r) => r.id)).toContain("card");
	});

	// Two steps of migration now, and a card that was already tuned must not jump when the
	// title splits off it.
	it("gives the title whatever the card was set to", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { card: 1.3 } }) });
		expect(readAppearance(s).scales.cardTitle).toBe(1.3);
	});

	it("falls back to body when there was never a card scale either", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { body: 1.15 } }) });
		const scales = readAppearance(s).scales;
		expect(scales.card).toBe(1.15);
		expect(scales.cardTitle).toBe(1.15);
	});

	it("keeps a title scale that was set deliberately", () => {
		const s = fakeStorage({ appearance: JSON.stringify({ font: "inter", scales: { card: 1.3, cardTitle: 0.9 } }) });
		expect(readAppearance(s).scales.cardTitle).toBe(0.9);
	});

	it("writes the title variable too", () => {
		const root = document.createElement("div");
		applyAppearance({ font: "inter", scales: { cardTitle: 1.25 } }, root);
		expect(root.style.getPropertyValue("--font-scale-card-title")).toBe("1.25");
	});
});
