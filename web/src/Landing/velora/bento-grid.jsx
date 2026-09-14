// Ported from velora-ui-main/src/components/velora/bento-grid.tsx
import { forwardRef } from "react";
import { ArrowRightIcon } from "lucide-react";

import { cn } from "../lib/cn";

export const BentoGrid = forwardRef(function BentoGrid({ className, children, ...props }, ref) {
    return (
        <div
            ref={ref}
            data-slot="bento-grid"
            className={cn("tw:grid tw:w-full tw:auto-rows-[22rem] tw:grid-cols-1 tw:gap-4 tw:md:grid-cols-3", className)}
            {...props}
        >
            {children}
        </div>
    );
});

export function BentoCard({ name, description, background, icon, href, cta = "Learn more", className, ...props }) {
    return (
        <div
            data-slot="bento-card"
            className={cn(
                "tw:group tw:relative tw:flex tw:flex-col tw:justify-end tw:overflow-hidden tw:rounded-2xl tw:border tw:border-border tw:bg-card tw:transition-shadow tw:duration-300 tw:hover:shadow-xl tw:hover:shadow-primary/5",
                className
            )}
            {...props}
        >
            {background && (
                <div className="tw:absolute tw:inset-0 tw:overflow-hidden tw:transition-transform tw:duration-500 tw:ease-out tw:group-hover:scale-[1.04] tw:motion-reduce:transition-none">
                    {background}
                </div>
            )}
            <div
                aria-hidden
                className="tw:pointer-events-none tw:absolute tw:inset-0 tw:bg-gradient-to-t tw:from-card tw:via-card/60 tw:to-transparent"
            />
            <div
                className={cn(
                    "tw:pointer-events-none tw:relative tw:z-10 tw:flex tw:flex-col tw:gap-1 tw:p-6 tw:transition-transform tw:duration-300 tw:ease-out tw:motion-reduce:transition-none",
                    href && "tw:group-hover:-translate-y-7"
                )}
            >
                {icon && <div className="tw:mb-2 tw:w-fit tw:text-primary tw:[&_svg]:size-8">{icon}</div>}
                <h3 className="tw:text-lg tw:font-semibold tw:text-card-foreground">{name}</h3>
                <p className="tw:text-sm tw:text-muted-foreground">{description}</p>
            </div>
            {href && (
                <div className="tw:absolute tw:inset-x-0 tw:bottom-0 tw:z-10 tw:translate-y-full tw:p-6 tw:pt-0 tw:transition-transform tw:duration-300 tw:ease-out tw:group-hover:translate-y-0 tw:motion-reduce:translate-y-0">
                    <a
                        href={href}
                        className="tw:inline-flex tw:items-center tw:gap-1 tw:text-sm tw:font-medium tw:text-primary tw:hover:underline"
                    >
                        {cta}
                        <ArrowRightIcon className="tw:size-4 tw:transition-transform tw:group-hover:translate-x-0.5" />
                    </a>
                </div>
            )}
        </div>
    );
}
