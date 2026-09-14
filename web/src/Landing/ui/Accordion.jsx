import { useState } from "react";
import { ChevronDownIcon } from "lucide-react";

import { cn } from "../lib/cn";

/**
 * Single-open, collapsible accordion. Replaces radix's Accordion - the FAQ is
 * the only consumer and needs none of radix's animation or type machinery.
 */
export function Accordion({ items, className }) {
    const [open, setOpen] = useState(null);

    return (
        <div className={cn("tw:divide-y tw:divide-border tw:border-y tw:border-border", className)}>
            {items.map((item, index) => {
                const isOpen = open === index;
                return (
                    <div key={item.q}>
                        <button
                            type="button"
                            aria-expanded={isOpen}
                            onClick={() => setOpen(isOpen ? null : index)}
                            className="tw:flex tw:w-full tw:items-center tw:justify-between tw:gap-4 tw:py-5 tw:text-left tw:text-base tw:font-medium"
                        >
                            {item.q}
                            <ChevronDownIcon
                                className={cn(
                                    "tw:size-4 tw:shrink-0 tw:transition-transform",
                                    isOpen && "tw:rotate-180"
                                )}
                            />
                        </button>
                        {isOpen && (
                            <div role="region" className="tw:pb-5 tw:text-sm tw:text-muted-foreground">
                                {item.a}
                            </div>
                        )}
                    </div>
                );
            })}
        </div>
    );
}
