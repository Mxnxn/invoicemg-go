// Ported from velora-ui-main/src/components/velora/scroll-progress.tsx
import { motion, useReducedMotion, useScroll, useSpring } from "motion/react";

import { cn } from "../lib/cn";

export function ScrollProgress({ className }) {
    const { scrollYProgress } = useScroll();
    const scaleX = useSpring(scrollYProgress, {
        stiffness: 180,
        damping: 32,
        restDelta: 0.001,
    });
    const reducedMotion = useReducedMotion();

    return (
        <motion.div
            aria-hidden
            data-slot="scroll-progress"
            className={cn(
                "tw:fixed tw:inset-x-0 tw:top-0 tw:z-[60] tw:h-0.75 tw:origin-left tw:bg-gradient-to-r tw:from-brand-from tw:via-brand-via tw:to-brand-to",
                className
            )}
            style={{ scaleX: reducedMotion ? scrollYProgress : scaleX }}
        />
    );
}
