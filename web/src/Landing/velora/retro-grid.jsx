// Ported from velora-ui-main/src/components/velora/retro-grid.tsx
import { cn } from "../lib/cn";

/**
 * Scrolling perspective grid backdrop. Place inside a
 * `tw:relative tw:overflow-hidden` section.
 */
export function RetroGrid({ className, angle = 55, cellSize = 56, opacity = 0.4, ...props }) {
    return (
        <div
            aria-hidden
            data-slot="retro-grid"
            className={cn(
                "tw:pointer-events-none tw:absolute tw:inset-0 tw:overflow-hidden tw:[perspective:240px]",
                className
            )}
            style={{ opacity }}
            {...props}
        >
            <div className="tw:absolute tw:inset-0" style={{ transform: `rotateX(${angle}deg)` }}>
                <div
                    className="tw:animate-retro-grid tw:[inset:0%_0px] tw:[margin-left:-200%] tw:[transform-origin:100%_0_0] tw:absolute tw:h-[300vh] tw:w-[600vw] tw:[background-image:linear-gradient(to_right,var(--grid-line)_1px,transparent_0),linear-gradient(to_bottom,var(--grid-line)_1px,transparent_0)] tw:[background-repeat:repeat]"
                    style={{
                        backgroundSize: `${cellSize}px ${cellSize}px`,
                        "--grid-line": "color-mix(in oklab, var(--border) 90%, transparent)",
                    }}
                />
            </div>
            <div className="tw:absolute tw:inset-0 tw:bg-gradient-to-t tw:from-background tw:via-transparent tw:to-background" />
        </div>
    );
}
