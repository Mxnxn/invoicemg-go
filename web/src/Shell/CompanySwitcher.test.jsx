import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const listCompanies = vi.fn();
vi.mock("../Common/company_backend", () => ({
    companyBackend: {
        listCompanies: (...a) => listCompanies(...a),
        switchCompany: vi.fn(),
        createCompany: vi.fn(),
    },
}));
vi.mock("../Common/access", () => ({ can: () => true }));
vi.mock("react-router-dom", () => ({ useNavigate: () => vi.fn() }));
vi.mock("./CompanyFormModal", () => ({ default: () => null }));

import CompanySwitcher from "./CompanySwitcher";

const payload = (over) => ({
    data: {
        companies: [{ _id: "co1", name: "Acme" }],
        active_company_id: "co1",
        company_limit: 1,
        can_add_company: false,
        ...over,
    },
});

beforeEach(() => {
    vi.clearAllMocks();
    listCompanies.mockResolvedValue(payload());
});

const openMenu = async () => {
    // The trigger carries the active company's name.
    await waitFor(() => expect(screen.getByText("Acme")).toBeDefined());
    fireEvent.click(screen.getByText("Acme"));
};

describe("CompanySwitcher — company cap", () => {
    // The server refuses the create anyway (routes/Company.js), but offering a button that
    // will be rejected wastes a filled-in form to find out.
    it("disables Add company when the admin is at their cap", async () => {
        render(<CompanySwitcher />);
        await openMenu();
        const add = await screen.findByText("Add company");
        expect(add.closest("button").disabled).toBe(true);
    });

    // Disabled alone says "no" without saying why. The reason belongs on the control.
    it("explains the cap rather than just refusing", async () => {
        render(<CompanySwitcher />);
        await openMenu();
        const add = await screen.findByText("Add company");
        expect(add.closest("button").title).toMatch(/limited to 1 company/i);
    });

    // Plural wording, because "limited to 3 company" reads as a bug.
    it("pluralises the limit correctly", async () => {
        listCompanies.mockResolvedValue(payload({ company_limit: 3, can_add_company: false }));
        render(<CompanySwitcher />);
        await openMenu();
        const add = await screen.findByText("Add company");
        expect(add.closest("button").title).toMatch(/limited to 3 companies/i);
    });

    // Under the cap: fully usable, and the tooltip does not nag.
    it("leaves Add company clickable when there is room", async () => {
        listCompanies.mockResolvedValue(payload({ company_limit: 5, can_add_company: true }));
        render(<CompanySwitcher />);
        await openMenu();
        const add = await screen.findByText("Add company");
        expect(add.closest("button").disabled).toBe(false);
        expect(add.closest("button").title).toBe("Add a company");
    });

    // A response that omits the field must not silently grant an extra company - the flag is
    // absent on an older API, and failing open would offer a button the server then refuses.
    it("fails closed when the flag is missing", async () => {
        listCompanies.mockResolvedValue({ data: { companies: [{ _id: "co1", name: "Acme" }], active_company_id: "co1" } });
        render(<CompanySwitcher />);
        await openMenu();
        const add = await screen.findByText("Add company");
        expect(add.closest("button").disabled).toBe(true);
    });
});
