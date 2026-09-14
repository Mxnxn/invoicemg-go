// A day's work, gathered by customer.
//
// A sheet is one date. What is on it is job-ids, and the question anybody asks of it is
// "whose, and what" - so the shape the page needs is customer -> their job-ids, not a flat
// list of rows.
//
// Kept out of the component because the grouping and the money are the parts that can be
// wrong without looking wrong.

const jobTotal = (job) => Number(job?.total) || 0;
const jobAdvance = (job) => Number(job?.advance) || 0;

// A job's client arrives populated on /jobs/list, but an older payload can still carry a bare
// id - so both shapes resolve to the same key rather than grouping the same customer twice.
export const clientKey = (job) => {
    const c = job?.client_id;
    if (!c) return "";
    return String(c._id || c);
};

const clientName = (job) => {
    const c = job?.client_id;
    if (!c || typeof c !== "object") return { firm: "", person: "" };
    return { firm: c.clientFirm || "", person: c.clientName || "" };
};

/**
 * The job-ids of one day, grouped by the customer they belong to.
 *
 * `date` is the sheet's own "YYYY-MM-DD" string and is compared as a string on purpose:
 * job.receivedDate is stored in that form, and putting either side through `new Date()` shifts
 * the day in any timezone behind UTC - which is how a sheet ends up showing yesterday's work.
 *
 * Customers come out ordered by firm so the page reads the same on every load; a customer's
 * own job-ids keep the order the API sent, which is newest first.
 */
export function groupJobsByClient(jobs, date) {
    const onThisDay = (jobs || []).filter((job) => job && job.receivedDate === date);

    const byClient = new Map();
    onThisDay.forEach((job) => {
        const key = clientKey(job);
        if (!byClient.has(key)) {
            const { firm, person } = clientName(job);
            byClient.set(key, { key, firm, person, jobs: [], total: 0, advance: 0 });
        }
        const group = byClient.get(key);
        group.jobs.push(job);
        group.total += jobTotal(job);
        group.advance += jobAdvance(job);
    });

    return [...byClient.values()]
        .map((g) => ({ ...g, due: g.total - g.advance }))
        .sort((a, b) => (a.firm || "").localeCompare(b.firm || ""));
}

/**
 * What the day came to, across every customer on it.
 *
 * Summed from the groups rather than from the raw list, so the header can never disagree with
 * the sections under it - if a job is missing from a group it is missing from the total too,
 * which is visible, rather than silently counted in a figure nothing accounts for.
 */
export function sheetTotals(groups) {
    return (groups || []).reduce(
        (acc, g) => ({
            total: acc.total + g.total,
            advance: acc.advance + g.advance,
            due: acc.due + (g.total - g.advance),
            jobs: acc.jobs + g.jobs.length,
            customers: acc.customers + 1,
        }),
        { total: 0, advance: 0, due: 0, jobs: 0, customers: 0 }
    );
}

/**
 * Whether a group survives a search. Matches the customer, any of their job-ids, and any
 * product on those job-ids - the three things somebody standing at the bench would type.
 *
 * Returns a group narrowed to the job-ids that matched, so searching a product name does not
 * leave a customer card claiming five job-ids while showing one.
 */
export function filterGroups(groups, term) {
    const needle = String(term || "").trim().toLowerCase();
    if (!needle) return groups || [];

    return (groups || [])
        .map((group) => {
            const customerHit = `${group.firm} ${group.person}`.toLowerCase().includes(needle);
            const jobs = group.jobs.filter((job) => {
                const products = (job.rows || []).map((r) => `${r.material || ""} ${r.description || ""}`).join(" ");
                return `${job.challanNumber || ""} ${products}`.toLowerCase().includes(needle);
            });
            // A customer whose NAME matched keeps all of their job-ids; one that matched only
            // through a product keeps the job-ids that carried it.
            if (customerHit) return group;
            if (jobs.length === 0) return null;
            const total = jobs.reduce((n, j) => n + jobTotal(j), 0);
            const advance = jobs.reduce((n, j) => n + jobAdvance(j), 0);
            return { ...group, jobs, total, advance, due: total - advance };
        })
        .filter(Boolean);
}
