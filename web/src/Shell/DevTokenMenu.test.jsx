import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const listRegistrationTokens = vi.fn();
const createRegistrationToken = vi.fn();
const copyTextMock = vi.fn();

vi.mock("../Views/Dev/dev_backend", () => ({
    devBackend: {
        listRegistrationTokens: (...a) => listRegistrationTokens(...a),
        createRegistrationToken: (...a) => createRegistrationToken(...a),
    },
}));
vi.mock("../Common/clipboard", () => ({ copyText: (...a) => copyTextMock(...a) }));
vi.mock("../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import DevTokenMenu, { newestRedeemable, toApiDate } from "./DevTokenMenu";

const HOUR = 3600 * 1000;
const live = { token: "LIVE-TOKEN-1", expiresAt: new Date(Date.now() + 6 * HOUR).toISOString(), usedAt: null };
const used = { token: "USED-TOKEN", expiresAt: new Date(Date.now() + 6 * HOUR).toISOString(), usedAt: new Date().toISOString() };
const expired = { token: "OLD-TOKEN", expiresAt: new Date(Date.now() - HOUR).toISOString(), usedAt: null };

beforeEach(() => {
    vi.clearAllMocks();
    listRegistrationTokens.mockResolvedValue({ code: 200, data: [live] });
    createRegistrationToken.mockResolvedValue({ code: 200, data: { token: "FRESH", expiresAt: new Date(Date.now() + 24 * HOUR).toISOString() } });
    copyTextMock.mockResolvedValue(true);
});

describe("newestRedeemable", () => {
    it("ignores tokens that are used or expired", () => {
        expect(newestRedeemable([used, expired])).toBeNull();
        expect(newestRedeemable([used, expired, live]).token).toBe("LIVE-TOKEN-1");
    });

    it("copes with nothing at all", () => {
        expect(newestRedeemable([])).toBeNull();
        expect(newestRedeemable(null)).toBeNull();
    });
});

describe("DevTokenMenu", () => {
    it("stays shut until asked, and mints nothing on open", async () => {
        render(<DevTokenMenu />);
        expect(screen.queryByRole("dialog")).toBeNull();

        fireEvent.click(screen.getByRole("button", { name: /Token/ }));
        await waitFor(() => expect(screen.getByRole("dialog")).toBeDefined());
        // Looking at the token must not issue one - they are single use.
        expect(createRegistrationToken).not.toHaveBeenCalled();
    });

    it("shows the live token", async () => {
        render(<DevTokenMenu />);
        fireEvent.click(screen.getByRole("button", { name: /Token/ }));
        await waitFor(() => expect(screen.getByText("LIVE-TOKEN-1")).toBeDefined());
    });

    it("offers to generate one when none is live", async () => {
        listRegistrationTokens.mockResolvedValue({ code: 200, data: [used, expired] });
        render(<DevTokenMenu />);
        fireEvent.click(screen.getByRole("button", { name: /Token/ }));

        await waitFor(() => expect(screen.getByText(/No live token/)).toBeDefined());
        fireEvent.click(screen.getByRole("button", { name: /Generate token/ }));
        await waitFor(() => expect(screen.getByText("FRESH")).toBeDefined());
    });

    it("copies the token", async () => {
        render(<DevTokenMenu />);
        fireEvent.click(screen.getByRole("button", { name: /Token/ }));
        await waitFor(() => expect(screen.getByText("LIVE-TOKEN-1")).toBeDefined());

        fireEvent.click(screen.getByRole("button", { name: /Copy/ }));
        await waitFor(() => expect(copyTextMock).toHaveBeenCalledWith("LIVE-TOKEN-1"));
    });

    it("closes on Escape and on an outside click", async () => {
        render(<DevTokenMenu />);
        const trigger = screen.getByRole("button", { name: /Token/ });

        fireEvent.click(trigger);
        await waitFor(() => expect(screen.getByRole("dialog")).toBeDefined());
        fireEvent.keyDown(document, { key: "Escape" });
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

        fireEvent.click(trigger);
        await waitFor(() => expect(screen.getByRole("dialog")).toBeDefined());
        fireEvent.mouseDown(document.body);
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    });

    it("reports its state to assistive tech", async () => {
        render(<DevTokenMenu />);
        const trigger = screen.getByRole("button", { name: /Token/ });
        expect(trigger.getAttribute("aria-expanded")).toBe("false");
        fireEvent.click(trigger);
        await waitFor(() => expect(trigger.getAttribute("aria-expanded")).toBe("true"));
    });
});

describe("token expiry", () => {
    it("converts the picker's ISO date to the API's yyyymmdd", () => {
        expect(toApiDate("2026-09-10")).toBe("20260910");
        // Blank stays blank, so the API applies its own 24-hour default.
        expect(toApiDate("")).toBe("");
        expect(toApiDate(null)).toBe("");
    });

    it("mints with no expiry until one is picked", async () => {
        listRegistrationTokens.mockResolvedValue({ code: 200, data: [] });
        render(<DevTokenMenu />);
        fireEvent.click(screen.getByRole("button", { name: /Token/ }));
        await waitFor(() => expect(screen.getByText(/No live token/)).toBeDefined());

        fireEvent.click(screen.getByRole("button", { name: /Generate token/ }));
        await waitFor(() => expect(createRegistrationToken).toHaveBeenCalledWith(""));
    });

    it("sends the chosen date through to the API", async () => {
        listRegistrationTokens.mockResolvedValue({ code: 200, data: [] });
        render(<DevTokenMenu />);
        fireEvent.click(screen.getByRole("button", { name: /Token/ }));
        await waitFor(() => expect(screen.getByText(/No live token/)).toBeDefined());

        const input = document.querySelector('input[name="dev-token-expiry"]');
        expect(input).not.toBeNull();
        fireEvent.change(input, { target: { value: "2026-09-10" } });

        fireEvent.click(screen.getByRole("button", { name: /Generate token/ }));
        await waitFor(() => expect(createRegistrationToken).toHaveBeenCalledWith("20260910"));
    });
});
