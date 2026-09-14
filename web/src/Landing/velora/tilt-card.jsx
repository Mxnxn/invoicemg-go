// Ported from velora-ui-main/src/components/velora/tilt-card.tsx
import { useRef } from "react";
import { motion, useMotionValue, useReducedMotion, useSpring } from "motion/react";

import { cn } from "../lib/cn";

/** 3D perspective tilt that follows the cursor. */
export function TiltCard({ children, className, maxTilt = 8 }) {
    const ref = useRef(null);
    const reducedMotion = useReducedMotion();

    const rotateX = useMotionValue(0);
    const rotateY = useMotionValue(0);
    const springX = useSpring(rotateX, { stiffness: 220, damping: 18 });
    const springY = useSpring(rotateY, { stiffness: 220, damping: 18 });

    const handleMouseMove = (e) => {
        if (reducedMotion) return;
        const rect = ref.current?.getBoundingClientRect();
        if (!rect) return;
        const px = (e.clientX - rect.left) / rect.width - 0.5;
        const py = (e.clientY - rect.top) / rect.height - 0.5;
        rotateY.set(px * maxTilt * 2);
        rotateX.set(-py * maxTilt * 2);
    };

    const handleMouseLeave = () => {
        rotateX.set(0);
        rotateY.set(0);
    };

    return (
        <motion.div
            ref={ref}
            data-slot="tilt-card"
            onMouseMove={handleMouseMove}
            onMouseLeave={handleMouseLeave}
            style={{
                rotateX: springX,
                rotateY: springY,
                transformPerspective: 800,
            }}
            className={cn("tw:will-change-transform", className)}
        >
            {children}
        </motion.div>
    );
}
