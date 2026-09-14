// Ported from velora-ui-main/src/components/velora/spotlight-card.tsx
import { useRef } from "react";

import { cn } from "../lib/cn";

export function SpotlightCard({
    children,
    className,
    // The card wraps its children in a positioned layer, so a flex/height class on
    // `className` stops at that wrapper and never reaches the content. This forwards
    // onto the wrapper itself, which is what a card needing a bottom-anchored footer
    // has to reach.
    contentClassName,
    radius = 320,
    color = "color-mix(in oklab, var(--brand) 14%, transparent)",
    ...props
}) {
    const ref = useRef(null);

    const handleMouseMove = (e) => {
        const el = ref.current;
        if (!el) return;
        const rect = el.getBoundingClientRect();
        el.style.setProperty("--spot-x", `${e.clientX - rect.left}px`);
        el.style.setProperty("--spot-y", `${e.clientY - rect.top}px`);
    };

    return (
        <div
            ref={ref}
            data-slot="spotlight-card"
            onMouseMove={handleMouseMove}
            className={cn(
                "tw:group tw:relative tw:overflow-hidden tw:rounded-2xl tw:border tw:border-border tw:bg-card",
                className
            )}
            style={{
                "--spot-radius": `${radius}px`,
                "--spot-color": color,
            }}
            {...props}
        >
            <div
                aria-hidden
                className="tw:pointer-events-none tw:absolute tw:inset-0 tw:opacity-0 tw:transition-opacity tw:duration-300 tw:group-hover:opacity-100"
                style={{
                    background:
                        "radial-gradient(var(--spot-radius) circle at var(--spot-x, 50%) var(--spot-y, 50%), var(--spot-color), transparent 65%)",
                }}
            />
            <div className={cn("tw:relative tw:z-10", contentClassName)}>{children}</div>
        </div>
    );
}
