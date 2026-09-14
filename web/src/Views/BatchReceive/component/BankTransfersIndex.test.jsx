import { describe, it, expect, vi, beforeEach } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";

// Each tab body pulls a whole domain behind it - stub them so this tests the tab strip.
vi.mock("./BatchReceiveForm", () => ({ default: () => <div data-testid="received" /> }));
vi.mock("../../SupplierPayment/component/SupplierPaymentForm", () => ({ default: () => <div data-testid="paid" /> }));
vi.mock("../../BankReport/component/ExpenseForm", () => ({ default: () => <div data-testid="expense" /> }));
vi.mock("../../../Common/Header/LiteHeader", () => ({ default: () => null }));

import BankTransfersIndex from "./BankTransfersIndex";

async function mount() {
	const container = document.createElement("div");
	document.body.appendChild(container);
	await act(async () => {
		createRoot(container).render(<BankTransfersIndex />);
	});
	return container;
}

const tabs = (c) => [...c.querySelectorAll('[role="tab"]')].map((b) => b.textContent.trim());

describe("BankTransfersIndex", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
	});

	it("offers Received, Paid and Expense", async () => {
		expect(tabs(await mount())).toEqual(["Received", "Paid", "Expense"]);
	});

	it("opens on Received", async () => {
		const c = await mount();
		expect(c.querySelector('[data-testid="received"]')).toBeTruthy();
	});

	it("switches to the expense form on the third tab", async () => {
		const c = await mount();
		const expenseTab = [...c.querySelectorAll('[role="tab"]')].find((b) => b.textContent.includes("Expense"));
		await act(async () => {
			expenseTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		});
		expect(c.querySelector('[data-testid="expense"]')).toBeTruthy();
		expect(c.querySelector('[data-testid="received"]')).toBeNull();
	});
});
