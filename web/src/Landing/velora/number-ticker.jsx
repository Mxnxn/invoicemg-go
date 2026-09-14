// Ported from velora-ui-main/src/components/velora/number-ticker.tsx
import { useEffect, useRef } from "react";
import { useInView, useMotionValue, useReducedMotion, useSpring } from "motion/react";

import { cn } from "../lib/cn";

export function NumberTicker({
    value,
    startValue = 0,
    delay = 0,
    decimalPlaces = 0,
    prefix = "",
    suffix = "",
    className,
    ...props
}) {
    const ref = useRef(null);
    const motionValue = useMotionValue(startValue);
    const springValue = useSpring(motionValue, {
        damping: 60,
        stiffness: 100,
    });
    const isInView = useInView(ref, { once: true, margin: "0px 0px -10% 0px" });
    const reducedMotion = useReducedMotion();

    const format = (n) =>
        `${prefix}${Intl.NumberFormat("en-US", {
            minimumFractionDigits: decimalPlaces,
            maximumFractionDigits: decimalPlaces,
        }).format(n)}${suffix}`;

    useEffect(() => {
        if (!isInView) return;
        if (reducedMotion) {
            motionValue.jump(value);
            return;
        }
        const timeout = setTimeout(() => motionValue.set(value), delay * 1000);
        return () => clearTimeout(timeout);
    }, [isInView, reducedMotion, motionValue, value, delay]);

    useEffect(
        () =>
            springValue.on("change", (latest) => {
                if (ref.current) ref.current.textContent = format(latest);
            }),
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [springValue, prefix, suffix, decimalPlaces]
    );

    return (
        <span
            ref={ref}
            data-slot="number-ticker"
            className={cn("tw:inline-block tw:tabular-nums", className)}
            {...props}
        >
            {format(startValue)}
        </span>
    );
}
