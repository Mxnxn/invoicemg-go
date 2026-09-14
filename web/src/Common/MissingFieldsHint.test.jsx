import { describe, it, expect } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import MissingFieldsHint from "./MissingFieldsHint";

function render(props) {
	const container = document.createElement("div");
	document.body.appendChild(container);
	act(() => {
		createRoot(container).render(<MissingFieldsHint {...props} />);
	});
	return container;
}

describe("MissingFieldsHint", () => {
	it("renders nothing when the form is complete", () => {
		// A complete form must not leave an empty amber strip sitting in the footer.
		expect(render({ missing: [] }).textContent).toBe("");
		expect(render({}).textContent).toBe("");
	});

	it("names the single outstanding field", () => {
		expect(render({ missing: ["Client"] }).textContent).toContain("Client");
	});

	it("lists every outstanding field, so nothing is discovered one at a time", () => {
		const text = render({ missing: ["Client", "Job Number", "at least one row"] }).textContent;
		expect(text).toContain("Client");
		expect(text).toContain("Job Number");
		expect(text).toContain("at least one row");
	});
});
