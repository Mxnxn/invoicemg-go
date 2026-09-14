// Ported from velora-ui-main/src/components/velora/meteors.tsx
import { cn } from "../lib/cn";

/**
 * Falling meteor streaks. Values are derived from the index (not random) so
 * every render produces the same markup.
 */
export function Meteors({ number = 12, className }) {
    const meteors = Array.from({ length: number }, (_, i) => ({
        left: `${(i * 37 + 11) % 100}%`,
        top: `${(i * 17) % 40}%`,
        delay: `${((i * 53) % 70) / 10}s`,
        duration: `${4 + ((i * 29) % 35) / 10}s`,
    }));

    return (
        <div aria-hidden data-slot="meteors" className="tw:pointer-events-none tw:absolute tw:inset-0 tw:overflow-hidden">
            {meteors.map((meteor, i) => (
                <span
                    key={i}
                    className={cn(
                        "tw:animate-meteor tw:absolute tw:size-0.5 tw:rounded-full tw:bg-foreground/70 tw:shadow-[0_0_0_1px_rgba(255,255,255,0.08)]",
                        "tw:before:absolute tw:before:top-1/2 tw:before:h-px tw:before:w-[70px] tw:before:-translate-y-1/2 tw:before:bg-gradient-to-r tw:before:from-foreground/60 tw:before:to-transparent",
                        className
                    )}
                    style={{
                        left: meteor.left,
                        top: meteor.top,
                        animationDelay: meteor.delay,
                        animationDuration: meteor.duration,
                    }}
                />
            ))}
        </div>
    );
}
