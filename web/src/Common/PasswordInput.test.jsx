import { describe, it, expect } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import PasswordInput from "./PasswordInput";

async function render(props = {}) {
	const container = document.createElement("div");
	document.body.appendChild(container);
	const root = createRoot(container);
	await act(async () => {
		root.render(<PasswordInput value="" onChange={() => {}} {...props} />);
	});
	return container;
}

async function click(el) {
	await act(async () => {
		el.dispatchEvent(new MouseEvent("click", { bubbles: true }));
	});
}

describe("PasswordInput", () => {
	it("starts masked", async () => {
		const c = await render();
		expect(c.querySelector("input").type).toBe("password");
	});

	it("reveals and re-masks on toggle", async () => {
		const c = await render();
		const btn = c.querySelector(".password-input-toggle");
		await click(btn);
		expect(c.querySelector("input").type).toBe("text");
		await click(btn);
		expect(c.querySelector("input").type).toBe("password");
	});

	it("uses type=button so it cannot submit a surrounding form", async () => {
		const c = await render();
		expect(c.querySelector(".password-input-toggle").type).toBe("button");
	});

	it("passes arbitrary input props through", async () => {
		const c = await render({ placeholder: "Password", autoComplete: "new-password" });
		const input = c.querySelector("input");
		expect(input.placeholder).toBe("Password");
		expect(input.getAttribute("autocomplete")).toBe("new-password");
	});
});
