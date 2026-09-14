import { useState } from "react";
import { ArrowRightIcon, CheckCircle2Icon, LoaderIcon, MessageCircleIcon, SendIcon } from "lucide-react";

import useRequiredFields from "../../Common/useRequiredFields";
import { cn } from "../lib/cn";
import { BlurFade } from "../velora/blur-fade";
import { GridPattern } from "../velora/grid-pattern";
import { Button } from "../ui/Button";
import { DEFAULT_DIAL_CODE, greetingName, normalisePhone, submitEnquiry, whatsappHandoffUrl } from "../lib/enquiry";
import { siteConfig } from "../lib/site-config";

const EMPTY = { name: "", email: "", phone: "", companyName: "", note: "", website: "" };

const NOTE_PLACEHOLDER =
    "What is slowing you down at the moment? Tell us how you invoice, track jobs on the floor or chase payments today — the more detail you give, the better we can show you something useful.";

export function Contact() {
    const [form, setForm] = useState(EMPTY);
    const [sending, setSending] = useState(false);
    const [sent, setSent] = useState(null);

    // The house helper (docs/STYLE.md): one asterisk, one named error per field, submit
    // disabled until the form can actually succeed.
    const required = useRequiredFields(form, {
        name: "Name",
        email: {
            label: "Email",
            validate: (value) => (/^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/.test(value) ? "" : "Enter a valid email address."),
        },
        phone: {
            label: "Phone number",
            // Ten local digits, or a pasted international number.
            validate: (value) => (normalisePhone(value).length >= 9 ? "" : "Enter a valid phone number."),
        },
    });

    const set = (field) => (event) => setForm((prev) => ({ ...prev, [field]: event.target.value }));

    async function onSubmit(event) {
        event.preventDefault();
        if (!required.isComplete) {
            required.showAll();
            return;
        }
        setSending(true);
        try {
            const payload = { ...form, phone: normalisePhone(form.phone) };
            const res = await submitEnquiry(payload);
            // The global interceptor already raises a toast for a non-200 envelope, so this
            // only decides whether to advance the form.
            if (res?.code === 200) setSent(payload);
        } finally {
            setSending(false);
        }
    }

    const handoff = sent ? whatsappHandoffUrl(sent) : "";

    const fieldClass = (field) =>
        cn(
            "tw:w-full tw:rounded-lg tw:border tw:bg-background tw:px-3 tw:py-2 tw:text-sm tw:transition-colors",
            "tw:focus-visible:outline-2 tw:focus-visible:outline-offset-1 tw:focus-visible:outline-ring",
            required.errorFor(field) ? "tw:border-danger" : "tw:border-border"
        );

    const label = (field, text) => (
        <label htmlFor={`enquiry-${field}`} className="tw:mb-1 tw:block tw:text-xs tw:font-medium">
            {text}
            {required.errors[field] !== undefined && <span className="tw:ml-0.5 tw:text-danger">*</span>}
        </label>
    );

    const error = (field) =>
        required.errorFor(field) ? (
            <span className="tw:mt-1 tw:block tw:text-[11px] tw:text-danger">{required.errorFor(field)}</span>
        ) : null;

    return (
        <section id="contact" className="tw:relative tw:overflow-hidden tw:py-24 tw:lg:py-32">
            <GridPattern
                width={48}
                height={48}
                className="tw:fill-transparent tw:stroke-border tw:[mask-image:radial-gradient(ellipse_60%_60%_at_50%_50%,black,transparent)]"
            />
            <div className="tw:relative tw:mx-auto tw:grid tw:max-w-6xl tw:items-start tw:gap-12 tw:px-4 tw:lg:grid-cols-2 tw:lg:gap-20 tw:lg:px-8">
                <BlurFade direction="right">
                    <div>
                        <span className="tw:text-sm tw:font-medium tw:text-primary">Book a demo</span>
                        <h2 className="tw:mt-3 tw:text-3xl tw:font-semibold tw:tracking-tight tw:text-balance tw:lg:text-4xl">
                            No price list — <span className="tw:text-primary">just a conversation</span>
                        </h2>
                        <p className="tw:mt-4 tw:text-muted-foreground">
                            Every print shop runs differently, so we would rather see yours before quoting anything.
                            Tell us what you are working with and we will walk you through InvoiceMG on your own jobs.
                        </p>
                        <p className="tw:mt-4 tw:text-sm tw:text-muted-foreground">
                            We reply on WhatsApp, usually the same day.
                        </p>
                    </div>
                </BlurFade>

                <BlurFade direction="left" delay={0.15}>
                    <div className="tw:rounded-2xl tw:border tw:border-border tw:bg-card tw:p-6 tw:shadow-xl">
                        {sent ? (
                            <div className="tw:text-center">
                                <span className="tw:mx-auto tw:mb-4 tw:flex tw:size-12 tw:items-center tw:justify-center tw:rounded-full tw:bg-primary/10 tw:text-primary">
                                    <CheckCircle2Icon className="tw:size-6" />
                                </span>
                                <h3 className="tw:text-lg tw:font-semibold">Thanks, {greetingName(sent.name)}</h3>
                                <p className="tw:mt-2 tw:text-sm tw:text-muted-foreground">
                                    We have your details and will be in touch shortly. In the meantime, the demo is
                                    open — it runs on its own sample data, so nothing you do there is real.
                                </p>

                                {/* The primary next step. A plain link, not a redirect: sending
                                    someone away from their own confirmation is disorienting, and
                                    it would bury the WhatsApp option below. */}
                                <a href={siteConfig.demoUrl} target="_blank" rel="noopener noreferrer">
                                    <Button className="tw:mt-4">
                                        Open the demo
                                        <ArrowRightIcon className="tw:size-4" />
                                    </Button>
                                </a>

                                {/* Credentials, because a login screen with no way in is a dead
                                    end. Selectable rather than an image so they can be copied. */}
                                <p className="tw:mt-3 tw:text-xs tw:text-muted-foreground">
                                    Sign in with{" "}
                                    <span className="tw:font-medium tw:text-foreground">{siteConfig.demoEmail}</span>
                                    {" / "}
                                    <span className="tw:font-medium tw:text-foreground">{siteConfig.demoPassword}</span>
                                </p>
                                {handoff && (
                                    <>
                                        <p className="tw:mt-4 tw:text-sm tw:text-muted-foreground">
                                            In a hurry? Send it straight to us on WhatsApp.
                                        </p>
                                        {/* Deliberately a second, explicit click: opening a window
                                            after an awaited request is what popup blockers stop. */}
                                        <a href={handoff} target="_blank" rel="noopener noreferrer">
                                            <Button className="tw:mt-3">
                                                <MessageCircleIcon className="tw:size-4" />
                                                Continue on WhatsApp
                                            </Button>
                                        </a>
                                    </>
                                )}
                            </div>
                        ) : (
                            <form onSubmit={onSubmit} noValidate>
                                <div className="tw:grid tw:gap-4 tw:sm:grid-cols-2">
                                    <div className="tw:sm:col-span-2">
                                        {label("name", "Your name")}
                                        <input
                                            id="enquiry-name"
                                            value={form.name}
                                            onChange={set("name")}
                                            autoComplete="name"
                                            {...required.props("name")}
                                            className={fieldClass("name")}
                                        />
                                        {error("name")}
                                    </div>

                                    <div>
                                        {label("email", "Email")}
                                        <input
                                            id="enquiry-email"
                                            type="email"
                                            value={form.email}
                                            onChange={set("email")}
                                            autoComplete="email"
                                            {...required.props("email")}
                                            className={fieldClass("email")}
                                        />
                                        {error("email")}
                                    </div>

                                    <div>
                                        {label("phone", "Phone")}
                                        {/* The country code is shown, not typed - people enter a
                                            local number, and a pasted +code still wins. */}
                                        {/* items-stretch with the padding on the input, not the
                                            wrapper: putting it on both made this group 42px
                                            against every other field's 38px. The ring moves to
                                            focus-within, since the input suppresses its own. */}
                                        <div
                                            className={cn(
                                                "tw:flex tw:items-stretch tw:overflow-hidden tw:rounded-lg tw:border tw:bg-background",
                                                "tw:focus-within:outline-2 tw:focus-within:outline-offset-1 tw:focus-within:outline-ring",
                                                required.errorFor("phone") ? "tw:border-danger" : "tw:border-border"
                                            )}
                                        >
                                            <span className="tw:flex tw:items-center tw:border-r tw:border-border tw:bg-muted tw:px-2.5 tw:text-sm tw:text-muted-foreground tw:select-none">
                                                {DEFAULT_DIAL_CODE}
                                            </span>
                                            <input
                                                id="enquiry-phone"
                                                inputMode="tel"
                                                value={form.phone}
                                                onChange={set("phone")}
                                                autoComplete="tel"
                                                placeholder="99999 99999"
                                                onBlur={() => required.markTouched("phone")}
                                                aria-invalid={Boolean(required.errorFor("phone"))}
                                                className="tw:w-full tw:bg-transparent tw:px-3 tw:py-2 tw:text-sm tw:focus-visible:outline-none"
                                            />
                                        </div>
                                        {error("phone")}
                                    </div>

                                    <div className="tw:sm:col-span-2">
                                        {label("companyName", "Company name")}
                                        <input
                                            id="enquiry-companyName"
                                            value={form.companyName}
                                            onChange={set("companyName")}
                                            autoComplete="organization"
                                            className={fieldClass("companyName")}
                                        />
                                    </div>

                                    <div className="tw:sm:col-span-2">
                                        {label("note", "Anything you want us to know")}
                                        <textarea
                                            id="enquiry-note"
                                            rows={4}
                                            value={form.note}
                                            onChange={set("note")}
                                            placeholder={NOTE_PLACEHOLDER}
                                            className={cn(fieldClass("note"), "tw:resize-y")}
                                        />
                                    </div>
                                </div>

                                {/* Honeypot: off-screen, hidden from assistive tech, never
                                    autofilled. Only a bot reaches it. */}
                                <div className="tw:absolute tw:left-[-9999px]" aria-hidden="true">
                                    <label htmlFor="enquiry-website">Website</label>
                                    <input
                                        id="enquiry-website"
                                        tabIndex={-1}
                                        autoComplete="off"
                                        value={form.website}
                                        onChange={set("website")}
                                    />
                                </div>

                                <Button type="submit" size="lg" disabled={sending} className="tw:mt-5 tw:w-full">
                                    {sending ? (
                                        <LoaderIcon className="tw:size-4 tw:animate-spin" />
                                    ) : (
                                        <SendIcon className="tw:size-4" />
                                    )}
                                    {sending ? "Sending" : "Request a demo"}
                                </Button>
                                <p className="tw:mt-3 tw:text-center tw:text-[11px] tw:text-muted-foreground">
                                    We only use these details to reply to you.
                                </p>
                            </form>
                        )}
                    </div>
                </BlurFade>
            </div>
        </section>
    );
}
