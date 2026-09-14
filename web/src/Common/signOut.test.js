import { beforeEach, describe, expect, it, vi } from "vitest";

const post = vi.fn();
vi.mock("axios", () => ({ default: { post: (...a) => post(...a) } }));

import { signOut, clearSession, SESSION_KEYS, LOGIN_PATH } from "./signOut";

const assign = vi.fn();

beforeEach(() => {
    vi.clearAllMocks();
    post.mockResolvedValue({ data: { code: 200 } });
    SESSION_KEYS.forEach((k) => window.localStorage.setItem(k, "x"));
    delete window.location;
    window.location = { assign, pathname: "/admin/lifecycle" };
});

const stored = () => SESSION_KEYS.filter((k) => window.localStorage.getItem(k) !== null);

describe("signOut", () => {
    // The navbar's version removed three of the five, so a signed-out browser still held a
    // `role` and a `permissions` list and went on claiming to be an admin.
    it("clears every key login writes", async () => {
        await signOut();
        expect(stored()).toEqual([]);
    });

    // The old version awaited with no try/catch: a dropped network threw, nothing caught it,
    // and the button did nothing at all.
    it("signs out anyway when the request fails", async () => {
        post.mockRejectedValue(new Error("Network Error"));
        await signOut();
        expect(stored()).toEqual([]);
        expect(assign).toHaveBeenCalledWith(LOGIN_PATH);
    });

    // And it only cleared storage on code === 200, so an already-expired session answered
    // 401, hit the else branch, alerted, and left you logged in.
    it("signs out anyway when the session is already dead", async () => {
        post.mockResolvedValue({ data: { code: 401, message: "Unauthorized." } });
        await signOut();
        expect(stored()).toEqual([]);
        expect(assign).toHaveBeenCalledWith(LOGIN_PATH);
    });

    // "/" is the public marketing page - landing there gives no sign of having signed out.
    it("lands on the login screen, not the marketing page", async () => {
        await signOut();
        expect(assign).toHaveBeenCalledWith("/admin");
        expect(assign).not.toHaveBeenCalledWith("/");
    });

    it("still tells the server, so the token cannot be replayed", async () => {
        await signOut();
        expect(post).toHaveBeenCalled();
        expect(post.mock.calls[0][0]).toMatch(/\/user\/logout$/);
    });

    // Nothing to revoke, so nothing to ask.
    it("skips the request when there is no token", async () => {
        window.localStorage.removeItem("session_token");
        await signOut();
        expect(post).not.toHaveBeenCalled();
        expect(assign).toHaveBeenCalledWith(LOGIN_PATH);
    });

    it("can clear without redirecting", async () => {
        await signOut({ redirect: false });
        expect(stored()).toEqual([]);
        expect(assign).not.toHaveBeenCalled();
    });

    it("clearSession alone removes everything", () => {
        clearSession();
        expect(stored()).toEqual([]);
    });
});
