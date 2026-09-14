import { describe, it, expect, vi, beforeEach } from "vitest";
import axios from "axios";
import { lifecycleBackend } from "./lifecycle_backend";

vi.mock("axios");

describe("lifecycleBackend", () => {
    beforeEach(() => {
        vi.clearAllMocks();
        window.localStorage.setItem("session_token", "test-token");
    });

    it("listJobs resolves with data on code 200", async () => {
        axios.post.mockResolvedValue({ data: { code: 200, message: "ok", data: [{ _id: "1" }], status: true } });
        const result = await lifecycleBackend.listJobs();
        expect(result.data).toEqual([{ _id: "1" }]);
        expect(axios.post).toHaveBeenCalledWith(
            expect.stringContaining("/lifecycle/jobs/list"),
            {},
            expect.objectContaining({ headers: { "SESSION-TOKEN": "test-token" } })
        );
    });

    it("createJob rejects when code is not 200", async () => {
        axios.post.mockResolvedValue({ data: { code: 422, message: "This challan number is already in use.", status: false } });
        await expect(lifecycleBackend.createJob({ challanNumber: "CH-1" })).rejects.toEqual({
            code: 422,
            message: "This challan number is already in use.",
            status: false,
        });
    });

    it("assignJob posts to /lifecycle/jobs/assign", async () => {
        axios.post.mockResolvedValue({ data: { code: 200, message: "ok", data: { _id: "1" }, status: true } });
        await lifecycleBackend.assignJob({ job_id: "1", type: "employee", person_id: "p1" });
        expect(axios.post).toHaveBeenCalledWith(
            expect.stringContaining("/lifecycle/jobs/assign"),
            { job_id: "1", type: "employee", person_id: "p1" },
            expect.anything()
        );
    });
});
