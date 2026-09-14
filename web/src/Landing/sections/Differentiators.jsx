import { Building2Icon, MoonIcon, ShieldCheckIcon } from "lucide-react";

import { BlurFade } from "../velora/blur-fade";
import { SpotlightCard } from "../velora/spotlight-card";

// The lead differentiator, then the two supporting ones.
//
// These were three equal thirds, which said the three are equally important. They are not:
// one login holding several firms at once, a tab each, is the reason a shop that keeps two
// sets of books picks this over a general invoicing tool. Roles and dark mode are good
// answers to "does it also do X" - they do not win the decision.
const LEAD = {
    icon: Building2Icon,
    title: "Multi-company, per tab",
    body: "One login, many firms. Each browser tab can act as a different company, so switching books never means signing out.",
    // Reuses the firm names the dashboard mockup and activity feed already invent, so the
    // page keeps one fictional world instead of introducing a second.
    tabs: ["Sharma Packaging", "Vertex Labels"],
    caption: "Two tabs, two sets of books, one login.",
};

const SUPPORTING = [
    {
        icon: ShieldCheckIcon,
        title: "Roles and permissions",
        body: "Admins see everything; employees see exactly the features you grant them, snapshotted at login.",
    },
    {
        icon: MoonIcon,
        title: "Dark mode and a safety net",
        body: "The whole app follows one theme setting, and deleted records land in Trash instead of disappearing.",
    },
];

export function Differentiators() {
    return (
        <section className="tw:relative tw:py-24 tw:lg:py-32">
            <div className="tw:mx-auto tw:max-w-6xl tw:px-4 tw:lg:px-8">
                <div className="tw:grid tw:gap-6 tw:md:grid-cols-5">
                    <BlurFade className="tw:md:col-span-3">
                        <SpotlightCard
                            className="tw:h-full tw:p-8 tw:lg:p-10"
                            contentClassName="tw:flex tw:h-full tw:flex-col"
                        >
                            <div className="tw:mb-5 tw:w-fit tw:rounded-xl tw:bg-primary/10 tw:p-3.5 tw:text-primary">
                                <LEAD.icon className="tw:size-7" />
                            </div>
                            <h3 className="tw:text-xl tw:font-semibold tw:lg:text-2xl">{LEAD.title}</h3>
                            <p className="tw:mt-3 tw:max-w-md tw:text-muted-foreground">{LEAD.body}</p>

                            {/* Decorative: two tab chips showing the feature rather than
                                describing it again. Hidden from screen readers, which get
                                the caption underneath instead. */}
                            <div className="tw:mt-auto tw:pt-8">
                                <div className="tw:flex tw:flex-wrap tw:gap-2" aria-hidden="true">
                                    <span className="tw:rounded-lg tw:border tw:border-primary/40 tw:bg-primary/10 tw:px-3 tw:py-1.5 tw:text-xs tw:font-medium tw:text-primary">
                                        {LEAD.tabs[0]}
                                    </span>
                                    <span className="tw:rounded-lg tw:border tw:border-border tw:px-3 tw:py-1.5 tw:text-xs tw:font-medium tw:text-muted-foreground">
                                        {LEAD.tabs[1]}
                                    </span>
                                </div>
                                <p className="tw:mt-3 tw:text-xs tw:text-muted-foreground">{LEAD.caption}</p>
                            </div>
                        </SpotlightCard>
                    </BlurFade>

                    <div className="tw:grid tw:gap-6 tw:md:col-span-2">
                        {SUPPORTING.map((card, index) => (
                            <BlurFade key={card.title} delay={(index + 1) * 0.12}>
                                <SpotlightCard className="tw:h-full tw:p-8">
                                    <div className="tw:mb-4 tw:w-fit tw:rounded-xl tw:bg-primary/10 tw:p-3 tw:text-primary">
                                        <card.icon className="tw:size-6" />
                                    </div>
                                    <h3 className="tw:text-lg tw:font-semibold">{card.title}</h3>
                                    <p className="tw:mt-2 tw:text-sm tw:text-muted-foreground">{card.body}</p>
                                </SpotlightCard>
                            </BlurFade>
                        ))}
                    </div>
                </div>
            </div>
        </section>
    );
}
