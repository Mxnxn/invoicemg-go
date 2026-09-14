// Ported from velora-ui-main/src/components/velora/marquee.tsx
import { cn } from "../lib/cn";

export function Marquee({
    className,
    reverse = false,
    pauseOnHover = false,
    vertical = false,
    repeat = 4,
    fade = true,
    children,
    ...props
}) {
    return (
        <div
            {...props}
            data-slot="marquee"
            className={cn(
                "tw:group tw:flex tw:gap-(--gap) tw:overflow-hidden tw:[--duration:40s] tw:[--gap:1rem]",
                vertical ? "tw:flex-col" : "tw:flex-row",
                fade &&
                    (vertical
                        ? "tw:[mask-image:linear-gradient(to_bottom,transparent,black_12%,black_88%,transparent)]"
                        : "tw:[mask-image:linear-gradient(to_right,transparent,black_12%,black_88%,transparent)]"),
                className
            )}
        >
            {Array.from({ length: repeat }).map((_, i) => (
                <div
                    key={i}
                    aria-hidden={i > 0 || undefined}
                    className={cn(
                        "tw:flex tw:shrink-0 tw:justify-around tw:gap-(--gap)",
                        vertical ? "tw:animate-marquee-vertical tw:flex-col" : "tw:animate-marquee tw:flex-row",
                        reverse && "tw:[animation-direction:reverse]",
                        pauseOnHover && "tw:group-hover:[animation-play-state:paused]"
                    )}
                >
                    {children}
                </div>
            ))}
        </div>
    );
}
