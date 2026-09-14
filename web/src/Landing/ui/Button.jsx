import { cn } from "../lib/cn";

const VARIANTS = {
    default: "tw:bg-primary tw:text-primary-foreground tw:hover:opacity-90",
    outline: "tw:border tw:border-border tw:bg-transparent tw:hover:bg-muted",
    ghost: "tw:bg-transparent tw:hover:bg-muted",
};

const SIZES = {
    default: "tw:h-10 tw:px-5 tw:text-sm",
    sm: "tw:h-8 tw:px-3 tw:text-xs",
    lg: "tw:h-12 tw:px-7 tw:text-base",
    icon: "tw:size-9",
};

/**
 * Minimal stand-in for shadcn's Button. Deliberately has no `asChild` / Slot
 * support so the landing page needs no radix dependency - wrap it in a Link or
 * render an <a> directly where a link is wanted.
 */
export function Button({ variant = "default", size = "default", className, children, ...rest }) {
    return (
        <button
            className={cn(
                "tw:inline-flex tw:items-center tw:justify-center tw:gap-2 tw:rounded-full tw:font-medium tw:transition-colors tw:[&_svg]:size-4",
                VARIANTS[variant],
                SIZES[size],
                className
            )}
            {...rest}
        >
            {children}
        </button>
    );
}
