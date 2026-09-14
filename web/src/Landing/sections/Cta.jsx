import { Link } from "react-router-dom";
import {
    ArrowRightIcon,
    BarChart3Icon,
    FileTextIcon,
    ListChecksIcon,
    MessageCircleIcon,
    MoonIcon,
    TruckIcon,
} from "lucide-react";

import { AuroraBackground } from "../velora/aurora-background";
import { Particles } from "../velora/particles";
import { BlurFade } from "../velora/blur-fade";
import { ShimmerButton } from "../velora/shimmer-button";
import { Dock, DockIcon } from "../velora/dock";
import { Button } from "../ui/Button";
import { siteConfig } from "../lib/site-config";

export function Cta() {
    const signedIn = Boolean(window.localStorage.getItem("uid"));

    return (
        <section className="tw:relative tw:overflow-hidden tw:py-24 tw:lg:py-32">
            <AuroraBackground intensity="subtle" />
            <Particles quantity={50} />
            <div className="tw:relative tw:mx-auto tw:max-w-4xl tw:px-4 tw:text-center tw:lg:px-8">
                <BlurFade>
                    <h2 className="tw:text-3xl tw:font-semibold tw:tracking-tight tw:text-balance tw:lg:text-5xl">
                        Stop chasing paper. <span className="tw:text-primary">Start dispatching.</span>
                    </h2>
                    <p className="tw:mx-auto tw:mt-6 tw:max-w-xl tw:text-lg tw:text-muted-foreground">
                        Every invoice, challan and job card in one place - with the ledger already balanced.
                    </p>
                    <div className="tw:mt-10 tw:flex tw:flex-wrap tw:items-center tw:justify-center tw:gap-4">
                        <Link to={siteConfig.adminPath}>
                            <ShimmerButton className="tw:h-14 tw:px-10 tw:text-base">
                                {signedIn ? "Go to dashboard" : "Sign in"}
                                <ArrowRightIcon className="tw:size-5" />
                            </ShimmerButton>
                        </Link>
                        {/* No price list to send people to - the next step is a conversation. */}
                        <a href="#contact">
                            <Button variant="outline" size="lg" className="tw:h-14 tw:px-8 tw:text-base">
                                <MessageCircleIcon className="tw:size-5" />
                                Contact us
                            </Button>
                        </a>
                    </div>
                </BlurFade>
                <BlurFade delay={0.2}>
                    <div className="tw:mt-16">
                        <Dock>
                            <DockIcon label="Invoices">
                                <FileTextIcon className="tw:size-5" />
                            </DockIcon>
                            <DockIcon label="Challans">
                                <TruckIcon className="tw:size-5" />
                            </DockIcon>
                            <DockIcon label="Lifecycle">
                                <ListChecksIcon className="tw:size-5" />
                            </DockIcon>
                            <DockIcon label="Analytics">
                                <BarChart3Icon className="tw:size-5" />
                            </DockIcon>
                            <DockIcon label="Dark mode">
                                <MoonIcon className="tw:size-5" />
                            </DockIcon>
                        </Dock>
                    </div>
                </BlurFade>
            </div>
        </section>
    );
}
