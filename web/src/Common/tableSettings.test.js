import { describe, it, expect, beforeEach, vi } from "vitest";
import axios from "axios";

vi.mock("axios", () => ({ default: { post: vi.fn(() => Promise.resolve({ data: { code: 200, data: null } })) } }));

const KEY = "lifecycle_visible_columns";

// Loads the module fresh, so its import-time snapshot of localStorage is taken with whatever
// this test seeded - the snapshot is the whole mechanism being tested.
const loadModule = async () => {
    vi.resetModules();
    return import("./tableSettings");
};

beforeEach(() => {
    // Fake timers, and cleared between tests: the push is debounced, so a pending timer from
    // the previous test would otherwise fire during this one - against a stale module whose
    // collect() reads the localStorage this test just seeded, and post a request nobody asked
    // for. That is a test-isolation trap, not a bug in the module.
    vi.useFakeTimers();
    vi.clearAllTimers();
    window.localStorage.clear();
    window.localStorage.setItem("session_token", "t");
    vi.clearAllMocks();
});

describe("pushTableSettings", () => {
    // The bug this exists for. LifecycleIndex writes its column state in a mount effect, so
    // merely OPENING the board pushed the values it had just read back to the server - which
    // answers "Settings saved.", which the interceptor toasts. Visiting a page announced that
    // it had saved something.
    it("sends nothing when the values match what the server already has", async () => {
        window.localStorage.setItem(KEY, JSON.stringify(["a", "b"]));
        const { writeTableSetting } = await loadModule();

        writeTableSetting(KEY, ["a", "b"]);
        await vi.runAllTimersAsync();
        expect(axios.post).not.toHaveBeenCalled();
    });

    it("sends when a column arrangement actually changes", async () => {
        window.localStorage.setItem(KEY, JSON.stringify(["a", "b"]));
        const { writeTableSetting } = await loadModule();

        writeTableSetting(KEY, ["b", "a"]);
        await vi.runAllTimersAsync();
        expect(axios.post).toHaveBeenCalledTimes(1);
    });

    it("sends when a column is hidden", async () => {
        window.localStorage.setItem(KEY, JSON.stringify(["a", "b"]));
        const { writeTableSetting } = await loadModule();

        writeTableSetting(KEY, ["a"]);
        await vi.runAllTimersAsync();
        expect(axios.post).toHaveBeenCalledTimes(1);
    });

    // Two changes in quick succession are one save, not two - dragging a column fires
    // repeatedly and each one would otherwise be a request and a toast.
    it("collapses a burst of changes into one request", async () => {
        window.localStorage.setItem(KEY, JSON.stringify(["a", "b"]));
        const { writeTableSetting } = await loadModule();

        writeTableSetting(KEY, ["b", "a"]);
        writeTableSetting(KEY, ["b", "a", "c"]);
        writeTableSetting(KEY, ["c", "b", "a"]);
        await vi.runAllTimersAsync();
        expect(axios.post).toHaveBeenCalledTimes(1);
    });

    // Having saved, settling back to the saved arrangement is not a new save.
    it("does not re-send the same arrangement twice", async () => {
        window.localStorage.setItem(KEY, JSON.stringify(["a", "b"]));
        const { writeTableSetting } = await loadModule();

        writeTableSetting(KEY, ["b", "a"]);
        await vi.runAllTimersAsync();
        expect(axios.post).toHaveBeenCalledTimes(1);

        writeTableSetting(KEY, ["b", "a"]);
        await vi.runAllTimersAsync();
        expect(axios.post).toHaveBeenCalledTimes(1);
    });

    it("stays silent when there is no session to save against", async () => {
        window.localStorage.removeItem("session_token");
        const { writeTableSetting } = await loadModule();

        writeTableSetting(KEY, ["b", "a"]);
        await vi.runAllTimersAsync();
        expect(axios.post).not.toHaveBeenCalled();
    });
});
