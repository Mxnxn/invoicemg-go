import { Link } from "react-router-dom";
import { ArrowRightIcon, SparklesIcon } from "lucide-react";

import { AuroraBackground } from "../velora/aurora-background";
import { GridPattern } from "../velora/grid-pattern";
import { BlurFade } from "../velora/blur-fade";
import { TextReveal } from "../velora/text-reveal";
import { Typewriter } from "../velora/typewriter";
import { ShimmerButton } from "../velora/shimmer-button";
import { DashboardMockup } from "../demo/DashboardMockup";
import { siteConfig } from "../lib/site-config";

/**
 * Left-set rather than centred, and deliberately so.
 *
 * The headline, the paragraph and the buttons used to be centred on one axis, with the
 * accent word in an animated purple-to-blue gradient and a filled button beside a ghost
 * one. Each is a reasonable choice alone; stacked in a single viewport they are the exact
 * combination that reads as a generated template.
 *
 * Left-setting also earns its keep: the headline runs to two full lines instead of three
 * short centred ones, and every line now starts from the same edge as the paragraph and
 * the buttons, so the eye makes one journey down the page rather than three.
 *
 * The accent word is `text-primary`, the same solid accent Features and Workflow already
 * use for their emphasised words - so the three section headings finally agree.
 */
export function Hero() {
    const signedIn = Boolean(window.localStorage.getItem("uid"));

    return (
        <section className="tw:relative tw:overflow-hidden tw:pt-32 tw:pb-24 tw:lg:pt-44 tw:lg:pb-28">
            <AuroraBackground intensity="subtle" />
            {/* Mask origin moved from 50% to 32% so the grid fades in behind the headline
                rather than behind the empty half the copy no longer occupies. */}
            <GridPattern
                width={48}
                height={48}
                className="tw:fill-transparent tw:stroke-border tw:[mask-image:radial-gradient(ellipse_65%_50%_at_32%_0%,black,transparent)]"
            />
            <div className="tw:relative tw:mx-auto tw:max-w-6xl tw:px-4 tw:lg:px-8">
                <div className="tw:max-w-3xl">
                    <BlurFade delay={0} direction="down">
                        <span className="tw:inline-flex tw:items-center tw:gap-2 tw:rounded-full tw:border tw:border-border tw:bg-card/60 tw:px-4 tw:py-1.5 tw:text-sm tw:backdrop-blur">
                            <SparklesIcon className="tw:size-3.5 tw:text-primary" />
                            <span className="tw:font-medium">Built for print and packaging workflows</span>
                        </span>
                    </BlurFade>

                    <h1 className="tw:mt-8 tw:text-4xl tw:font-semibold tw:tracking-tight tw:text-balance tw:lg:text-6xl">
                        <TextReveal text="Run the whole shop, from quote to" />{" "}
                        <span className="tw:text-primary">
                            <Typewriter words={["invoices.", "challans.", "GST returns."]} />
                        </span>
                    </h1>

                    <BlurFade delay={0.35}>
                        <p className="tw:mt-6 tw:max-w-2xl tw:text-lg tw:text-muted-foreground tw:text-pretty">
                            {siteConfig.description}
                        </p>
                    </BlurFade>

                    <BlurFade delay={0.5}>
                        <div className="tw:mt-10 tw:flex tw:flex-wrap tw:items-center tw:gap-6">
                            <Link to={siteConfig.adminPath}>
                                <ShimmerButton>
                                    {signedIn ? "Go to dashboard" : "Sign in"}
                                    <ArrowRightIcon className="tw:size-4" />
                                </ShimmerButton>
                            </Link>
                            {/* A text link, not a second button. It was previously a ghost
                                <Button> wrapped in an <a>, which nests a control inside a
                                link - two tab stops for one destination, and a screen
                                reader announcing both. */}
                            <a
                                href="#features"
                                className="tw:group tw:inline-flex tw:items-center tw:gap-1.5 tw:rounded-sm tw:text-sm tw:font-medium tw:text-primary tw:underline-offset-4 tw:hover:underline tw:focus-visible:outline-2 tw:focus-visible:outline-offset-4 tw:focus-visible:outline-primary"
                            >
                                See what it does
                                <ArrowRightIcon className="tw:size-4 tw:transition-transform tw:group-hover:translate-x-0.5" />
                            </a>
                        </div>
                    </BlurFade>
                </div>

                {/* Full width, still. The mockup is the evidence for the claim above it and
                    wants the whole column, so only the copy is left-set. */}
                <BlurFade delay={0.75} offset={32}>
                    <DashboardMockup className="tw:mt-20" />
                </BlurFade>
            </div>
        </section>
    );
}
