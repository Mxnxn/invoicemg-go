import { useEffect } from "react";
import { motion, useReducedMotion } from "motion/react";
import { CheckIcon, XIcon } from "lucide-react";

import { cn } from "../lib/cn";

/** The default pipeline, mirroring the queue stages the lifecycle view shows. */
export const STAGES = ["Received", "Design", "Plate", "Printing", "Lamination", "Cutting", "Packing", "Dispatched"];

// The app's shared easing (docs/STYLE.md "Motion"). 0.12s matches --motion-fast;
// the panel gets a touch longer because it travels further.
const EASE = [0.16, 1, 0.3, 1];

/**
 * A pared-back stand-in for Lifecycle's JobDetailModal, sized for the landing
 * page's browser mockup: header, stage strip, and the two lines that say what
 * last happened. Static placeholder data - it never talks to the API.
 */
export function JobDetailPreview({ row, onClose }) {
    const reducedMotion = useReducedMotion();
    const { job } = row;
    const currentIndex = STAGES.indexOf(job.stage);

    useEffect(() => {
        const onKeyDown = (event) => {
            if (event.key === "Escape") onClose();
        };
        window.addEventListener("keydown", onKeyDown);
        return () => window.removeEventListener("keydown", onKeyDown);
    }, [onClose]);

    return (
        <div className="tw:absolute tw:inset-0 tw:z-20 tw:flex tw:items-end tw:overflow-hidden tw:rounded-2xl">
            <motion.div
                data-testid="job-preview-scrim"
                onClick={onClose}
                aria-hidden
                className="tw:absolute tw:inset-0 tw:bg-background/70 tw:backdrop-blur-[2px]"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: reducedMotion ? 0 : 0.12, ease: EASE }}
            />

            <motion.div
                role="dialog"
                aria-label={`Job ${job.number}`}
                className="tw:relative tw:w-full tw:rounded-t-2xl tw:border-t tw:border-border tw:bg-card tw:p-5 tw:text-left tw:shadow-2xl"
                initial={reducedMotion ? { opacity: 0 } : { y: "12%", opacity: 0 }}
                animate={{ y: 0, opacity: 1 }}
                exit={reducedMotion ? { opacity: 0 } : { y: "12%", opacity: 0 }}
                transition={{ duration: reducedMotion ? 0 : 0.28, ease: EASE }}
            >
                <div className="tw:flex tw:items-start tw:justify-between tw:gap-4">
                    <div className="tw:min-w-0">
                        <p className="tw:flex tw:flex-wrap tw:items-center tw:gap-2 tw:text-sm tw:font-semibold">
                            {job.number}
                            <span className="tw:rounded-full tw:bg-chip tw:px-2 tw:py-0.5 tw:text-xs tw:font-medium tw:text-chip-foreground">
                                {job.stage}
                            </span>
                        </p>
                        <p className="tw:mt-1 tw:truncate tw:text-xs tw:text-muted-foreground">
                            {row.party} · {row.amount} · {job.quantity}
                        </p>
                    </div>
                    <button
                        type="button"
                        onClick={onClose}
                        aria-label={`Close job ${job.number}`}
                        className="tw:shrink-0 tw:rounded-full tw:p-1 tw:text-muted-foreground tw:transition-colors tw:hover:text-foreground tw:focus-visible:outline-2 tw:focus-visible:outline-offset-2 tw:focus-visible:outline-ring"
                    >
                        <XIcon className="tw:size-4" />
                    </button>
                </div>

                <ol className="tw:mt-4 tw:flex tw:flex-wrap tw:gap-x-1 tw:gap-y-2">
                    {STAGES.map((stage, index) => {
                        const state = index < currentIndex ? "done" : index === currentIndex ? "current" : "todo";
                        return (
                            <li key={stage} data-stage-state={state} className="tw:flex tw:items-center tw:gap-1">
                                <motion.span
                                    initial={reducedMotion ? false : { opacity: 0, y: 4 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    // Fills left-to-right as the panel opens.
                                    transition={{ delay: reducedMotion ? 0 : 0.1 + index * 0.05, duration: 0.2, ease: EASE }}
                                    className={cn(
                                        "tw:inline-flex tw:items-center tw:gap-1 tw:rounded-full tw:px-2 tw:py-0.5 tw:text-[11px]",
                                        state === "done" && "tw:bg-muted tw:text-muted-foreground",
                                        // The house status pair, not white-on-brand: white on the
                                        // dark-theme brand orange measures 3.2:1.
                                        state === "current" && "tw:bg-chip tw:text-chip-foreground tw:font-medium",
                                        state === "todo" && "tw:border tw:border-border tw:text-muted-foreground"
                                    )}
                                >
                                    {state === "done" && <CheckIcon className="tw:size-3" />}
                                    {stage}
                                </motion.span>
                                {index < STAGES.length - 1 && (
                                    <span aria-hidden className="tw:text-muted-foreground/40">
                                        ·
                                    </span>
                                )}
                            </li>
                        );
                    })}
                </ol>

                {/* Lifecycle's Rows table, pared to what fits: what it is, how many,
                    who has it, and where that row has reached. */}
                <p className="tw:mt-4 tw:mb-1.5 tw:text-[11px] tw:font-medium tw:tracking-wide tw:text-muted-foreground tw:uppercase">
                    Rows
                </p>
                <ul className="tw:overflow-hidden tw:rounded-lg tw:border tw:border-border">
                    {job.rows.map((jobRow, index) => (
                        <motion.li
                            key={jobRow.material}
                            data-job-row
                            initial={reducedMotion ? false : { opacity: 0, x: -6 }}
                            animate={{ opacity: 1, x: 0 }}
                            transition={{ delay: reducedMotion ? 0 : 0.16 + index * 0.08, duration: 0.2, ease: EASE }}
                            className="tw:flex tw:items-center tw:gap-3 tw:border-b tw:border-border tw:px-3 tw:py-2 tw:text-xs tw:last:border-b-0"
                        >
                            <span className="tw:min-w-0 tw:flex-1">
                                <span className="tw:block tw:truncate tw:font-medium">{jobRow.material}</span>
                                <span className="tw:block tw:truncate tw:text-[11px] tw:text-muted-foreground">
                                    {jobRow.size} · {jobRow.qty} qty
                                </span>
                            </span>
                            <span className="tw:hidden tw:shrink-0 tw:text-muted-foreground tw:sm:inline">
                                {jobRow.employee}
                            </span>
                            <span className="tw:shrink-0 tw:rounded-full tw:bg-chip tw:px-2 tw:py-0.5 tw:text-[11px] tw:font-medium tw:text-chip-foreground">
                                {jobRow.queue}
                            </span>
                            <span className="tw:hidden tw:shrink-0 tw:text-[11px] tw:text-muted-foreground tw:md:inline">
                                {jobRow.progress}
                            </span>
                        </motion.li>
                    ))}
                </ul>

                <p className="tw:mt-3 tw:truncate tw:text-[11px] tw:text-muted-foreground">
                    {job.note} · moved to {job.stage} {job.updated}
                </p>
            </motion.div>
        </div>
    );
}
