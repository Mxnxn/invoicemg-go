// The company block that heads every exported report, as lines.
//
// Its own module because BOTH formats need it and neither may import the other:
// exportWorkbook.js pulls in ExcelJS and is lazily loaded at download time, so importing it
// from ReportPDF would drag a spreadsheet library into the PDF path. This has no dependencies
// at all.
//
// One source of truth is the point. The rules live in Configure > Exports
// (Company.exportTemplate), and until this existed only the spreadsheet honoured them - the
// printed page always showed the firm and GST whatever the toggles said, and never showed the
// phone or the logo at all. Two formats of the same report disagreeing about the letterhead
// is exactly the drift ReportPDF's own header comment warns about.
//
// Every toggle defaults to ON when absent (`!== false`), so a company that predates
// exportTemplate keeps the letterhead it has always had.

/** The company block, as lines, honouring what the template turns on. */
export function headerLinesFor(company, template = {}) {
    if (!company) return [];
    const lines = [];
    if (template.firm !== false && company.firm) lines.push(company.firm);
    if (template.address !== false && company.address) lines.push(company.address);

    const contact = [];
    if (template.phone !== false && company.phone) contact.push(company.phone);
    if (template.gst !== false && company.gst) contact.push(`GST ${company.gst}`);
    if (contact.length) lines.push(contact.join("  ·  "));

    return lines;
}

/** Whether the logo is wanted AND there is one to show. */
export function wantsLogo(company, template = {}) {
    return template.logo !== false && Boolean(company?.url);
}

/**
 * Whether the report's period line prints.
 *
 * The "Report period" toggle is the only period text a report shows, so both formats have to
 * read it the same way or one of them prints a range the other omits.
 */
export function showsSubtitle(subtitle, template = {}) {
    return template.showPeriod !== false && Boolean(subtitle);
}
