import { describe, it, expect, beforeEach, vi } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { MemoryRouter } from "react-router-dom";

// If the Dev shell ever pulled these in, the assertions below would silently pass on a
// stub - so they are deliberately NOT mocked. Importing DevSideMenu must not reach them.
import DevSideMenu from "./DevSideMenu";
import DevNavbar from "./DevNavbar";

async function mount(el) {
	const container = document.createElement("div");
	document.body.appendChild(container);
	await act(async () => {
		createRoot(container).render(<MemoryRouter>{el}</MemoryRouter>);
	});
	return container;
}

describe("DevSideMenu", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
	});

	it("offers Dev and no tenant navigation", async () => {
		const c = await mount(<DevSideMenu collapsed={false} onToggleCollapsed={() => {}} />);
		const labels = Array.from(c.querySelectorAll(".shell-nav a")).map((a) => a.textContent.trim());
		expect(labels).toEqual(["Dev"]);
	});

	// An operator never acts as a tenant, so the company switcher - which is also the
	// account details and "add account" surface - has no place in this shell.
	it("has no company switcher, account details or add-account control", async () => {
		const c = await mount(<DevSideMenu collapsed={false} onToggleCollapsed={() => {}} />);
		expect(c.querySelector(".shell-user")).toBeNull();
		expect(c.querySelector(".company-switcher")).toBeNull();
		expect(c.textContent).not.toMatch(/add account/i);
	});

	it("labels itself Developer rather than Admin", async () => {
		const c = await mount(<DevSideMenu collapsed={false} onToggleCollapsed={() => {}} />);
		expect(c.querySelector(".shell-brand-text").textContent).toContain("Developer");
	});
});

describe("DevNavbar", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
	});

	// Trash is a customer's own soft-delete bin - it is company-scoped and meaningless for
	// an operator account.
	it("has no trash link", async () => {
		const c = await mount(<DevNavbar heading="Dev" theme="light" onToggleTheme={() => {}} />);
		expect(c.querySelector('a[aria-label="Trash"]')).toBeNull();
	});

	it("keeps the theme toggle and logout", async () => {
		const c = await mount(<DevNavbar heading="Dev" theme="light" onToggleTheme={() => {}} />);
		expect(c.querySelector('[aria-label="Logout"]')).toBeTruthy();
		expect(c.querySelector('[aria-label="Switch to dark theme"]')).toBeTruthy();
	});
});
