import { describe, it, expect } from "vitest";
import { buildQueueHistoryFromLog } from "./queueConstants";

describe("buildQueueHistoryFromLog", () => {
    it("returns just the Created stage when there is no history", () => {
        const job = { queue: "Created", createdAt: "2026-08-01T09:00:00Z" };
        const result = buildQueueHistoryFromLog(job, []);
        expect(result).toEqual([{ stage: "Created", enteredOn: "2026-08-01", days: expect.any(Number), current: true }]);
    });

    it("walks Queue advanced rows to build the stage-by-stage timeline", () => {
        const job = { queue: "Printing", createdAt: "2026-08-01T09:00:00Z" };
        const history = [
            // newest-first, matching the API's sort order
            { action: "Queue advanced", detail: "Designed → Printing", createdAt: "2026-08-03T09:00:00Z" },
            { action: "Progress changed", detail: "Unassigned → In Progress", createdAt: "2026-08-02T10:00:00Z" },
            { action: "Queue advanced", detail: "Created → Designed", createdAt: "2026-08-02T09:00:00Z" },
        ];
        const result = buildQueueHistoryFromLog(job, history);
        expect(result).toEqual([
            { stage: "Created", enteredOn: "2026-08-01", days: 1, current: false },
            { stage: "Designed", enteredOn: "2026-08-02", days: 1, current: false },
            { stage: "Printing", enteredOn: "2026-08-03", days: expect.any(Number), current: true },
        ]);
    });
});
