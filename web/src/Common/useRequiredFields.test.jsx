import { describe, it, expect } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import useRequiredFields from "./useRequiredFields";

// No @testing-library in this project, so the hook is exercised through a probe component.
function probe(values, rules, onReady) {
	const Probe = () => {
		const req = useRequiredFields(values, rules);
		onReady(req);
		return null;
	};
	const container = document.createElement("div");
	document.body.appendChild(container);
	act(() => {
		createRoot(container).render(<Probe />);
	});
}

const RULES = { name: "Name", phone: { label: "Phone", validate: (v) => (v.length === 10 ? null : "Phone must be 10 digits.") } };

describe("useRequiredFields", () => {
	it("reports every missing field", () => {
		let req;
		probe({ name: "", phone: "" }, RULES, (r) => (req = r));
		expect(req.errors.name).toBe("Name is required.");
		expect(req.errors.phone).toBe("Phone is required.");
		expect(req.isComplete).toBe(false);
	});

	it("treats whitespace as missing", () => {
		let req;
		probe({ name: "   ", phone: "1234567890" }, RULES, (r) => (req = r));
		expect(req.errors.name).toBe("Name is required.");
	});

	it("runs custom validators once a value is present", () => {
		let req;
		probe({ name: "A", phone: "123" }, RULES, (r) => (req = r));
		expect(req.errors.phone).toBe("Phone must be 10 digits.");
		expect(req.isComplete).toBe(false);
	});

	it("is complete when every rule passes", () => {
		let req;
		probe({ name: "A", phone: "1234567890" }, RULES, (r) => (req = r));
		expect(req.errors).toEqual({});
		expect(req.isComplete).toBe(true);
	});

	it("hides errors until a field is touched", () => {
		let req;
		probe({ name: "", phone: "" }, RULES, (r) => (req = r));
		// The error exists, but nothing is shown before the user has interacted.
		expect(req.errors.name).toBeTruthy();
		expect(req.errorFor("name")).toBeUndefined();
	});
});
