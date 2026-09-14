import { describe, expect, it } from "vitest";
import { landingPathFor } from "./landingPath";

// Where every login lands. This had no test, which is how the "/" value below rotted: it was
// correct when "/" redirected to /admin, and became wrong the day src/Landing took over "/"
// - dropping an admin back on the public marketing site after signing in.
describe("landingPathFor", () => {
    it("sends an admin into the app, never to the marketing site", () => {
        expect(landingPathFor("admin", [])).toBe("/admin");
        expect(landingPathFor("admin", [])).not.toBe("/");
    });

    it("sends a superadmin into the app too", () => {
        expect(landingPathFor("superadmin", [])).toBe("/admin");
    });

    // An employee lands on the first thing they can actually open - being bounced by the
    // route guard on the first paint is what this avoids.
    it("sends an employee to a feature they have", () => {
        expect(landingPathFor("employee", ["invoices"])).toBe("/admin/invoices");
    });

    it("falls back for an employee with no matching permission", () => {
        expect(landingPathFor("employee", [])).toBe("/admin/lifecycle");
    });

    // Whatever the role, the destination is inside the app.
    it("never returns the public landing page", () => {
        for (const role of ["admin", "superadmin", "employee", "", undefined]) {
            expect(landingPathFor(role, [])).toMatch(/^\/admin/);
        }
    });
});
