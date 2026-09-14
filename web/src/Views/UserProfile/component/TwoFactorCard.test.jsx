import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const totpStatus = vi.fn();
const totpReveal = vi.fn();
vi.mock("../user_backend", () => ({
    userBackend: {
        totpStatus: (...a) => totpStatus(...a),
        totpReveal: (...a) => totpReveal(...a),
        totpSetup: vi.fn(),
        totpEnable: vi.fn(),
        totpDisable: vi.fn(),
    },
}));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));
// The QR is drawn to a canvas, which jsdom does not implement. The key is shown as text
// beside it and is what these tests are about.
vi.mock("qrcode", () => ({ default: { toDataURL: () => Promise.resolve("") } }));

import TwoFactorCard from "./TwoFactorCard";

const SECRET = "JBSWY3DPEHPK3PXP";

beforeEach(() => {
    vi.clearAllMocks();
    totpStatus.mockResolvedValue({ code: 200, data: { enabled: true } });
    totpReveal.mockResolvedValue({ code: 200, data: { secret: SECRET, otpauth: "otpauth://totp/x" } });
});

const showPrompt = async () => {
    render(<TwoFactorCard />);
    await waitFor(() => expect(screen.getByRole("button", { name: /Show key again/i })).toBeDefined());
    fireEvent.click(screen.getByRole("button", { name: /Show key again/i }));
    return screen.getByLabelText(/Your account password/i);
};

describe("TwoFactorCard re-reveal", () => {
    // The offer only makes sense once there is a key to show again. Before that the setup
    // button is the way in, and two entry points would just be two ways to be confused.
    it("offers the re-reveal only when two-factor is already on", async () => {
        totpStatus.mockResolvedValue({ code: 200, data: { enabled: false } });
        render(<TwoFactorCard />);
        await waitFor(() => expect(screen.getByRole("button", { name: /Set up two-factor/i })).toBeDefined());
        expect(screen.queryByRole("button", { name: /Show key again/i })).toBeNull();
    });

    it("keeps the key hidden until the password is accepted", async () => {
        await showPrompt();
        expect(screen.queryByText(SECRET)).toBeNull();
        expect(totpReveal).not.toHaveBeenCalled();
    });

    it("shows the key once the password is accepted", async () => {
        const field = await showPrompt();
        fireEvent.change(field, { target: { value: "hunter22" } });
        fireEvent.click(screen.getByRole("button", { name: /^Show key$/i }));
        await waitFor(() => expect(screen.getByText(SECRET)).toBeDefined());
        expect(totpReveal).toHaveBeenCalledWith("hunter22");
    });

    // The password is what stands between anyone at the keyboard and the account's second
    // factor. A wrong one must leave the key exactly as hidden as before.
    it("says so and shows nothing when the password is wrong", async () => {
        totpReveal.mockRejectedValue({ message: "That is not your account password." });
        const field = await showPrompt();
        fireEvent.change(field, { target: { value: "wrong" } });
        fireEvent.click(screen.getByRole("button", { name: /^Show key$/i }));
        await waitFor(() => expect(screen.getByText(/not your account password/i)).toBeDefined());
        expect(screen.queryByText(SECRET)).toBeNull();
    });

    // Nothing may fill this in or carry it away: the point of the prompt is that the person
    // present knows the password, not that a manager on the machine remembers it.
    it("refuses paste and keeps password managers out of the field", async () => {
        const field = await showPrompt();
        expect(field.getAttribute("autocomplete")).toBe("off");
        expect(field.getAttribute("data-1p-ignore")).toBe("true");
        expect(field.getAttribute("data-lpignore")).toBe("true");
        // A name no saved credential can match, rather than "password".
        expect(field.getAttribute("name")).toMatch(/^totp-reveal-/);

        const paste = new Event("paste", { bubbles: true, cancelable: true });
        field.dispatchEvent(paste);
        expect(paste.defaultPrevented).toBe(true);

        const copy = new Event("copy", { bubbles: true, cancelable: true });
        field.dispatchEvent(copy);
        expect(copy.defaultPrevented).toBe(true);
    });

    // Closing must forget the secret, not just stop drawing it - otherwise one password entry
    // buys a reveal that lasts the rest of the visit.
    it("asks for the password again after the panel is closed", async () => {
        const field = await showPrompt();
        fireEvent.change(field, { target: { value: "hunter22" } });
        fireEvent.click(screen.getByRole("button", { name: /^Show key$/i }));
        await waitFor(() => expect(screen.getByText(SECRET)).toBeDefined());

        fireEvent.click(screen.getByRole("button", { name: /^Done$/i }));
        expect(screen.queryByText(SECRET)).toBeNull();
        expect(screen.getByRole("button", { name: /Show key again/i })).toBeDefined();
    });

    // Copy is for a key that has just been minted. This one already protects the account, and
    // asking for a password to put it on the clipboard would defeat the asking.
    it("does not offer to copy a re-revealed key", async () => {
        const field = await showPrompt();
        fireEvent.change(field, { target: { value: "hunter22" } });
        fireEvent.click(screen.getByRole("button", { name: /^Show key$/i }));
        await waitFor(() => expect(screen.getByText(SECRET)).toBeDefined());
        expect(screen.queryByRole("button", { name: /Copy key/i })).toBeNull();
    });
});
