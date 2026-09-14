// One customer's job-ids, gathered by the day they came in.
//
// The inversion of the sheet: that page is one day across many customers, this one is one
// customer across many days. Same two nouns, so the same shape of answer - a heading you can
// read, and the work under it.
//
// Kept out of the component because the grouping and the money are the parts that can be wrong
// without looking wrong.

const num = (v) => Number(v) || 0;

/**
 * Job-ids grouped by `receivedDate`.
 *
 * The date is compared and sorted as a STRING on purpose. receivedDate is stored as
 * "YYYY-MM-DD", which sorts chronologically on its own, and putting it through `new Date()`
 * shifts the day in any timezone behind UTC - the bug that had every sheet on the dashboard
 * showing the day before.
 *
 * A job with no date at all is not dropped. It is real work someone can see in the table it
 * came from, and silently hiding it here would make this page disagree with that one; it goes
 * into a group of its own at the end.
 */
export function groupJobsByDate(jobs, { oldestFirst = false } = {}) {
    const byDate = new Map();

    (jobs || []).forEach((job) => {
        const date = job?.receivedDate || "";
        if (!byDate.has(date)) byDate.set(date, { date, jobs: [], total: 0, advance: 0 });
        const group = byDate.get(date);
        group.jobs.push(job);
        group.total += num(job.total);
        group.advance += num(job.advance);
    });

    const groups = [...byDate.values()].map((g) => ({ ...g, due: g.total - g.advance }));

    // Undated work sits at the end whichever way the rest is ordered - it has no place on a
    // timeline, and putting it first would bury today's work under it.
    const dated = groups.filter((g) => g.date);
    const undated = groups.filter((g) => !g.date);
    dated.sort((a, b) => (oldestFirst ? a.date.localeCompare(b.date) : b.date.localeCompare(a.date)));
    return [...dated, ...undated];
}

/**
 * What the visible job-ids come to. Summed from the groups rather than the raw list so the
 * header cannot disagree with the sections beneath it.
 */
export function groupTotals(groups) {
    return (groups || []).reduce(
        (acc, g) => ({
            total: acc.total + g.total,
            advance: acc.advance + g.advance,
            due: acc.due + (g.total - g.advance),
            jobs: acc.jobs + g.jobs.length,
            days: acc.days + 1,
        }),
        { total: 0, advance: 0, due: 0, jobs: 0, days: 0 }
    );
}
