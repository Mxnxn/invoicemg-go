import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const activeCompany = vi.fn();
const updateCompany = vi.fn();
vi.mock("../../Common/company_backend", () => ({
    companyBackend: { activeCompany: (...a) => activeCompany(...a), updateCompany: (...a) => updateCompany(...a) },
}));
vi.mock("../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import ExportsManager from "./ExportsManager";

beforeEach(() => {
    vi.clearAllMocks();
    activeCompany.mockResolvedValue({
        code: 200,
        data: {
            _id: "company-1",
            firm: "Plain Firm",
            url: "logo.png",
            exportTemplate: { logo: true, firm: true, address: false, phone: true, gst: true, showPeriod: true },
        },
    });
    updateCompany.mockResolvedValue({ code: 200 });
});

describe("ExportsManager", () => {
    // Toggle buttons, not checkboxes - state is aria-pressed, which is what a screen reader
    // announces and what the styling keys off.
    it("shows what this company currently prints", async () => {
        render(<ExportsManager />);
        await waitFor(() => expect(screen.getByRole("button", { name: /Logo/i })).toBeDefined());
        expect(screen.getByRole("button", { name: /Logo/i }).getAttribute("aria-pressed")).toBe("true");
        expect(screen.getByRole("button", { name: /Address/i }).getAttribute("aria-pressed")).toBe("false");
    });

    it("flips a toggle when pressed", async () => {
        render(<ExportsManager />);
        await waitFor(() => expect(screen.getByRole("button", { name: /Address/i })).toBeDefined());
        fireEvent.click(screen.getByRole("button", { name: /Address/i }));
        expect(screen.getByRole("button", { name: /Address/i }).getAttribute("aria-pressed")).toBe("true");
    });

    it("saves a change as exportTemplate fields", async () => {
        render(<ExportsManager />);
        await waitFor(() => expect(screen.getByRole("button", { name: /Address/i })).toBeDefined());

        fireEvent.click(screen.getByRole("button", { name: /Address/i }));
        fireEvent.click(screen.getByRole("button", { name: /Save/i }));

        await waitFor(() => expect(updateCompany).toHaveBeenCalled());
        const posted = updateCompany.mock.calls[0][0];
        // All six keys, every time - not just the one that changed. The API's exportTemplate
        // guard is all-or-nothing per request: a subset submission resets the unsent keys to
        // defaults, so a test that only checks the toggled key would not catch a regression to
        // "only send dirty fields" on any of the other five.
        expect(posted.get("exportTemplate[logo]")).toBe("true");
        expect(posted.get("exportTemplate[firm]")).toBe("true");
        expect(posted.get("exportTemplate[address]")).toBe("true");
        expect(posted.get("exportTemplate[phone]")).toBe("true");
        expect(posted.get("exportTemplate[gst]")).toBe("true");
        expect(posted.get("exportTemplate[showPeriod]")).toBe("true");
        // /company/update is all-or-nothing on company_id: omit it and the API returns 422
        // before it ever looks at the exportTemplate fields.
        expect(posted.get("company_id")).toBe("company-1");
    });

    it("says when the company has no logo to print", async () => {
        activeCompany.mockResolvedValue({ code: 200, data: { firm: "Plain Firm", url: "", exportTemplate: {} } });
        render(<ExportsManager />);
        await waitFor(() => expect(screen.getByText(/No logo uploaded/i)).toBeDefined());
    });

    // The save control only exists once something has actually changed, so reaching it means
    // toggling a chip first. What is being pinned is unchanged: with no company id loaded,
    // /company/update is never called - posting "null" as company_id would pass the API's
    // truthy check and fail as a cast error rather than a clean refusal.
    it("refuses to save when the active company failed to load", async () => {
        activeCompany.mockRejectedValue(new Error("network error"));
        render(<ExportsManager />);
        await waitFor(() => expect(screen.getByRole("button", { name: /Firm name/i })).toBeDefined());

        fireEvent.click(screen.getByRole("button", { name: /Firm name/i }));

        const save = await screen.findByRole("button", { name: /Save changes/i });
        expect(save.disabled).toBe(true);
        fireEvent.click(save);
        expect(updateCompany).not.toHaveBeenCalled();
    });

    it("shows no save control until something is changed, then offers to discard", async () => {
        render(<ExportsManager />);
        await waitFor(() => expect(screen.getByRole("button", { name: /Firm name/i })).toBeDefined());
        expect(screen.queryByRole("button", { name: /Save changes/i })).toBeNull();

        fireEvent.click(screen.getByRole("button", { name: /Firm name/i }));
        expect(await screen.findByRole("button", { name: /Save changes/i })).toBeDefined();

        // Discarding puts the toggle back and takes the bar away with it.
        fireEvent.click(screen.getByRole("button", { name: /Discard/i }));
        await waitFor(() => expect(screen.queryByRole("button", { name: /Save changes/i })).toBeNull());
        expect(updateCompany).not.toHaveBeenCalled();
    });
});
