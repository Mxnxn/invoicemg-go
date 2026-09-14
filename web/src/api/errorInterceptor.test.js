vi.mock("../global/toast", () => ({
    notifyError: vi.fn(),
}));

import { notifyError } from "../global/toast";
import { onFulfilled, onRejected } from "./errorInterceptor";

describe("onFulfilled", () => {
    afterEach(() => jest.clearAllMocks());

    it("returns the response unchanged when code is 200", () => {
        const response = { data: { code: 200, message: "ok" } };

        expect(onFulfilled(response)).toBe(response);
        expect(notifyError).not.toHaveBeenCalled();
    });

    it("notifies with the response message when code is not 200", () => {
        const response = { data: { code: 422, message: "Invalid request" } };

        onFulfilled(response);

        expect(notifyError).toHaveBeenCalledWith("Invalid request", expect.objectContaining({ code: 422 }));
    });

    // The envelope's code, not the HTTP status. This API answers 200 with a `code` in the
    // body, so the HTTP status is 200 on every business failure and says nothing.
    it("shows the envelope code and the endpoint, not the transport status", () => {
        const response = {
            data: { code: 422, message: "Invalid request" },
            config: { url: "http://localhost:5001/invoice/save?x=1" },
        };

        onFulfilled(response);

        expect(notifyError).toHaveBeenCalledWith("Invalid request", {
            code: 422,
            description: "/invoice/save",
        });
    });

    it("ignores responses without a code field", () => {
        const response = { data: { foo: "bar" } };

        onFulfilled(response);

        expect(notifyError).not.toHaveBeenCalled();
    });
});

describe("onRejected", () => {
    afterEach(() => jest.clearAllMocks());

    it("notifies with the server error message and re-rejects with the original error", async () => {
        const error = { response: { data: { message: "Invalid password." } } };

        await expect(onRejected(error)).rejects.toBe(error);
        expect(notifyError).toHaveBeenCalledWith("Invalid password.", expect.any(Object));
    });

    it("falls back to error.message when there is no response body", async () => {
        const error = { message: "Network Error" };

        await expect(onRejected(error)).rejects.toBe(error);
        expect(notifyError).toHaveBeenCalledWith("Network Error", expect.any(Object));
    });

    // A request that never reached the server has no status at all. A blank chip would read
    // as a missing value; "Network" is the distinction actually worth showing.
    it("labels a request that never got a response as Network rather than blank", async () => {
        const error = { message: "Network Error", config: { url: "/invoice/save" } };

        await expect(onRejected(error)).rejects.toBe(error);
        expect(notifyError).toHaveBeenCalledWith("Network Error", { code: "Network", description: "/invoice/save" });
    });

    it("uses the HTTP status when there was a response", async () => {
        const error = { response: { status: 500, data: { message: "Internal Error" } }, config: { url: "/invoice/save" } };

        await expect(onRejected(error)).rejects.toBe(error);
        expect(notifyError).toHaveBeenCalledWith("Internal Error", { code: 500, description: "/invoice/save" });
    });
});

// The login form owns its own error line, and a failed admin login deliberately retries
// against the employee endpoint (AuthForm.jsx) - so a single wrong password produces TWO
// failing requests. Toasting both put two identical errors on screen for one mistake.
describe("login errors are left to the form", () => {
    // vi, not jest: the older blocks in this file call jest.clearAllMocks(), which is not
    // defined under Vitest, so their afterEach throws and the mock is never actually reset.
    // Those assertions use toHaveBeenCalledWith and tolerate the leftovers; "was not called
    // at all" does not, so this block clears the mock itself.
    beforeEach(() => vi.clearAllMocks());

    it("does not toast a failed admin login", () => {
        const response = { data: { code: 422, message: "Invalid credential." }, config: { url: "http://api/user/login" } };

        onFulfilled(response);

        expect(notifyError).not.toHaveBeenCalled();
    });

    it("does not toast the employee-login fallback either", () => {
        const response = { data: { code: 422, message: "Invalid credential." }, config: { url: "http://api/person/login" } };

        onFulfilled(response);

        expect(notifyError).not.toHaveBeenCalled();
    });

    it("does not toast a rejected login request", () => {
        const error = { config: { url: "http://api/user/login" }, message: "Network error" };

        expect(() => onRejected(error)).toBeDefined();
        onRejected(error).catch(() => {});

        expect(notifyError).not.toHaveBeenCalled();
    });

    // Everything else still speaks up - the point is to silence one noisy pair, not to make
    // the interceptor quiet in general.
    it("still toasts other failures", () => {
        const response = { data: { code: 422, message: "Invalid request" }, config: { url: "http://api/client/add" } };

        onFulfilled(response);

        expect(notifyError).toHaveBeenCalledWith("Invalid request", expect.any(Object));
    });

    // A logout must still happen even though the toast is suppressed: the session being
    // invalid is not a login-form error, and leaving the user on a dead session is worse
    // than a missing toast.
    it("still logs out on a 401 from a login path", () => {
        const response = { data: { code: 401, message: "Unauthorized." }, config: { url: "http://api/user/login" } };

        expect(() => onFulfilled(response)).not.toThrow();
        expect(notifyError).not.toHaveBeenCalled();
    });
});

// The Go service can answer in either style (API_STYLE), and the Node API only answers in the
// first, so this client has to understand both:
//
//   legacy  HTTP 200 with {code: 401} in the body  -> axios resolves -> onFulfilled
//   rest    HTTP 401                               -> axios rejects  -> onRejected
//
// Before the session effects were shared between the two paths, a 401 under rest style
// toasted and then did nothing: the dead session stayed in localStorage and the app kept
// calling with it, looking broken rather than logged out.
describe("a real HTTP status, as the Go service sends under API_STYLE=rest", () => {
    const originalLocation = window.location;

    beforeEach(() => {
        jest.clearAllMocks();
        delete window.location;
        window.location = { pathname: "/admin/lifecycle", assign: vi.fn(), href: "" };
        window.localStorage.clear();
    });

    afterEach(() => {
        window.location = originalLocation;
    });

    const rejection = (status, body, url = "http://api/sheet/only") => ({
        config: { url },
        response: { status, data: body },
    });

    it("clears the session on a 401 that arrived as a status rather than a body code", async () => {
        window.localStorage.setItem("uid", "u1");

        await expect(onRejected(rejection(401, { code: 401, message: "Unauthorized." }))).rejects.toBeTruthy();

        expect(window.localStorage.getItem("uid")).toBeNull();
        expect(window.location.assign).toHaveBeenCalled();
    });

    // The status alone has to be enough: a proxy or a dropped connection produces a failure
    // with no envelope at all, and a dead session must still be cleared.
    it("clears the session on a bare 401 with no body", async () => {
        window.localStorage.setItem("uid", "u1");

        await expect(onRejected(rejection(401, undefined))).rejects.toBeTruthy();

        expect(window.localStorage.getItem("uid")).toBeNull();
    });

    it("prefers the body code over the status when both are present", async () => {
        await expect(onRejected(rejection(422, { code: 422, message: "Invalid request." }))).rejects.toBeTruthy();

        expect(notifyError).toHaveBeenCalledWith("Invalid request.", expect.objectContaining({ code: 422 }));
    });

    // Silenced for the toast, never for the consequences - the same rule the legacy path has.
    it("still clears the session on a 401 from a login path without toasting", async () => {
        window.localStorage.setItem("uid", "u1");

        await expect(
            onRejected(rejection(401, { code: 401, message: "Unauthorized." }, "http://api/user/login"))
        ).rejects.toBeTruthy();

        expect(notifyError).not.toHaveBeenCalled();
        expect(window.localStorage.getItem("uid")).toBeNull();
    });

    it("leaves an ordinary failure alone", async () => {
        window.localStorage.setItem("uid", "u1");

        await expect(onRejected(rejection(500, { code: 500, message: "Internal Error" }))).rejects.toBeTruthy();

        expect(window.localStorage.getItem("uid")).toBe("u1");
        expect(notifyError).toHaveBeenCalled();
    });
});
