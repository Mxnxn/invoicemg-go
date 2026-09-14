// The letterhead comes from ./letterhead.js so the printed page and the spreadsheet cannot
// disagree about it - see that file for why it is not defined here.
import { headerLinesFor } from "./letterhead";

// Building a branded spreadsheet.
//
// The layout decisions live in buildSheetModel, which is pure: no ExcelJS, no network, no
// DOM. That is what makes "does a company with no logo still get a header?" a test rather
// than something discovered by a customer opening the file.

const MIN_WIDTH = 10;
const MAX_WIDTH = 48;

// ExcelJS's addImage only renders png, jpeg and gif. A company's logo can be webp or avif
// (ImageUpload.js on the API accepts both, and the stored filename keeps whatever extension
// was uploaded), so guessing "png or else jpeg" from the URL mislabels those and Excel shows
// a broken image or a repair prompt. The extension is read from the real filename instead,
// and anything ExcelJS can't render maps to null so the caller skips the image - the text
// header already carries the company identity as a fallback.
const EXCELJS_EXTENSIONS = { png: "png", jpg: "jpeg", jpeg: "jpeg", gif: "gif" };

// The logo's drawn size, and the rows the letterhead must reserve for it.
//
// These belong together: the image is drawn as a floating picture at a pixel size, while the
// text below it is positioned in ROWS. If the two disagree the logo overlaps the title, and
// nothing catches it short of opening the file. A default Excel row is ~15px, so the reserved
// count is the height divided by that, rounded up.
export const LOGO_WIDTH = 180;
export const LOGO_HEIGHT = 90;
const EXCEL_ROW_HEIGHT = 15;
export const LOGO_ROWS = Math.ceil(LOGO_HEIGHT / EXCEL_ROW_HEIGHT);

export function imageExtensionForLogo(url) {
    const clean = String(url || "").split(/[?#]/)[0];
    const match = /\.([a-z0-9]+)$/i.exec(clean);
    const ext = match ? match[1].toLowerCase() : "";
    return EXCELJS_EXTENSIONS[ext] || null;
}

// Excel sheet names must be <=31 chars and can't contain : \ / ? * [ ] - the report's title
// (used verbatim as the downloaded file name, which has no such limit) can't share a value
// with the worksheet name, so it goes through this rather than a bare slice(0, 31).
export function sheetNameFor(title) {
    return String(title || "Report")
        .replace(/[:\\/?*[\]]/g, "")
        .slice(0, 31);
}



/**
 * The layout, decided before any spreadsheet exists.
 *
 * Returns the header lines, the logo filename (or null), the width for each column, the
 * alignment for each column, whether a totals row is reserved, and the row the table starts
 * on.
 */
export function buildSheetModel({ title, subtitle, columns = [], rows = [], foot, company, template = {}, summary = [] }) {
    const headerLines = headerLinesFor(company, template);
    const logo = template.logo !== false && company?.url ? company.url : null;
    const summaryRows = summary.length;
    // The "Report period" toggle in Configure -> Exports (Company.exportTemplate.showPeriod)
    // is the only period text a report prints, so honouring it here is honouring it wherever
    // buildSheetModel is the source of truth for row layout.
    const showSubtitle = template.showPeriod !== false && Boolean(subtitle);

    // Widths follow the longest thing in the column, capped: uncapped, one long description
    // pushes the money columns off the screen, and too narrow renders numbers as ####.
    const columnWidths = columns.map((col, i) => {
        const longestCell = rows.reduce((max, row) => {
            const cell = row?.[i];
            return Math.max(max, String(cell == null ? "" : cell).length);
        }, 0);
        const longest = Math.max(String(col.label || "").length, longestCell);
        return Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, longest + 2));
    });

    // Alignment is a layout decision, not an ExcelJS one - exportWorkbook reads it rather
    // than recomputing col.numeric ? "right" : "left" itself, so there is one place that
    // decides and one place that applies.
    const alignments = columns.map((col) => (col.numeric ? "right" : "left"));

    // One blank row after the letterhead, the title, the subtitle if any, the summary block
    // if any, a blank, then the column headers. A company with nothing enabled starts higher
    // up rather than leaving a gap where a letterhead would have been.
    const letterheadRows = Math.max(headerLines.length, logo ? LOGO_ROWS : 0);
    const startRow = letterheadRows + (letterheadRows ? 2 : 1) + 1 + (showSubtitle ? 1 : 0) + summaryRows + 1;

    // Cell positions, computed once here rather than re-derived from startRow at the call
    // site - a wrong offset would silently overwrite the subtitle with the summary or the
    // summary with the header row, and nothing would catch it short of opening the file.
    // Title sits right after the letterhead gap regardless of what follows; subtitle and
    // summary cascade below it, in that order, before the blank row and the column headers.
    const titleRow = startRow - summaryRows - (showSubtitle ? 3 : 2);
    const subtitleRow = showSubtitle ? startRow - summaryRows - 2 : null;
    const summaryStartRow = summaryRows > 0 ? startRow - summaryRows - 1 : null;

    return {
        headerLines,
        logo,
        columnWidths,
        alignments,
        startRow,
        hasFoot: Boolean(foot),
        summaryRows,
        titleRow,
        subtitleRow,
        summaryStartRow,
    };
}

/**
 * Build the workbook and return it as a Blob.
 *
 * ExcelJS is imported here rather than at module scope so its weight lands only on someone
 * who actually downloads - the same reason @react-pdf/renderer is imported inside the click
 * handler in ReportDownloads.
 */
export async function exportWorkbook({ title, subtitle, columns = [], rows = [], foot, company, template = {}, logoUrl, summary = [] }) {
    const { default: ExcelJS } = await import("exceljs");
    const model = buildSheetModel({ title, subtitle, columns, rows, foot, company, template, summary });

    const workbook = new ExcelJS.Workbook();
    workbook.creator = company?.firm || "InvoiceMG";
    workbook.created = new Date();
    const sheet = workbook.addWorksheet(sheetNameFor(title));

    sheet.columns = model.columnWidths.map((width) => ({ width }));

    model.headerLines.forEach((line, i) => {
        const cell = sheet.getCell(i + 1, 1);
        cell.value = line;
        cell.font = { bold: i === 0, size: i === 0 ? 14 : 10 };
    });

    // The logo is fetched, not read from disk - this runs in a browser. A failure here must
    // not lose the export: the sheet is still correct without it.
    const logoExtension = model.logo ? imageExtensionForLogo(logoUrl) : null;
    if (model.logo && logoUrl && logoExtension) {
        try {
            const response = await fetch(logoUrl);
            if (response.ok) {
                const buffer = await response.arrayBuffer();
                const imageId = workbook.addImage({ buffer, extension: logoExtension });
                sheet.addImage(imageId, { tl: { col: Math.max(0, columns.length - 2), row: 0 }, ext: { width: LOGO_WIDTH, height: LOGO_HEIGHT } });
            }
        } catch {
            // No logo on the sheet; the header text still identifies the company.
        }
    }

    const titleCell = sheet.getCell(model.titleRow, 1);
    titleCell.value = title;
    titleCell.font = { bold: true, size: 12 };
    if (model.subtitleRow) {
        const sub = sheet.getCell(model.subtitleRow, 1);
        sub.value = subtitle;
        sub.font = { size: 10, color: { argb: "FF6B7280" } };
    }

    // The same figures the PDF prints as a strip of stat boxes (ReportPDF's summaryRow) - a
    // spreadsheet has no room for that layout, so each becomes a label/value row instead. The
    // XLSX and PDF must agree on what a report totals to, not just on its line items.
    summary.forEach((item, i) => {
        const summaryRow = sheet.getRow(model.summaryStartRow + i);
        const label = summaryRow.getCell(1);
        label.value = item.label;
        label.font = { size: 9, color: { argb: "FF6B7280" } };
        const value = summaryRow.getCell(2);
        value.value = item.value;
        value.font = { bold: true, size: 10 };
    });

    const headerRow = sheet.getRow(model.startRow);
    columns.forEach((col, i) => {
        const cell = headerRow.getCell(i + 1);
        cell.value = col.label;
        cell.font = { bold: true };
        cell.fill = { type: "pattern", pattern: "solid", fgColor: { argb: "FFEFF1F3" } };
        cell.border = { bottom: { style: "thin", color: { argb: "FFD2D5D9" } } };
        cell.alignment = { vertical: "middle", horizontal: model.alignments[i] };
    });
    // Freezing keeps the headers visible in a long report.
    sheet.views = [{ state: "frozen", ySplit: model.startRow }];

    rows.forEach((row, r) => {
        const sheetRow = sheet.getRow(model.startRow + 1 + r);
        columns.forEach((col, i) => {
            const cell = sheetRow.getCell(i + 1);
            const raw = row?.[i];
            cell.value = raw == null ? "" : raw;
            cell.alignment = { horizontal: model.alignments[i] };
            if (col.numeric && typeof raw === "number") cell.numFmt = "#,##0.00";
        });
    });

    if (foot) {
        const footRow = sheet.getRow(model.startRow + 1 + rows.length);
        foot.forEach((value, i) => {
            const cell = footRow.getCell(i + 1);
            cell.value = value == null ? "" : value;
            cell.font = { bold: true };
            cell.border = { top: { style: "thin", color: { argb: "FFD2D5D9" } } };
        });
    }

    const buffer = await workbook.xlsx.writeBuffer();
    return new Blob([buffer], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
}
