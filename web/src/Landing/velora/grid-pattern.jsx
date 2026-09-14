// Ported from velora-ui-main/src/components/velora/grid-pattern.tsx
import { useId } from "react";

import { cn } from "../lib/cn";

export function GridPattern({
    width = 40,
    height = 40,
    x = -1,
    y = -1,
    strokeDasharray = "0",
    squares,
    className,
    ...props
}) {
    const id = useId();

    return (
        <svg
            aria-hidden
            data-slot="grid-pattern"
            className={cn(
                "tw:pointer-events-none tw:absolute tw:inset-0 tw:size-full tw:fill-muted/40 tw:stroke-border",
                className
            )}
            {...props}
        >
            <defs>
                <pattern id={id} width={width} height={height} patternUnits="userSpaceOnUse" x={x} y={y}>
                    <path d={`M.5 ${height}V.5H${width}`} fill="none" strokeDasharray={strokeDasharray} />
                </pattern>
            </defs>
            <rect width="100%" height="100%" strokeWidth={0} fill={`url(#${id})`} />
            {squares && (
                <svg x={x} y={y} className="tw:overflow-visible">
                    {squares.map(([sx, sy]) => (
                        <rect
                            key={`${sx}-${sy}`}
                            strokeWidth="0"
                            width={width - 1}
                            height={height - 1}
                            x={sx * width + 1}
                            y={sy * height + 1}
                        />
                    ))}
                </svg>
            )}
        </svg>
    );
}

export function DotPattern({ width = 20, height = 20, cx = 1, cy = 1, cr = 1, className, ...props }) {
    const id = useId();

    return (
        <svg
            aria-hidden
            data-slot="dot-pattern"
            className={cn(
                "tw:pointer-events-none tw:absolute tw:inset-0 tw:size-full tw:fill-muted-foreground/30",
                className
            )}
            {...props}
        >
            <defs>
                <pattern id={id} width={width} height={height} patternUnits="userSpaceOnUse">
                    <circle cx={cx} cy={cy} r={cr} />
                </pattern>
            </defs>
            <rect width="100%" height="100%" strokeWidth={0} fill={`url(#${id})`} />
        </svg>
    );
}
