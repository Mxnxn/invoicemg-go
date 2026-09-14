import {
    BanknoteIcon,
    FileCheck2Icon,
    PackageCheckIcon,
    PrinterIcon,
    SheetIcon,
    Trash2Icon,
} from "lucide-react";

import { cn } from "../lib/cn";
import { AnimatedList } from "../velora/animated-list";

const EVENTS = [
    { icon: BanknoteIcon, title: "Invoice INV-1042 paid", meta: "Sharma Packaging · ₹48,200" },
    { icon: PackageCheckIcon, title: "Challan received", meta: "Batch B-221 · 4,000 sheets" },
    { icon: PrinterIcon, title: "Job moved to Printing", meta: "JOB-318 · Offset press 2" },
    { icon: FileCheck2Icon, title: "Quotation approved", meta: "Vertex Labels · ₹1,12,000" },
    { icon: Trash2Icon, title: "Wastage logged", meta: "JOB-311 · 120 sheets" },
    { icon: SheetIcon, title: "GST report exported", meta: "GSTR-1 · August" },
];

/** Illustrative feed of job-lifecycle events. Static data, no API. */
export function ActivityFeed({ className }) {
    return (
        <div className={cn("tw:relative tw:h-96 tw:overflow-hidden", className)}>
            <AnimatedList>
                {EVENTS.map((event) => (
                    <figure
                        key={event.title}
                        className="tw:flex tw:items-center tw:gap-4 tw:rounded-2xl tw:border tw:border-border tw:bg-card tw:p-4"
                    >
                        <span className="tw:flex tw:size-10 tw:shrink-0 tw:items-center tw:justify-center tw:rounded-xl tw:bg-primary/10 tw:text-primary">
                            <event.icon className="tw:size-5" />
                        </span>
                        <span className="tw:min-w-0">
                            <span className="tw:block tw:truncate tw:text-sm tw:font-medium">{event.title}</span>
                            <span className="tw:block tw:truncate tw:text-xs tw:text-muted-foreground">
                                {event.meta}
                            </span>
                        </span>
                    </figure>
                ))}
            </AnimatedList>
            <div className="tw:pointer-events-none tw:absolute tw:inset-x-0 tw:bottom-0 tw:h-24 tw:bg-gradient-to-t tw:from-background" />
        </div>
    );
}
