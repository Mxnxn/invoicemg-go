import { useCallback, useEffect, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";

import { cn } from "../lib/cn";
import { BorderBeam } from "../velora/border-beam";
import { JobDetailPreview } from "./JobDetailPreview";

// Placeholder figures - swap for real numbers before launch.
const STATS = [
    { label: "Outstanding", value: "₹4,82,150" },
    { label: "This month", value: "₹11,20,400" },
    { label: "Jobs in queue", value: "18" },
];

// Each job carries two rows, mirroring Lifecycle's Rows table: material, size,
// quantity, the employee it is assigned to, its own queue stage and progress.
// The two rows sit at different stages on purpose - that is what the real board
// looks like mid-job.
const ROWS = [
    {
        no: "INV-1042",
        party: "Sharma Packaging",
        amount: "₹48,200",
        status: "Paid",
        job: {
            number: "JOB-318",
            stage: "Printing",
            quantity: "4,000 sheets",
            note: "Plate approved by client",
            updated: "2h ago",
            rows: [
                { material: "Art card 300gsm", size: "18 × 23 in", qty: "2,500", employee: "Ramesh P.", queue: "Printing", progress: "In progress" },
                { material: "Kraft liner", size: "20 × 30 in", qty: "1,500", employee: "Imran S.", queue: "Plate", progress: "Done" },
            ],
        },
    },
    {
        no: "INV-1041",
        party: "Vertex Labels",
        amount: "₹1,12,000",
        status: "Sent",
        job: {
            number: "JOB-317",
            stage: "Lamination",
            quantity: "12,500 labels",
            note: "Matt finish, not gloss",
            updated: "5h ago",
            rows: [
                { material: "Chromo label", size: "4 × 6 in", qty: "8,000", employee: "Kavita R.", queue: "Lamination", progress: "In progress" },
                { material: "Vinyl sticker", size: "3 × 5 in", qty: "4,500", employee: "Suresh M.", queue: "Cutting", progress: "Assign" },
            ],
        },
    },
    {
        no: "INV-1040",
        party: "Northwind Cartons",
        amount: "₹26,750",
        status: "Overdue",
        job: {
            number: "JOB-311",
            stage: "Design",
            quantity: "900 cartons",
            note: "Awaiting revised artwork",
            updated: "yesterday",
            rows: [
                { material: "Duplex board", size: "24 × 36 in", qty: "600", employee: "Dinesh K.", queue: "Design", progress: "In progress" },
                { material: "Corrugated 5ply", size: "24 × 36 in", qty: "300", employee: "Anita V.", queue: "Received", progress: "Assign" },
            ],
        },
    },
    {
        no: "INV-1039",
        party: "Apex Printers",
        amount: "₹73,400",
        status: "Paid",
        job: {
            number: "JOB-309",
            stage: "Dispatched",
            quantity: "2,200 sheets",
            note: "Collected from the gate",
            updated: "2 days ago",
            rows: [
                { material: "Maplitho 90gsm", size: "18 × 23 in", qty: "1,400", employee: "Ramesh P.", queue: "Dispatched", progress: "Done" },
                { material: "Art paper 130gsm", size: "18 × 23 in", qty: "800", employee: "Kavita R.", queue: "Packing", progress: "Done" },
            ],
        },
    },
];

// Each row gets two beats: closed (the table reads first), then open.
const CLOSED_MS = 1000;
const OPEN_MS = 2600;

/**
 * Static stand-in for the admin dashboard - no data, no API, purely
 * illustrative. The invoice rows demo themselves: one opens its job panel,
 * holds, closes, and the next takes over. Clicking a row takes control and
 * stops the loop; closing the panel hands it back.
 */
export function DashboardMockup({ className }) {
    const reducedMotion = useReducedMotion();
    const [step, setStep] = useState(0);
    const [picked, setPicked] = useState(null);

    const autoIndex = Math.floor(step / 2) % ROWS.length;
    const autoOpen = step % 2 === 1;
    const activeIndex = picked !== null ? picked : autoOpen ? autoIndex : null;
    // During the closed beat the row that is about to open is already outlined,
    // so the highlight leads the panel rather than arriving with it.
    const armedIndex = picked === null && !autoOpen ? autoIndex : null;

    useEffect(() => {
        // A hero that animates forever is exactly what this setting is for.
        if (picked !== null || reducedMotion) return undefined;
        const id = setTimeout(() => setStep((s) => s + 1), autoOpen ? OPEN_MS : CLOSED_MS);
        return () => clearTimeout(id);
    }, [step, picked, autoOpen, reducedMotion]);

    // Closing hands control back to the loop, resuming on the row after this one.
    const close = useCallback(() => {
        setPicked((current) => {
            if (current !== null) setStep(current * 2 + 2);
            return null;
        });
    }, []);

    const activeRow = activeIndex === null ? null : ROWS[activeIndex];

    return (
        <div
            className={cn(
                "tw:relative tw:overflow-hidden tw:rounded-2xl tw:border tw:border-border tw:bg-card tw:shadow-2xl",
                className
            )}
        >
            <BorderBeam size={140} duration={10} />
            <div className="tw:flex tw:items-center tw:gap-2 tw:border-b tw:border-border tw:px-4 tw:py-3">
                <span className="tw:size-3 tw:rounded-full tw:bg-muted-foreground/30" />
                <span className="tw:size-3 tw:rounded-full tw:bg-muted-foreground/30" />
                <span className="tw:size-3 tw:rounded-full tw:bg-muted-foreground/30" />
                <span className="tw:ml-3 tw:text-xs tw:text-muted-foreground">invoicemg.in/admin/invoices</span>
            </div>
            <div className="tw:p-6 tw:text-left">
                <div className="tw:grid tw:grid-cols-3 tw:gap-4">
                    {STATS.map((stat) => (
                        <div key={stat.label} className="tw:rounded-xl tw:border tw:border-border tw:p-4">
                            <p className="tw:text-xs tw:text-muted-foreground">{stat.label}</p>
                            <p className="tw:mt-1 tw:text-xl tw:font-semibold">{stat.value}</p>
                        </div>
                    ))}
                </div>
                <p className="tw:mt-6 tw:mb-3 tw:text-sm tw:font-medium">Invoices</p>
                <div className="tw:overflow-hidden tw:rounded-xl tw:border tw:border-border">
                    {ROWS.map((row, index) => (
                        <motion.button
                            key={row.no}
                            type="button"
                            aria-label={`Open job ${row.job.number} for ${row.party}`}
                            data-row-state={
                                activeIndex === index ? "active" : armedIndex === index ? "armed" : "idle"
                            }
                            onClick={() => setPicked(index)}
                            animate={
                                reducedMotion ? undefined : { scale: activeIndex === index ? 1.015 : 1 }
                            }
                            transition={{ duration: 0.12, ease: [0.16, 1, 0.3, 1] }}
                            className={cn(
                                "tw:relative tw:flex tw:w-full tw:items-center tw:justify-between tw:gap-4 tw:border-b tw:border-border tw:px-4 tw:py-3 tw:text-left tw:text-sm tw:transition-colors tw:last:border-b-0",
                                "tw:hover:bg-muted tw:focus-visible:outline-2 tw:focus-visible:-outline-offset-2 tw:focus-visible:outline-ring",
                                // Armed: outlined, no fill yet. Active: filled.
                                armedIndex === index &&
                                    "tw:bg-primary/5 tw:ring-1 tw:ring-inset tw:ring-primary/60",
                                activeIndex === index && "tw:bg-muted"
                            )}
                        >
                            <span className="tw:font-mono tw:text-xs tw:text-muted-foreground">{row.no}</span>
                            <span className="tw:flex-1 tw:truncate">{row.party}</span>
                            <span className="tw:font-medium">{row.amount}</span>
                            <span className="tw:rounded-full tw:bg-muted tw:px-2 tw:py-0.5 tw:text-xs tw:text-muted-foreground">
                                {row.status}
                            </span>
                        </motion.button>
                    ))}
                </div>
            </div>

            {/* mode="wait" so a closing panel finishes before the next row's opens,
                rather than the two overlapping mid-cycle. */}
            <AnimatePresence mode="wait">
                {activeRow && <JobDetailPreview key={activeRow.no} row={activeRow} onClose={close} />}
            </AnimatePresence>
        </div>
    );
}
