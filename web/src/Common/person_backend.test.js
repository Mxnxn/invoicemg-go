import { describe, it, expect, vi, beforeEach } from "vitest";
import axios from "axios";
import { personBackend } from "./person_backend";

vi.mock("axios");

describe("personBackend", () => {
    beforeEach(() => {
        vi.clearAllMocks();
        window.localStorage.setItem("session_token", "test-token");
    });

    it("list resolves with data on code 200", async () => {
        axios.post.mockResolvedValue({ data: { code: 200, message: "ok", data: [{ _id: "1", name: "Raj" }], status: true } });
        const result = await personBackend.list();
        expect(result.data).toEqual([{ _id: "1", name: "Raj" }]);
    });

    it("create rejects on duplicate email (code 422)", async () => {
        axios.post.mockResolvedValue({ data: { code: 422, message: "That email is already registered to a person.", status: false } });
        await expect(personBackend.create({ name: "Raj", type: "Employee" })).rejects.toEqual({
            code: 422,
            message: "That email is already registered to a person.",
            status: false,
        });
    });
});
