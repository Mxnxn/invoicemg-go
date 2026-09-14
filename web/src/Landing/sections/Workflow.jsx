import { CheckIcon } from "lucide-react";

import { BlurFade } from "../velora/blur-fade";
import { IntegrationsBeam } from "../demo/IntegrationsBeam";

const POINTS = [
    "One entry, many documents - invoice, challan and quotation share the same numbers",
    "Company-scoped data, with a different company per browser tab",
    "Every register exports to Excel, and invoices go out over WhatsApp",
];

export function Workflow() {
    return (
        <section id="workflow" className="tw:relative tw:py-24 tw:lg:py-32">
            <div className="tw:mx-auto tw:grid tw:max-w-6xl tw:items-center tw:gap-12 tw:px-4 tw:lg:grid-cols-2 tw:lg:gap-20 tw:lg:px-8">
                <BlurFade direction="right">
                    <div>
                        <span className="tw:text-sm tw:font-medium tw:text-primary">The flow</span>
                        <h2 className="tw:mt-3 tw:text-3xl tw:font-semibold tw:tracking-tight tw:text-balance tw:lg:text-4xl">
                            From client entry to <span className="tw:text-primary">dispatch</span>
                        </h2>
                        <p className="tw:mt-4 tw:text-muted-foreground">
                            Clients, materials and entries flow into one record. Out the other side: a GST invoice, a
                            delivery challan, an Excel export and a WhatsApp message - all from the same numbers.
                        </p>
                        <ul className="tw:mt-6 tw:space-y-3 tw:text-sm">
                            {POINTS.map((point) => (
                                <li key={point} className="tw:flex tw:items-start tw:gap-3">
                                    <span className="tw:mt-0.5 tw:flex tw:size-5 tw:shrink-0 tw:items-center tw:justify-center tw:rounded-full tw:bg-primary/15 tw:text-primary">
                                        <CheckIcon className="tw:size-3" />
                                    </span>
                                    {point}
                                </li>
                            ))}
                        </ul>
                    </div>
                </BlurFade>
                <BlurFade direction="left" delay={0.15}>
                    <IntegrationsBeam />
                </BlurFade>
            </div>
        </section>
    );
}
