import { BarChart3Icon, ZapIcon } from "lucide-react";

import { BlurFade } from "../velora/blur-fade";
import { BentoCard, BentoGrid } from "../velora/bento-grid";
import { BorderBeam } from "../velora/border-beam";
import { DotPattern } from "../velora/grid-pattern";
import { Marquee } from "../velora/marquee";
import { Meteors } from "../velora/meteors";
import { OrbitingCircles } from "../velora/orbiting-circles";

const STAGES = ["Received", "Design", "Plate", "Printing", "Lamination", "Cutting", "Packing", "Dispatched"];

export function Features() {
    return (
        <section id="features" className="tw:relative tw:py-24 tw:lg:py-32">
            <div className="tw:mx-auto tw:max-w-6xl tw:px-4 tw:lg:px-8">
                <BlurFade>
                    <h2 className="tw:mx-auto tw:max-w-2xl tw:text-center tw:text-3xl tw:font-semibold tw:tracking-tight tw:text-balance tw:lg:text-5xl">
                        One system for <span className="tw:text-primary">the whole job</span>
                    </h2>
                    <p className="tw:mx-auto tw:mt-4 tw:max-w-xl tw:text-center tw:text-muted-foreground">
                        Documents, floor tracking, books and reporting - all reading from the same entries, so nothing
                        gets typed twice.
                    </p>
                </BlurFade>

                <BlurFade delay={0.15}>
                    <BentoGrid className="tw:mt-16">
                        <BentoCard
                            name="Invoices & challans"
                            description="Raise GST invoices, delivery challans and quotations off the same entries - numbering, round-off and terms handled."
                            className="tw:md:col-span-2"
                            background={
                                <div className="tw:absolute tw:inset-6 tw:rounded-xl tw:border tw:border-border tw:bg-card/50">
                                    <BorderBeam size={72} duration={7} />
                                </div>
                            }
                        />
                        <BentoCard
                            name="Job lifecycle queue"
                            description="Every job from received to dispatched, with stage history and notes."
                            className="tw:md:col-span-1"
                            background={
                                <div className="tw:absolute tw:inset-x-6 tw:top-4 tw:bottom-24">
                                    <Marquee vertical pauseOnHover className="tw:h-full tw:[--duration:24s]">
                                        {STAGES.map((stage) => (
                                            <div
                                                key={stage}
                                                className="tw:rounded-xl tw:border tw:border-border tw:bg-card/80 tw:p-3 tw:text-sm tw:text-muted-foreground"
                                            >
                                                {stage}
                                            </div>
                                        ))}
                                    </Marquee>
                                </div>
                            }
                        />
                        <BentoCard
                            name="Ledger & GST reports"
                            description="Party-wise ledgers, purchase and sales registers, GST summaries - exported to Excel in a click."
                            className="tw:md:col-span-1"
                            background={
                                <div className="tw:absolute tw:inset-0">
                                    <DotPattern className="tw:[mask-image:radial-gradient(ellipse_at_center,black,transparent_75%)]" />
                                    <Meteors number={10} />
                                </div>
                            }
                        />
                        <BentoCard
                            name="Analytics"
                            description="Weekly, monthly and yearly buckets across sales, materials and wastage."
                            className="tw:md:col-span-2"
                            background={
                                <div className="tw:relative tw:flex tw:size-full tw:items-center tw:justify-center tw:pb-20">
                                    <BarChart3Icon className="tw:size-8 tw:text-primary" />
                                    <OrbitingCircles radius={90} iconSize={28} duration={24}>
                                        <ZapIcon className="tw:size-5 tw:text-muted-foreground" />
                                        <BarChart3Icon className="tw:size-5 tw:text-muted-foreground" />
                                    </OrbitingCircles>
                                </div>
                            }
                        />
                    </BentoGrid>
                </BlurFade>
            </div>
        </section>
    );
}
