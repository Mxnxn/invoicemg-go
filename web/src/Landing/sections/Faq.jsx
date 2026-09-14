import { BlurFade } from "../velora/blur-fade";
import { Accordion } from "../ui/Accordion";

const FAQS = [
    {
        q: "Is InvoiceMG GST-ready?",
        a: "Yes. Invoices carry HSN/SAC codes, tax splits and round-off, and the GST report view produces period-wise summaries you can export to Excel.",
    },
    {
        q: "Can one login handle multiple companies?",
        a: "Yes. Company profiles are scoped per session, and each browser tab can be bound to a different company at the same time - useful when you keep books for more than one firm.",
    },
    {
        q: "Can I get my data out?",
        a: "Every register - invoices, purchases, ledgers and GST summaries - exports to .xlsx and PDFs from inside the app.",
    },
    {
        q: "Who can see what?",
        a: "Access is role-based. Admins see everything; employees only see the features granted to them, and the permission set is fixed at login.",
    },
    {
        q: "Can I track stock?",
        a: "Yes. The inventory report works out stock per product - bought, less what jobs consumed, less wastage - over any date range. Negative stock is shown rather than hidden, because it usually means a purchase was never entered.",
    },
];

export function Faq() {
    return (
        <section id="faq" className="tw:py-24 tw:lg:py-32">
            <div className="tw:mx-auto tw:max-w-3xl tw:px-4 tw:lg:px-8">
                <BlurFade>
                    <h2 className="tw:text-center tw:text-3xl tw:font-semibold tw:tracking-tight tw:lg:text-4xl">
                        Frequently asked questions
                    </h2>
                </BlurFade>
                <BlurFade delay={0.15}>
                    <Accordion items={FAQS} className="tw:mt-12" />
                </BlurFade>
            </div>
        </section>
    );
}
