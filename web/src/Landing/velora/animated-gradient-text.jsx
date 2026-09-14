// Ported from velora-ui-main/src/components/velora/animated-gradient-text.tsx
import { cn } from "../lib/cn";

export function AnimatedGradientText({ className, children, ...props }) {
    return (
        <span
            data-slot="animated-gradient-text"
            className={cn(
                "tw:animate-gradient tw:inline-block tw:bg-gradient-to-r tw:from-brand-from tw:via-brand-via tw:to-brand-to tw:bg-[length:300%_auto] tw:bg-clip-text tw:text-transparent",
                className
            )}
            {...props}
        >
            {children}
        </span>
    );
}
