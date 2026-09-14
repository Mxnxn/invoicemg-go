import { describe, it, expect, vi, beforeEach } from "vitest";
// No @testing-library in this project - component tests drive React directly with
// createRoot + act, matching CashflowCard.test.jsx.
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";

const registerMock = vi.fn(() => Promise.resolve({ code: 200 }));
const loginMock = vi.fn(() =>
	Promise.resolve({ code: 200, data: { uid: "u1", token: "t1", email: "a@b.co", role: "admin" } })
);
const createCompanyMock = vi.fn(() => Promise.resolve({ code: 200 }));

vi.mock("../auth_backend", () => ({
	authBackend: {
		registerWithEmailAndPassword: (...args) => registerMock(...args),
		loginWithEmailAndPassword: (...args) => loginMock(...args),
		loginAsEmployee: vi.fn(),
	},
}));
vi.mock("../../../Common/company_backend", () => ({
	companyBackend: { createCompany: (...args) => createCompanyMock(...args) },
}));

import AuthForm from "./AuthForm";

// React tracks an input's value on the DOM node, so assigning .value directly is ignored.
// Go through the native setter and dispatch a bubbling input event instead.
function setValue(input, value) {
	const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value").set;
	setter.call(input, value);
	input.dispatchEvent(new Event("input", { bubbles: true }));
}

function byPlaceholder(container, placeholder) {
	return container.querySelector(`input[placeholder="${placeholder}"]`);
}

function buttonWith(container, text) {
	return Array.from(container.querySelectorAll("button")).find((el) =>
		el.textContent.trim().toLowerCase().includes(text)
	);
}

async function click(el) {
	await act(async () => {
		el.dispatchEvent(new MouseEvent("click", { bubbles: true }));
	});
}

async function mount() {
	const container = document.createElement("div");
	document.body.appendChild(container);
	const root = createRoot(container);
	await act(async () => {
		root.render(<AuthForm />);
	});
	return container;
}

async function gotoRegister(container) {
	await click(buttonWith(container, "register"));
}

async function fillStepOne(container, { token = "tok" } = {}) {
	await act(async () => {
		if (token) setValue(byPlaceholder(container, "Registration token"), token);
		setValue(byPlaceholder(container, "Name"), "A");
		setValue(byPlaceholder(container, "Email"), "a@b.co");
		setValue(byPlaceholder(container, "Password (min. 8 characters)"), "secret123");
	});
}

describe("AuthForm two-step registration", () => {
	beforeEach(() => {
		window.localStorage.clear();
		registerMock.mockClear();
		loginMock.mockClear();
		createCompanyMock.mockClear();
		document.body.innerHTML = "";
	});

	it("shows a registration token field on the register tab", async () => {
		const container = await mount();
		await gotoRegister(container);
		expect(byPlaceholder(container, "Registration token")).toBeTruthy();
		expect(byPlaceholder(container, "Company name")).toBeNull();
	});

	it("sends the registration token with the account request", async () => {
		const container = await mount();
		await gotoRegister(container);
		await fillStepOne(container);
		await click(buttonWith(container, "continue"));

		expect(registerMock).toHaveBeenCalled();
		expect(registerMock.mock.calls[0][0].get("registrationToken")).toBe("tok");
	});

	it("advances to step 2 and stores the session", async () => {
		const container = await mount();
		await gotoRegister(container);
		await fillStepOne(container);
		await click(buttonWith(container, "continue"));

		expect(byPlaceholder(container, "Company name")).toBeTruthy();
		expect(window.localStorage.getItem("session_token")).toBe("t1");
		// uid matters: ProtectiveRoute reads it, so without it step 2 would bounce to login.
		expect(window.localStorage.getItem("uid")).toBe("u1");
	});

	it("offers both uploads on step 2, marked optional", async () => {
		const container = await mount();
		await gotoRegister(container);
		await fillStepOne(container);
		await click(buttonWith(container, "continue"));

		expect(container.querySelector("#auth-logo")).toBeTruthy();
		expect(container.querySelector("#auth-upiqr")).toBeTruthy();
		expect(container.textContent.toLowerCase()).toContain("optional");
	});

	it("submits with no files attached, after the details step", async () => {
		const container = await mount();
		await gotoRegister(container);
		await fillStepOne(container);
		await click(buttonWith(container, "continue"));

		// Step 2 now advances to the details step rather than submitting.
		await act(async () => setValue(byPlaceholder(container, "Company name"), "Acme"));
		await click(buttonWith(container, "continue"));
		expect(byPlaceholder(container, "Company address")).toBeTruthy();
		// Phone lives on this step and is required to finish.
		await act(async () => setValue(byPlaceholder(container, "Phone"), "9825505771"));
		await click(buttonWith(container, "finish"));

		expect(createCompanyMock).toHaveBeenCalled();
		const form = createCompanyMock.mock.calls[0][0];
		expect(form.get("name")).toBe("Acme");
		// Optional uploads must be absent, not empty - the API guards on req.files.
		expect(form.get("logo")).toBeNull();
		expect(form.get("upiQr")).toBeNull();
	});

	it("returns to step 1 when the user switches back to login", async () => {
		const container = await mount();
		await gotoRegister(container);
		await fillStepOne(container);
		await click(buttonWith(container, "continue"));
		expect(byPlaceholder(container, "Company name")).toBeTruthy();

		await click(buttonWith(container, "login"));
		await gotoRegister(container);
		expect(byPlaceholder(container, "Registration token")).toBeTruthy();
		expect(byPlaceholder(container, "Company name")).toBeNull();
	});

	it("will not register a company without a phone number", async () => {
		const container = await mount();
		await gotoRegister(container);
		await fillStepOne(container);
		await click(buttonWith(container, "continue"));
		await act(async () => setValue(byPlaceholder(container, "Company name"), "Acme"));
		await click(buttonWith(container, "continue"));

		// Phone left blank - finishing must be refused rather than creating a company that
		// cannot be reached on WhatsApp or printed usefully on an invoice.
		await click(buttonWith(container, "finish"));
		expect(createCompanyMock).not.toHaveBeenCalled();
		expect(container.textContent).toMatch(/Phone number is required/i);
	});
});
