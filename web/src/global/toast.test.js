import { describe, it, expect, vi, afterEach } from "vitest";
import { notifyError, notifySuccess, notifyWarning, subscribeToasts, resetToasts } from "./toast";

afterEach(() => {
    resetToasts();
});

describe("notifyError", () => {
    it("delivers the message to a subscriber", () => {
        const seen = vi.fn();
        subscribeToasts(seen);

        notifyError("Something broke");

        expect(seen).toHaveBeenCalledTimes(1);
        expect(seen.mock.calls[0][0]).toMatchObject({ variant: "error", message: "Something broke" });
    });

    it("falls back to a default message when none is given", () => {
        const seen = vi.fn();
        subscribeToasts(seen);

        notifyError();

        expect(seen.mock.calls[0][0].message).toBe("Something went wrong");
    });

    it("carries a status code and a description when given", () => {
        const seen = vi.fn();
        subscribeToasts(seen);

        notifyError("Invalid request.", { code: 422, description: "/invoice/save" });

        expect(seen.mock.calls[0][0]).toMatchObject({ code: 422, description: "/invoice/save" });
    });

    it("gives every toast a distinct id, so a repeated message stacks rather than replacing", () => {
        const seen = vi.fn();
        subscribeToasts(seen);

        notifyError("Invalid request.");
        notifyError("Invalid request.");

        const [a, b] = seen.mock.calls.map((call) => call[0]);
        expect(a.id).not.toBe(b.id);
    });
});

// The interceptor is module-level code and can fire before the provider has mounted. The old
// implementation dropped those messages on the floor; this is the case that proves it does not.
describe("messages raised before anything is listening", () => {
    it("are held and delivered on the first subscribe", () => {
        notifyError("Raised early");

        const seen = vi.fn();
        subscribeToasts(seen);

        expect(seen).toHaveBeenCalledTimes(1);
        expect(seen.mock.calls[0][0].message).toBe("Raised early");
    });

    it("are delivered once only, not again to the next subscriber", () => {
        notifyError("Raised early");

        const first = vi.fn();
        const second = vi.fn();
        subscribeToasts(first);
        subscribeToasts(second);

        expect(first).toHaveBeenCalledTimes(1);
        expect(second).not.toHaveBeenCalled();
    });
});

describe("the other variants", () => {
    it("tag themselves correctly", () => {
        const seen = vi.fn();
        subscribeToasts(seen);

        notifySuccess("Saved");
        notifyWarning("Careful");

        expect(seen.mock.calls[0][0]).toMatchObject({ variant: "success", message: "Saved" });
        expect(seen.mock.calls[1][0]).toMatchObject({ variant: "warning", message: "Careful" });
    });
});

describe("unsubscribe", () => {
    it("stops delivery", () => {
        const seen = vi.fn();
        const off = subscribeToasts(seen);
        off();

        notifyError("boom");

        expect(seen).not.toHaveBeenCalled();
    });
});
