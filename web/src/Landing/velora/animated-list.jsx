// Ported from velora-ui-main/src/components/velora/animated-list.tsx
import React, { useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion } from "motion/react";

import { cn } from "../lib/cn";

/**
 * Reveals children one by one, newest on top, then loops. Wrap in a
 * fixed-height container with a bottom mask for a feed effect.
 */
export function AnimatedList({ children, className, delay = 2000 }) {
    const [index, setIndex] = useState(0);
    const items = useMemo(() => React.Children.toArray(children), [children]);

    useEffect(() => {
        const interval = setInterval(() => setIndex((i) => (i + 1) % items.length), delay);
        return () => clearInterval(interval);
    }, [items.length, delay]);

    const visible = useMemo(() => items.slice(0, index + 1).reverse(), [items, index]);

    return (
        <div data-slot="animated-list" className={cn("tw:flex tw:flex-col tw:gap-3", className)}>
            <AnimatePresence>
                {visible.map((item) => (
                    <motion.div
                        key={item.key}
                        layout
                        initial={{ scale: 0, opacity: 0 }}
                        animate={{ scale: 1, opacity: 1, originY: 0 }}
                        exit={{ scale: 0, opacity: 0 }}
                        transition={{ type: "spring", stiffness: 350, damping: 40 }}
                        className="tw:w-full"
                    >
                        {item}
                    </motion.div>
                ))}
            </AnimatePresence>
        </div>
    );
}
