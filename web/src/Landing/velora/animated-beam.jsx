// Ported from velora-ui-main/src/components/velora/animated-beam.tsx
import { useEffect, useId, useState } from "react";
import { motion } from "motion/react";

import { cn } from "../lib/cn";

/**
 * Animated gradient beam connecting two elements inside a shared `tw:relative`
 * container. Pass refs to the container and both endpoints.
 */
export function AnimatedBeam({
    containerRef,
    fromRef,
    toRef,
    curvature = 0,
    reverse = false,
    duration = 5,
    delay = 0,
    pathWidth = 2,
    pathOpacity = 0.15,
    // Themed rather than the template's hardcoded violet/cyan, so the beam
    // follows the admin panel's brand colour in both themes.
    gradientStartColor = "var(--brand-from)",
    gradientStopColor = "var(--brand-to)",
    className,
}) {
    const id = useId();
    const [pathD, setPathD] = useState("");
    const [size, setSize] = useState({ width: 0, height: 0 });

    const gradientCoordinates = reverse
        ? { x1: ["90%", "-10%"], x2: ["100%", "0%"] }
        : { x1: ["10%", "110%"], x2: ["0%", "100%"] };

    useEffect(() => {
        const update = () => {
            const container = containerRef.current;
            const from = fromRef.current;
            const to = toRef.current;
            if (!container || !from || !to) return;

            const containerRect = container.getBoundingClientRect();
            const rectA = from.getBoundingClientRect();
            const rectB = to.getBoundingClientRect();

            const startX = rectA.left - containerRect.left + rectA.width / 2;
            const startY = rectA.top - containerRect.top + rectA.height / 2;
            const endX = rectB.left - containerRect.left + rectB.width / 2;
            const endY = rectB.top - containerRect.top + rectB.height / 2;

            setSize({ width: containerRect.width, height: containerRect.height });
            setPathD(
                `M ${startX},${startY} Q ${(startX + endX) / 2},${(startY + endY) / 2 - curvature} ${endX},${endY}`
            );
        };

        // ResizeObserver fires once on observe, which also draws the initial path
        const observer = new ResizeObserver(update);
        if (containerRef.current) observer.observe(containerRef.current);
        return () => observer.disconnect();
    }, [containerRef, fromRef, toRef, curvature]);

    return (
        <svg
            aria-hidden
            data-slot="animated-beam"
            fill="none"
            width={size.width}
            height={size.height}
            viewBox={`0 0 ${size.width} ${size.height}`}
            className={cn("tw:pointer-events-none tw:absolute tw:top-0 tw:left-0 tw:transform-gpu", className)}
        >
            <path
                d={pathD}
                stroke="currentColor"
                strokeWidth={pathWidth}
                strokeOpacity={pathOpacity}
                strokeLinecap="round"
                className="tw:text-border"
            />
            <path d={pathD} stroke={`url(#${id})`} strokeWidth={pathWidth} strokeLinecap="round" />
            <defs>
                <motion.linearGradient
                    id={id}
                    gradientUnits="userSpaceOnUse"
                    initial={{ x1: "0%", x2: "0%", y1: "0%", y2: "0%" }}
                    animate={{
                        x1: gradientCoordinates.x1,
                        x2: gradientCoordinates.x2,
                        y1: ["0%", "0%"],
                        y2: ["0%", "0%"],
                    }}
                    transition={{
                        delay,
                        duration,
                        ease: [0.16, 1, 0.3, 1],
                        repeat: Infinity,
                    }}
                >
                    <stop stopColor={gradientStartColor} stopOpacity="0" />
                    <stop stopColor={gradientStartColor} />
                    <stop offset="32.5%" stopColor={gradientStopColor} />
                    <stop offset="100%" stopColor={gradientStopColor} stopOpacity="0" />
                </motion.linearGradient>
            </defs>
        </svg>
    );
}
