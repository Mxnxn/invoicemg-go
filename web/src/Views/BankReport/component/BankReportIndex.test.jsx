import { describe, it, expect, vi, beforeEach } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";

// Rendering is the point of these tests: an icon name react-feather does not export resolves
// to undefined and only fails at render ("Element type is invalid"). Unit tests of the math
// passed happily while the page itself was dead, so these mount the real components.
vi.mock("../bank_report_backend", () => ({
	bankReportBackend: {
		report: vi.fn().mockResolvedValue({ data: { rows: [], totals: null, banks: [] } }),
		createExpense: vi.fn().mockResolvedValue({ code: 200 }),
		// ExpenseForm renders ExpenseList beneath it, which loads on mount.
		listExpenses: vi.fn().mockResolvedValue({ data: [] }),
		removeExpense: vi.fn().mockResolvedValue({ code: 200 }),
	},
}));
vi.mock("../../../Common/bank_backend", () => ({ bankBackend: { list: vi.fn().mockResolvedValue({ data: [] }) } }));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import BankReportIndex from "./BankReportIndex";
import ExpenseForm from "./ExpenseForm";

async function mount(el) {
	const container = document.createElement("div");
	document.body.appendChild(container);
	await act(async () => {
		createRoot(container).render(el);
	});
	return container;
}

describe("BankReportIndex", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
	});

	it("renders without crashing", async () => {
		const c = await mount(<BankReportIndex />);
		expect(c.textContent).toContain("No bank activity yet");
	});

	// The expense form moved to the Bank Transfers "Expense" tab - this screen is a report.
	it("does not embed the expense form", async () => {
		const c = await mount(<BankReportIndex />);
		expect(c.querySelector('input[name="amount"]')).toBeNull();
	});

	it("offers XLSX and PDF downloads", async () => {
		const c = await mount(<BankReportIndex />);
		const labels = [...c.querySelectorAll("button")].map((b) => b.textContent.trim());
		expect(labels).toContain("XLSX");
		expect(labels).toContain("PDF");
	});

	// Four figures on one row - the compact summary strip, not four full-height cards.
	it("puts the summary cards in a single row container", async () => {
		const c = await mount(<BankReportIndex />);
		expect(c.querySelector(".bank-report-toolbar")).toBeTruthy();
	});
});

describe("ExpenseForm", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
	});

	// The codebase's convention (see QuickCreateProductModal / AddClient) is that the star is
	// rule-driven: it marks a field that is CURRENTLY missing, and disappears once filled.
	// Date is pre-filled with today, so it starts without one.
	it("stars the required fields that are still empty", async () => {
		const c = await mount(<ExpenseForm />);
		const labels = [...c.querySelectorAll("label")].filter((l) => l.querySelector(".required-star"));
		expect(labels.map((l) => l.textContent.replace("*", "").trim())).toEqual(["Bank", "Amount"]);
	});

	it("does not star the date, which is pre-filled with today", async () => {
		const c = await mount(<ExpenseForm />);
		const dateLabel = [...c.querySelectorAll("label")].find((l) => l.textContent.startsWith("Date"));
		expect(dateLabel.querySelector(".required-star")).toBeNull();
	});

	// Submit is deliberately NOT disabled on an incomplete form any more, here or on the
	// Received/Paid forms beside it. A dead button cannot say what is missing, and the
	// handler's own required.showAll() branch was unreachable while it was disabled -
	// clicking now reveals which field is at fault instead of doing nothing.
	it("leaves submit clickable on an empty form, so it can report what is missing", async () => {
		const c = await mount(<ExpenseForm />);
		const btn = [...c.querySelectorAll("button")].find((b) => /Save Expense/.test(b.textContent));
		expect(btn).toBeTruthy();
		expect(btn.disabled).toBe(false);
	});
});
