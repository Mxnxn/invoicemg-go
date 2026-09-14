// Ported from velora-ui-main/src/components/velora/blur-fade.tsx
import { motion, useReducedMotion } from "motion/react";

import { cn } from "../lib/cn";

const axis = { up: "y", down: "y", left: "x", right: "x" };
const sign = { up: 1, down: -1, left: 1, right: -1 };

export function BlurFade({
    children,
    className,
    delay = 0,
    duration = 0.5,
    direction = "up",
    offset = 16,
    once = true,
}) {
    const reducedMotion = useReducedMotion();

    const hidden =
        direction === "none"
            ? { opacity: 0, filter: "blur(6px)" }
            : {
                  opacity: 0,
                  filter: "blur(6px)",
                  [axis[direction]]: sign[direction] * offset,
              };

    // Under reduced motion the reveal still happens, just instantly.
    return (
        <motion.div
            data-slot="blur-fade"
            className={cn(className)}
            initial={hidden}
            whileInView={{ opacity: 1, filter: "blur(0px)", x: 0, y: 0 }}
            viewport={{ once, margin: "0px 0px -10% 0px" }}
            transition={reducedMotion ? { duration: 0 } : { delay, duration, ease: [0.21, 0.47, 0.32, 0.98] }}
        >
            {children}
        </motion.div>
    );
}
