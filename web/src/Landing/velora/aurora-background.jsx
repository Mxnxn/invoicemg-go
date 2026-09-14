// Ported from velora-ui-main/src/components/velora/aurora-background.tsx
import { cn } from "../lib/cn";

const intensityClass = {
    subtle: "tw:opacity-25 tw:dark:opacity-20",
    medium: "tw:opacity-40 tw:dark:opacity-30",
    vivid: "tw:opacity-60 tw:dark:opacity-45",
};

/**
 * Velora's signature animated aurora backdrop. Absolutely positioned - place
 * inside a `tw:relative tw:overflow-hidden` section.
 */
export function AuroraBackground({ className, intensity = "medium", ...props }) {
    return (
        <div
            aria-hidden
            data-slot="aurora-background"
            className={cn(
                "tw:pointer-events-none tw:absolute tw:inset-0 tw:overflow-hidden",
                intensityClass[intensity],
                className
            )}
            {...props}
        >
            <div className="tw:animate-aurora-1 tw:absolute tw:-top-1/4 tw:left-[10%] tw:size-[44rem] tw:rounded-full tw:bg-brand-from tw:blur-[120px] tw:will-change-transform" />
            <div className="tw:animate-aurora-2 tw:absolute tw:top-[5%] tw:right-[5%] tw:size-[38rem] tw:rounded-full tw:bg-brand-via tw:blur-[130px] tw:will-change-transform" />
            <div className="tw:animate-aurora-3 tw:absolute tw:-bottom-1/4 tw:left-[35%] tw:size-[40rem] tw:rounded-full tw:bg-brand-to tw:blur-[140px] tw:will-change-transform" />
        </div>
    );
}
