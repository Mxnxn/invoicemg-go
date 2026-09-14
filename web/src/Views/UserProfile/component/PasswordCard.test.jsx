import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const requestPasswordReset = vi.fn();
const changePassword = vi.fn();
vi.mock("../user_backend", () => ({
    userBackend: {
        requestPasswordReset: (...a) => requestPasswordReset(...a),
        changePassword: (...a) => changePassword(...a),
    },
}));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import PasswordCard from "./PasswordCard";

beforeEach(() => {
    vi.clearAllMocks();
    requestPasswordReset.mockResolvedValue({ code: 200 });
});

const ask = () => {
    render(<PasswordCard email="ravi@boltvinyls.in" />);
    fireEvent.click(screen.getByRole("button", { name: /forgotten it/i }));
};

describe("PasswordCard forgot-password", () => {
    // The request lands in a queue a person works through by hand. One stray click filing a
    // ticket would fill that queue with noise.
    it("confirms before filing anything", () => {
        ask();
        expect(requestPasswordReset).not.toHaveBeenCalled();
        expect(screen.getByText(/ravi@boltvinyls.in/)).toBeDefined();
    });

    it("files the request for the signed-in account", async () => {
        ask();
        fireEvent.click(screen.getByRole("button", { name: /Ask for a reset/i }));
        await waitFor(() => expect(requestPasswordReset).toHaveBeenCalledWith("ravi@boltvinyls.in"));
    });

    it("backs out without filing anything", () => {
        ask();
        fireEvent.click(screen.getByRole("button", { name: /^Cancel$/i }));
        expect(requestPasswordReset).not.toHaveBeenCalled();
        expect(screen.getByRole("button", { name: /forgotten it/i })).toBeDefined();
    });

    // The endpoint keeps one open request per address, so a second press would change
    // nothing. Saying so beats a button that looks like it still does something.
    it("reports it plainly and stops offering once sent", async () => {
        ask();
        fireEvent.click(screen.getByRole("button", { name: /Ask for a reset/i }));
        await waitFor(() => expect(screen.getByText(/Reset requested/i)).toBeDefined());
        expect(screen.queryByRole("button", { name: /forgotten it/i })).toBeNull();
    });

    // It sits inside the change-password form. A bare <button> there defaults to
    // type="submit", which would try to change the password instead of asking for a reset.
    it("does not submit the form it sits in", () => {
        render(<PasswordCard email="ravi@boltvinyls.in" />);
        expect(screen.getByRole("button", { name: /forgotten it/i }).getAttribute("type")).toBe("button");
        expect(changePassword).not.toHaveBeenCalled();
    });
});
