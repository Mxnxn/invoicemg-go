// Ported from velora-ui-main/src/components/velora/orbiting-circles.tsx
import { cn } from "../lib/cn";

/**
 * Evenly distributes children around a circular orbit and rotates them. Place
 * inside a `tw:relative` container; stack multiple instances with different
 * radii for multi-ring layouts.
 */
export function OrbitingCircles({
    className,
    children,
    duration = 20,
    radius = 160,
    reverse = false,
    path = true,
    iconSize = 32,
    speed = 1,
    ...props
}) {
    const items = Array.isArray(children) ? children : [children];
    const calculatedDuration = duration / speed;

    return (
        <>
            {path && (
                <div
                    aria-hidden
                    className="tw:pointer-events-none tw:absolute tw:inset-0 tw:flex tw:items-center tw:justify-center"
                >
                    <div
                        className="tw:rounded-full tw:border tw:border-dashed tw:border-border"
                        style={{ width: radius * 2, height: radius * 2 }}
                    />
                </div>
            )}
            {items.map((child, i) => {
                const angle = (360 / items.length) * i;
                return (
                    <div
                        key={i}
                        data-slot="orbiting-circle"
                        style={{
                            "--duration": calculatedDuration,
                            "--radius": radius,
                            "--angle": angle,
                        }}
                        className={cn(
                            "tw:animate-orbit tw:absolute tw:top-1/2 tw:left-1/2 tw:flex tw:-translate-x-1/2 tw:-translate-y-1/2 tw:transform-gpu tw:items-center tw:justify-center tw:rounded-full",
                            reverse && "tw:[animation-direction:reverse]",
                            className
                        )}
                        {...props}
                    >
                        <div
                            className="tw:flex tw:items-center tw:justify-center"
                            style={{ width: iconSize, height: iconSize }}
                        >
                            {child}
                        </div>
                    </div>
                );
            })}
        </>
    );
}
