import { BlurFade } from "../velora/blur-fade";
import { NumberTicker } from "../velora/number-ticker";

// Placeholder figures - replace with real numbers before launch.
const STATS = [
    { value: 50000, prefix: "", suffix: "+", label: "Invoices raised" },
    { value: 120, prefix: "", suffix: "+", label: "Active clients" },
    { value: 8, prefix: "", suffix: "", label: "Lifecycle stages tracked" },
    { value: 4, prefix: "", suffix: "", label: "Document types" },
];

export function Stats() {
    return (
        <section className="tw:border-y tw:border-border tw:py-14">
            <div className="tw:mx-auto tw:grid tw:max-w-4xl tw:grid-cols-2 tw:gap-8 tw:px-4 tw:lg:grid-cols-4 tw:lg:px-8">
                {STATS.map((stat, index) => (
                    <BlurFade key={stat.label} delay={index * 0.1}>
                        <div data-stat className="tw:flex tw:flex-col tw:items-center tw:gap-1">
                            <span className="tw:text-4xl tw:font-semibold tw:tracking-tight">
                                <NumberTicker value={stat.value} prefix={stat.prefix} suffix={stat.suffix} />
                            </span>
                            <span className="tw:text-center tw:text-sm tw:text-muted-foreground">{stat.label}</span>
                        </div>
                    </BlurFade>
                ))}
            </div>
        </section>
    );
}
