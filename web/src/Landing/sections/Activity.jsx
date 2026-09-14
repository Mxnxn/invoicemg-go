import { BlurFade } from "../velora/blur-fade";
import { RetroGrid } from "../velora/retro-grid";
import { ActivityFeed } from "../demo/ActivityFeed";

export function Activity() {
    return (
        <section className="tw:relative tw:overflow-hidden tw:py-24 tw:lg:py-32">
            <RetroGrid />
            <div className="tw:relative tw:mx-auto tw:grid tw:max-w-6xl tw:items-center tw:gap-12 tw:px-4 tw:lg:grid-cols-2 tw:lg:gap-20 tw:lg:px-8">
                <BlurFade direction="right" className="tw:order-2 tw:lg:order-1">
                    <ActivityFeed />
                </BlurFade>
                <BlurFade direction="left" delay={0.15} className="tw:order-1 tw:lg:order-2">
                    <div>
                        <span className="tw:text-sm tw:font-medium tw:text-primary">Job lifecycle</span>
                        <h2 className="tw:mt-3 tw:text-3xl tw:font-semibold tw:tracking-tight tw:text-balance tw:lg:text-4xl">
                            Watch the floor <span className="tw:text-primary">move</span>
                        </h2>
                        <p className="tw:mt-4 tw:text-muted-foreground">
                            Payments, challans, stage changes and wastage all land on the same timeline. Every move is
                            recorded against the job, with notes and history you can go back through.
                        </p>
                        <p className="tw:mt-4 tw:text-muted-foreground">
                            Nothing is ever really deleted - removed records go to Trash, where they can be restored.
                        </p>
                    </div>
                </BlurFade>
            </div>
        </section>
    );
}
