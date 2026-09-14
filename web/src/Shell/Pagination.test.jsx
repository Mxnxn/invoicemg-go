import { describe, it, expect, vi } from "vitest";
import React from "react";
import { createRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import Pagination from "./Pagination";

async function render(props) {
	const container = document.createElement("div");
	document.body.appendChild(container);
	const root = createRoot(container);
	await act(async () => {
		root.render(<Pagination setCurrentPage={vi.fn()} currentPage={1} {...props} />);
	});
	return container;
}

describe("Pagination", () => {
	it("renders nothing when everything fits on one page", async () => {
		const c = await render({ totalItems: 8, perPage: 10 });
		expect(c.querySelector(".shell-pagination")).toBeNull();
	});

	it("renders nothing when the item count exactly fills one page", async () => {
		const c = await render({ totalItems: 10, perPage: 10 });
		expect(c.querySelector(".shell-pagination")).toBeNull();
	});

	it("renders nothing when there are no items at all", async () => {
		const c = await render({ totalItems: 0, perPage: 10 });
		expect(c.querySelector(".shell-pagination")).toBeNull();
	});

	it("renders once a second page exists", async () => {
		const c = await render({ totalItems: 11, perPage: 10 });
		expect(c.querySelector(".shell-pagination")).toBeTruthy();
	});
});
