/** Single source of truth for the landing page's copy-level constants. */
export const siteConfig = {
    name: "InvoiceMG",
    tagline: "Invoicing, challans and job tracking for print and packaging",
    description:
        "InvoiceMG turns client entries, material sheets and job cards into GST-ready invoices, delivery challans and ledgers - with a live job queue your floor can follow.",
    adminPath: "/admin",

    // Where "Try the demo" sends someone after they ask for one. Read from the environment
    // so the demo can move without a code change, with the current deployment as the
    // fallback - a missing env var should leave the button working, not pointing at nothing.
    //
    // /admin rather than the site root: with no session that path renders the login form
    // (see Views/Auth/ProtectiveRoute), which is exactly where a visitor wanting to look
    // around needs to land.
    demoUrl: import.meta.env.VITE_DEMO_URL || "https://invoicemg-demo.vercel.app/admin",

    // Published deliberately. The demo runs on its own seeded database - invented customers,
    // invented jobs - so these open nothing real. Sending someone to a login screen without
    // them would be a dead end.
    demoEmail: "demo@invoicemg.in",
    demoPassword: "demo1234",
};
