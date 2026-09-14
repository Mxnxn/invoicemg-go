// Default job pipeline. Individual jobs can reorder/extend this in the Queues tab, but
// this is what a fresh job (and the "Complete" progress button) advances through.
export const QUEUE_STAGES = ["Created", "Printing", "Ready-to-Pickup", "Done"];

// The last stage. A card that has reached it is finished; anything else is still work.
export const DONE_STAGE = "Done";

// Whether a job-id still has work on it.
//
// Per-CARD, not per-job: the job-level queue is a summary that a card can move without, so a
// job-id reading "Done" at the top can still hold three cards in Printing. The dashboard's
// "still open" alert counts the same thing (routes/Sheet.js /open-jobs), and the two must
// agree or the number in the alert will not match the list it links to.
export const jobHasOpenCards = (job) => (job?.rows || []).some((row) => row.queue !== DONE_STAGE);

export const openCardCount = (job) => (job?.rows || []).filter((row) => row.queue !== DONE_STAGE).length;

// Extra stages offered in the "add/customize queue" search, on top of QUEUE_STAGES - these
// used to be defaults before the pipeline was narrowed to 4 stages, so they stay one click
// away rather than disappearing outright.
export const CUSTOM_QUEUE_OPTIONS = ["Designed", "Fabrications", "Transportation", "Available/Ready to pick", "Transporter", "Fabricator"];

export const QUEUE_LIBRARY = [...new Set([...QUEUE_STAGES, ...CUSTOM_QUEUE_OPTIONS])];

// Real per-stage timeline, derived from JobHistory rows logged server-side every time a
// job's queue stage advances. historyRows are newest-first (the API's sort order); this
// walks them oldest-first to reconstruct "stage entered on X, sat for N days".
export const buildQueueHistoryFromLog = (job, historyRows) => {
    const advances = [...historyRows]
        .filter((h) => h.action === "Queue advanced")
        .sort((a, b) => new Date(a.createdAt) - new Date(b.createdAt));

    const stages = [{ stage: "Created", enteredOn: job.createdAt }];
    advances.forEach((h) => {
        const [, to] = h.detail.split(" → ");
        stages.push({ stage: to, enteredOn: h.createdAt });
    });

    return stages.map((s, idx) => {
        const isCurrent = idx === stages.length - 1;
        const enteredDate = new Date(s.enteredOn);
        const nextEntered = isCurrent ? new Date() : new Date(stages[idx + 1].enteredOn);
        const days = Math.max(1, Math.round((nextEntered.getTime() - enteredDate.getTime()) / 86400000));
        return { stage: s.stage, enteredOn: enteredDate.toISOString().slice(0, 10), days, current: isCurrent };
    });
};
