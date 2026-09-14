import { describe, it, expect, vi, beforeEach } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";

const listAdminsMock = vi.fn();
const listPasswordRequestsMock = vi.fn(() => Promise.resolve({ code: 200, data: [] }));

vi.mock("../dev_backend", () => ({
	devBackend: {
		listAdmins: (...a) => listAdminsMock(...a),
		footprint: vi.fn(),
		setStatus: vi.fn(),
		wipe: vi.fn(),
		restore: vi.fn(),
		exportData: vi.fn(),
		createRegistrationToken: vi.fn(),
		// Loaded on mount, so it has to resolve or the component throws before rendering.
		listEnquiries: vi.fn(() => Promise.resolve({ code: 200, data: [] })),
		setEnquiryHandled: vi.fn(() => Promise.resolve({ code: 200 })),
		listPasswordRequests: (...a) => listPasswordRequestsMock(...a),
	},
}));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));
// LiteHeader is a .js file containing JSX. The app builds it via the esbuild jsx loader in
// vite.config, but vitest's transform does not apply that to .js - so stub it here rather
// than reconfigure the shared build for one test.
vi.mock("../../../Common/Header/LiteHeader", () => ({ default: () => null }));

import DevIndex from "./DevIndex";

const YEAR_AWAY = new Date(Date.now() + 365 * 864e5).toISOString();
const YEAR_AGO = new Date(Date.now() - 365 * 864e5).toISOString();

async function mountWith(rows) {
	listAdminsMock.mockResolvedValue({ code: 200, data: rows });
	const container = document.createElement("div");
	document.body.appendChild(container);
	await act(async () => {
		createRoot(container).render(<DevIndex />);
	});
	return container;
}

const badgeText = (c) => c.querySelector("tbody .xan-status-badge").textContent.trim();

const row = (over = {}) => ({
	_id: "1",
	name: "A",
	email: "a@b.co",
	role: "admin",
	is_active: true,
	activeUntil: null,
	companies: 1,
	lastLoginAt: null,
	...over,
});

describe("DevIndex status derivation", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
		listAdminsMock.mockReset();
	});

	it("shows Active for an enabled account with no expiry", async () => {
		const c = await mountWith([row()]);
		expect(badgeText(c)).toBe("Active");
	});

	it("shows Expired once activeUntil is in the past", async () => {
		const c = await mountWith([row({ activeUntil: YEAR_AGO })]);
		expect(badgeText(c)).toBe("Expired");
	});

	it("stays Active while activeUntil is in the future", async () => {
		const c = await mountWith([row({ activeUntil: YEAR_AWAY })]);
		expect(badgeText(c)).toBe("Active");
	});

	it("shows Inactive when disabled, even with a future expiry", async () => {
		const c = await mountWith([row({ is_active: false, activeUntil: YEAR_AWAY })]);
		expect(badgeText(c)).toBe("Inactive");
	});

	it("protects superadmin rows from destructive actions", async () => {
		const c = await mountWith([row({ role: "superadmin" })]);
		expect(c.textContent).toContain("protected");
		expect(c.querySelector(".dev-icon-btn--danger")).toBeNull();
	});

	it("offers the destructive action for ordinary customers", async () => {
		const c = await mountWith([row()]);
		expect(c.querySelector(".dev-icon-btn--danger")).toBeTruthy();
	});
});

describe("DevIndex expiry column", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
		listAdminsMock.mockReset();
	});

	// The date input is the setter; the Expiry cell is the readout of what is currently set,
	// so a list of customers can be scanned without opening each picker.
	it("shows the set expiry date", async () => {
		const c = await mountWith([row({ activeUntil: "2027-03-04T00:00:00.000Z" })]);
		const cell = c.querySelector("tbody .dev-expiry");
		expect(cell.textContent).toContain(new Date("2027-03-04T00:00:00.000Z").toLocaleDateString());
	});

	it("says no expiry when none is set", async () => {
		const c = await mountWith([row({ activeUntil: null })]);
		expect(c.querySelector("tbody .dev-expiry").textContent.trim()).toBe("no expiry");
	});

	// Selected by role rather than by the old .dev-input--date class: the field is the shared
	// DateField now (Common/DateField.jsx), which brings its own styling. What matters here is
	// unchanged - the row's expiry is what the input shows.
	it("keeps the date input as the setter alongside it", async () => {
		const c = await mountWith([row({ activeUntil: "2027-03-04T00:00:00.000Z" })]);
		expect(c.querySelector('tbody input[type="date"]').value).toBe("2027-03-04");
	});
});

describe("DevIndex active toggle", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
		listAdminsMock.mockReset();
	});

	// The toggle used to be neutral grey in both states - you could not tell activate from
	// deactivate without reading the tooltip.
	it("colours the toggle as on for an active customer", async () => {
		const c = await mountWith([row({ is_active: true })]);
		expect(c.querySelector("tbody .dev-icon-btn--on")).toBeTruthy();
	});

	it("colours the toggle as off for an inactive customer", async () => {
		const c = await mountWith([row({ is_active: false })]);
		expect(c.querySelector("tbody .dev-icon-btn--off")).toBeTruthy();
	});
});

describe("password reset requests", () => {
	// They were only ever fetched after resolving one, so the queue could never fill:
	// the list started empty and nothing could be resolved from an empty list.
	it("fetches the pending queue on mount", async () => {
		listPasswordRequestsMock.mockClear();
		await mountWith([]);
		expect(listPasswordRequestsMock).toHaveBeenCalled();
	});
});

describe("dev panel tabs", () => {
	it("offers a tab per job, resets among them", async () => {
		// Scoped to this mount: earlier tests in this file leave their own trees in the body.
		const container = await mountWith([]);
		const labels = [...container.querySelectorAll('[role="tab"]')].map((t) => t.textContent.trim());
		expect(labels).toEqual(["Users", "Enquiries", "Resets", "Inbox"]);
	});

	it("opens on Users, so the customer table is what you land on", async () => {
		const container = await mountWith([]);
		const selected = container.querySelector('[role="tab"][aria-selected="true"]');
		expect(selected.textContent.trim()).toBe("Users");
	});
});
