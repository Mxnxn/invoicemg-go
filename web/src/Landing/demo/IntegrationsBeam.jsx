import { forwardRef, useRef } from "react";
import {
    FileSpreadsheetIcon,
    FileTextIcon,
    LayersIcon,
    MessageCircleIcon,
    PackageIcon,
    ReceiptTextIcon,
    UsersIcon,
} from "lucide-react";

import { cn } from "../lib/cn";
import { AnimatedBeam } from "../velora/animated-beam";

// Ported from velora-ui-main/src/components/demo/integrations-beam.tsx -
// React 19's ref-as-prop is unavailable on React 18, hence forwardRef.
const Node = forwardRef(function Node({ className, label, children }, ref) {
    return (
        <div className="tw:flex tw:flex-col tw:items-center tw:gap-2">
            <div
                ref={ref}
                className={cn(
                    "tw:z-10 tw:flex tw:size-12 tw:items-center tw:justify-center tw:rounded-full tw:border tw:border-border tw:bg-card tw:shadow-lg tw:[&_svg]:size-5",
                    className
                )}
            >
                {children}
            </div>
            <span className="tw:text-xs tw:text-muted-foreground">{label}</span>
        </div>
    );
});

/** Client entries flow in; documents and exports flow out. */
export function IntegrationsBeam({ className }) {
    const containerRef = useRef(null);
    const centerRef = useRef(null);
    const left1 = useRef(null);
    const left2 = useRef(null);
    const left3 = useRef(null);
    const right1 = useRef(null);
    const right2 = useRef(null);
    const right3 = useRef(null);

    return (
        <div
            ref={containerRef}
            className={cn(
                "tw:relative tw:flex tw:h-96 tw:w-full tw:items-center tw:justify-between tw:px-2 tw:sm:px-8",
                className
            )}
        >
            <div className="tw:flex tw:h-full tw:flex-col tw:justify-between tw:py-6">
                <Node ref={left1} label="Clients">
                    <UsersIcon className="tw:text-muted-foreground" />
                </Node>
                <Node ref={left2} label="Materials">
                    <PackageIcon className="tw:text-muted-foreground" />
                </Node>
                <Node ref={left3} label="Entries">
                    <LayersIcon className="tw:text-muted-foreground" />
                </Node>
            </div>

            <Node
                ref={centerRef}
                label="InvoiceMG"
                className="tw:size-16 tw:border-primary/40 tw:bg-primary/10 tw:[&_svg]:size-7"
            >
                <ReceiptTextIcon className="tw:text-primary" />
            </Node>

            <div className="tw:flex tw:h-full tw:flex-col tw:justify-between tw:py-6">
                <Node ref={right1} label="Invoice">
                    <FileTextIcon className="tw:text-muted-foreground" />
                </Node>
                <Node ref={right2} label="Excel">
                    <FileSpreadsheetIcon className="tw:text-muted-foreground" />
                </Node>
                <Node ref={right3} label="WhatsApp">
                    <MessageCircleIcon className="tw:text-muted-foreground" />
                </Node>
            </div>

            <AnimatedBeam containerRef={containerRef} fromRef={left1} toRef={centerRef} curvature={-60} />
            <AnimatedBeam containerRef={containerRef} fromRef={left2} toRef={centerRef} delay={1} />
            <AnimatedBeam containerRef={containerRef} fromRef={left3} toRef={centerRef} curvature={60} delay={2} />
            <AnimatedBeam
                containerRef={containerRef}
                fromRef={right1}
                toRef={centerRef}
                curvature={-60}
                reverse
                delay={0.5}
            />
            <AnimatedBeam containerRef={containerRef} fromRef={right2} toRef={centerRef} reverse delay={1.5} />
            <AnimatedBeam
                containerRef={containerRef}
                fromRef={right3}
                toRef={centerRef}
                curvature={60}
                reverse
                delay={2.5}
            />
        </div>
    );
}
