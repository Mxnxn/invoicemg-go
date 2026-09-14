// Ported from velora-ui-main/src/components/velora/shimmer-button.tsx
import { cn } from "../lib/cn";

export function ShimmerButton({ className, children, ...props }) {
    return (
        <button
            data-slot="shimmer-button"
            className={cn(
                "tw:group tw:relative tw:inline-flex tw:h-12 tw:cursor-pointer tw:items-center tw:justify-center tw:gap-2 tw:overflow-hidden tw:rounded-full tw:bg-primary tw:px-8 tw:text-sm tw:font-medium tw:text-primary-foreground tw:shadow-lg tw:shadow-primary/30 tw:transition-[transform,box-shadow] tw:duration-300 tw:hover:scale-[1.03] tw:hover:shadow-xl tw:hover:shadow-primary/40 tw:focus-visible:outline-2 tw:focus-visible:outline-offset-2 tw:focus-visible:outline-ring tw:active:scale-[0.98]",
                className
            )}
            {...props}
        >
            <span className="tw:relative tw:z-10 tw:inline-flex tw:items-center tw:gap-2">{children}</span>
            <span
                aria-hidden
                className="tw:animate-shimmer tw:absolute tw:inset-0 tw:bg-[linear-gradient(110deg,transparent_30%,rgba(255,255,255,0.35)_50%,transparent_70%)] tw:bg-[length:250%_100%]"
            />
        </button>
    );
}
