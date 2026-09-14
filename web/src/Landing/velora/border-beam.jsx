// Ported from velora-ui-main/src/components/velora/border-beam.tsx
import { motion, useReducedMotion } from "motion/react";

import { cn } from "../lib/cn";

/**
 * Animated beam that travels around the border of its nearest positioned
 * ancestor. Parent needs `position: relative` and a border.
 */
export function BorderBeam({
    className,
    size = 64,
    duration = 6,
    delay = 0,
    reverse = false,
    colorFrom = "var(--brand-from)",
    colorTo = "var(--brand-to)",
}) {
    // Hide via CSS and skip the animation under reduced motion rather than
    // branching the rendered tree on it.
    const reducedMotion = useReducedMotion();

    return (
        <div
            aria-hidden
            data-slot="border-beam"
            className="tw:pointer-events-none tw:absolute tw:inset-0 tw:rounded-[inherit] tw:border tw:border-transparent tw:[mask-clip:padding-box,border-box] tw:[mask-composite:intersect] tw:[mask-image:linear-gradient(transparent,transparent),linear-gradient(#000,#000)] tw:motion-reduce:hidden"
        >
            <motion.div
                className={cn(
                    "tw:absolute tw:aspect-square tw:bg-gradient-to-l tw:from-(--beam-from) tw:via-(--beam-to) tw:to-transparent",
                    className
                )}
                style={{
                    width: size,
                    offsetPath: `rect(0 auto auto 0 round ${size}px)`,
                    "--beam-from": colorFrom,
                    "--beam-to": colorTo,
                }}
                initial={{ offsetDistance: reverse ? "100%" : "0%" }}
                animate={reducedMotion ? undefined : { offsetDistance: reverse ? "0%" : "100%" }}
                transition={{
                    repeat: Infinity,
                    ease: "linear",
                    duration,
                    delay: -delay,
                }}
            />
        </div>
    );
}
