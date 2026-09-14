import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { Trash2, RotateCcw } from "react-feather";
import { notifyError } from "../global/toast";

// Undo-able deletes, shown in the xan-bulk-bar.
//
// Nothing is deleted while the bar is up. The row is removed from the caller's local list
// immediately, the DELETE request is held for UNDO_WINDOW_MS, and Undo simply cancels the
// pending request - so undo can't fail, and no data is ever destroyed and resurrected.
//
// The alternative (delete now, restore on undo) isn't available: there is no restore endpoint
// in the API. routes/Trash.js only exposes /setPassword and /get, and only Entry snapshots
// into Trash at all - every other route hard-deletes via findOneAndDelete. Re-creating a
// deleted document would also mint a new _id and orphan anything referencing the old one
// (payment allocations, entry->invoice links), which is not something to do to money records.

// Long enough to notice the bar, read it and reach for Undo - 2s proved too quick to act on,
// especially on the money records under Bank Transfers. The bar animates a depleting track
// for the same duration, so the deadline is visible rather than something you have to feel.
const UNDO_WINDOW_MS = 5000;

const UndoDeleteContext = createContext(null);

export const useUndoDelete = () => {
    const ctx = useContext(UndoDeleteContext);
    // Callers shouldn't have to care whether the provider is mounted (some views render
    // outside AdminLayout) - fall back to deleting immediately rather than throwing.
    if (!ctx) {
        return {
            scheduleDelete: async ({ commit }) => {
                try {
                    await commit();
                } catch (error) {
                    notifyError(error?.message || "Couldn't delete this.");
                }
            },
        };
    }
    return ctx;
};

export const UndoDeleteProvider = ({ children }) => {
    const [pending, setPending] = useState(null);
    // Held in a ref as well so the unmount/pagehide flush can reach the live value without
    // re-subscribing on every state change.
    const pendingRef = useRef(null);
    const timerRef = useRef(null);

    const clearTimer = () => {
        if (timerRef.current) {
            clearTimeout(timerRef.current);
            timerRef.current = null;
        }
    };

    const runCommit = useCallback(async (task) => {
        if (!task || task.done) return;
        task.done = true;
        try {
            await task.commit();
        } catch (error) {
            notifyError(error?.message || `Couldn't delete ${task.label}.`);
            // The delete failed, so the row still exists on the server - put it back in the
            // list rather than leaving the UI claiming it's gone.
            task.undo?.();
        }
    }, []);

    const finish = useCallback(
        async (task) => {
            clearTimer();
            pendingRef.current = null;
            setPending(null);
            await runCommit(task);
        },
        [runCommit]
    );

    // A second delete while one is pending commits the first immediately - queueing them would
    // mean several bars or a hidden backlog, and "delete two things quickly" shouldn't silently
    // extend the first one's window.
    const scheduleDelete = useCallback(
        ({ label = "item", commit, undo }) => {
            const previous = pendingRef.current;
            if (previous) {
                clearTimer();
                runCommit(previous);
            }
            // A per-task id so the bar can be keyed on it - see the render below.
            const task = { id: Date.now() + Math.random(), label, commit, undo, done: false };
            pendingRef.current = task;
            setPending(task);
            timerRef.current = setTimeout(() => finish(task), UNDO_WINDOW_MS);
        },
        [finish, runCommit]
    );

    const onUndo = () => {
        const task = pendingRef.current;
        clearTimer();
        pendingRef.current = null;
        setPending(null);
        if (task && !task.done) {
            task.done = true;
            // No request was ever sent, so this is just putting the row back on screen.
            task.undo?.();
        }
    };

    // Leaving the page with a delete still pending must commit it, not drop it - otherwise the
    // row vanishes, the user navigates away, and it's back on the next load.
    useEffect(() => {
        const flush = () => {
            const task = pendingRef.current;
            if (task && !task.done) {
                task.done = true;
                task.commit();
            }
        };
        window.addEventListener("pagehide", flush);
        return () => {
            window.removeEventListener("pagehide", flush);
            clearTimer();
            flush();
        };
    }, []);

    return (
        <UndoDeleteContext.Provider value={{ scheduleDelete }}>
            {children}
            {pending && (
                // Keyed by the task so a second delete restarts the countdown animation
                // instead of inheriting the first one's remaining sliver.
                <div className="xan-bulk-bar xan-undo-bar" role="status" key={pending.id}>
                    <div className="xan-bulk-bar-count">
                        <span className="badge">
                            <Trash2 size={15} />
                        </span>
                        <span className="label">Deleted {pending.label}</span>
                    </div>
                    <div className="xan-bulk-bar-actions">
                        <button type="button" onClick={onUndo}>
                            <RotateCcw size={15} />
                            Undo
                        </button>
                    </div>
                    <span
                        className="xan-undo-track"
                        style={{ animationDuration: `${UNDO_WINDOW_MS}ms` }}
                        aria-hidden="true"
                    />
                </div>
            )}
        </UndoDeleteContext.Provider>
    );
};

export { UNDO_WINDOW_MS };
