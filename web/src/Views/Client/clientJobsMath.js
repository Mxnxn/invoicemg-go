// Job status predicates for the Client page. Extracted from ClientJobsTab so the stat cards
// above the table and the badges inside it are computed from the same definitions - if these
// drifted apart, the "Ready For Invoice" count would disagree with the rows you can see.

export const pendingRowsCount = (job) => (job.rows || []).filter((row) => row.queue !== "Done").length;

// Every row finished on the shop floor. Says nothing about whether it's been billed yet.
export const isReadyForInvoice = (job) => (job.rows || []).length > 0 && pendingRowsCount(job) === 0;

// A Ready-For-Invoice job isn't necessarily still waiting on the Add Entry step - once every
// row has been converted it's "Converted", and once the resulting entries have actually been
// issued on an invoice it's "Invoiced".
export const isFullyConverted = (job) => (job.rows || []).length > 0 && (job.rows || []).every((row) => row.entry_id);

export const isFullyInvoiced = (job) => isFullyConverted(job) && (job.rows || []).every((row) => row.entry_id?.has_issued);

// job.advance/job.total only reflect payment recorded directly against the job itself
// (Batch Receive) - once rows convert to Entries, they can also get paid off downstream via
// an Invoice close (routes/Invoice.js's /paid forces every linked entry's total to 0), which
// never writes back to the job. So a job also counts as Paid once every one of its converted
// rows' entries has settled to total === 0, even if job.advance itself never moved.
export const isJobPaid = (job) => {
    if (job.total > 0 && job.advance >= job.total) return true;
    return isFullyConverted(job) && (job.rows || []).every((row) => Number(row.entry_id?.total) === 0);
};

// Counts for the stat cards. The three buckets are mutually exclusive and cover every job, so
// they always sum to the total - a job is either still being worked (pending), finished but
// not yet billed (readyForInvoice), or billed (invoiced). "Invoiced" is checked first
// because an invoiced job is also, trivially, ready for invoice.
export const jobStatusCounts = (jobs = []) => {
    let invoiced = 0;
    let readyForInvoice = 0;
    let pending = 0;

    for (const job of jobs) {
        if (isFullyInvoiced(job)) invoiced += 1;
        else if (isReadyForInvoice(job)) readyForInvoice += 1;
        else pending += 1;
    }

    return { invoiced, readyForInvoice, pending, total: jobs.length };
};
