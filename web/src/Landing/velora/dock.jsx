// Ported from velora-ui-main/src/components/velora/dock.tsx
import { createContext, useContext, useRef } from "react";
import { motion, useMotionValue, useReducedMotion, useSpring, useTransform } from "motion/react";

import { cn } from "../lib/cn";

const DockContext = createContext(null);

export function Dock({ children, className, baseSize = 40, magnification = 64, distance = 140 }) {
    const mouseX = useMotionValue(Infinity);

    return (
        <DockContext.Provider value={{ mouseX, baseSize, magnification, distance }}>
            <motion.div
                data-slot="dock"
                onMouseMove={(e) => mouseX.set(e.clientX)}
                onMouseLeave={() => mouseX.set(Infinity)}
                className={cn(
                    "tw:mx-auto tw:flex tw:h-16 tw:w-fit tw:items-end tw:gap-2 tw:rounded-2xl tw:border tw:border-border tw:bg-card/70 tw:px-3 tw:pb-2 tw:backdrop-blur-xl",
                    className
                )}
            >
                {children}
            </motion.div>
        </DockContext.Provider>
    );
}

export function DockIcon({ children, className, label }) {
    const ref = useRef(null);
    const reducedMotion = useReducedMotion();
    const fallbackX = useMotionValue(Infinity);
    const ctx = useContext(DockContext);
    const mouseX = ctx?.mouseX ?? fallbackX;
    const baseSize = ctx?.baseSize ?? 40;
    const magnification = ctx?.magnification ?? 64;
    const distance = ctx?.distance ?? 140;

    const distanceFromCursor = useTransform(mouseX, (x) => {
        const bounds = ref.current?.getBoundingClientRect() ?? { x: 0, width: 0 };
        return x - bounds.x - bounds.width / 2;
    });

    const sizeTarget = useTransform(distanceFromCursor, [-distance, 0, distance], [baseSize, magnification, baseSize]);
    const size = useSpring(sizeTarget, {
        mass: 0.1,
        stiffness: 160,
        damping: 13,
    });

    return (
        <motion.div
            ref={ref}
            data-slot="dock-icon"
            aria-label={label}
            style={reducedMotion ? { width: baseSize, height: baseSize } : { width: size, height: size }}
            className={cn(
                "tw:flex tw:aspect-square tw:cursor-pointer tw:items-center tw:justify-center tw:rounded-full tw:bg-muted tw:text-muted-foreground tw:transition-colors tw:hover:text-foreground",
                className
            )}
        >
            {children}
        </motion.div>
    );
}
