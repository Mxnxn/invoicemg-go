import { describe, it, expect } from "vitest";
import { isMutation } from "./errorInterceptor";

describe("isMutation", () => {
	it("announces user-initiated writes", () => {
		["/client/add", "/company/create", "/person/update", "/lifecycle/jobs/delete", "/userinfo/upload", "/dev/admin/wipe"].forEach(
			(u) => expect(isMutation(u)).toBe(true)
		);
	});

	it("stays silent on reads, even when the path contains a write-ish word", () => {
		// These fire on every page load - toasting them would train the user to ignore
		// toasts, which costs us the failures too.
		["/client/get", "/invoice/list", "/challan/getAll", "/lifecycle/lookups/materials", "/company/active", "/dev/admins"].forEach(
			(u) => expect(isMutation(u)).toBe(false)
		);
	});

	it("handles missing urls", () => {
		expect(isMutation("")).toBe(false);
		expect(isMutation(undefined)).toBe(false);
	});
});

describe("expense routes", () => {
	// /expense/create announces itself through the global interceptor. A component that also
	// calls notifySuccess therefore shows the same toast twice on one tap - which is exactly
	// what ExpenseForm did.
	it("announces creating and removing an expense", () => {
		expect(isMutation("/expense/create")).toBe(true);
		expect(isMutation("/expense/remove")).toBe(true);
	});

	it("stays silent listing expenses", () => {
		expect(isMutation("/expense/list")).toBe(false);
	});
});
