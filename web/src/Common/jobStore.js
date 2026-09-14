import { useEffect } from "react";

// Job-ids created from outside the Jobs board.
//
// The navbar "+" can raise a job-id from any screen, including the Jobs board itself - but
// the modal lives in AdminLayout and the board holds its own `jobs` state, so a job created
// there appeared nowhere until the page was reloaded. On the very screen that lists them.
//
// A two-line publish/subscribe rather than Redux: the store in this app persists to
// localStorage on every change and is rehydrated on load (see Redux/Reducers/Grab.js), which
// is right for cached lookups and wrong for a live row - it would write every new job-id to
// disk to solve a problem that lasts one render. Nothing here needs to survive a reload; the
// board refetches on mount anyway.

const listeners = new Set();

// Called after the server has confirmed the job - never with an optimistic one, or a failed
// create would leave a row on the board that does not exist.
export function publishJobCreated(job) {
    if (!job) return;
    listeners.forEach((listener) => listener(job));
}

// Subscribe for as long as the component is mounted. The board is the only subscriber today;
// the client detail view could take the same feed if it ever needs to.
export function useJobCreated(onCreated) {
    useEffect(() => {
        if (!onCreated) return undefined;
        listeners.add(onCreated);
        return () => listeners.delete(onCreated);
    }, [onCreated]);
}
