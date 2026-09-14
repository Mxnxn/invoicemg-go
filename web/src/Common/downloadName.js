// Download filenames, in one place.
//
// Convention: <YYYY-MM-DD>_<Firm>.<ext> - the date the file was produced, then who it is
// about (the client for an invoice, the company for a report). Every export used to invent
// its own name, and the invoice preview had none at all, so a downloaded invoice landed as
// whatever the browser called a blob.

// Filesystem-safe, readable, and short enough to stay legible in a downloads list. Spaces
// become nothing rather than underscores so "Plain Firm" reads as PlainFirm, not Plain_Firm -
// the underscore is the separator between date and firm, and doubling its job makes the
// name ambiguous.
export function firmSlug(name, fallback = "Firm") {
	const cleaned = String(name || "")
		.replace(/[^A-Za-z0-9 ]/g, "")
		.trim();
	if (!cleaned) return fallback;
	return cleaned
		.split(/\s+/)
		.map((w) => w.charAt(0).toUpperCase() + w.slice(1))
		.join("")
		.slice(0, 40);
}

export const isoDate = (d = new Date()) => {
	const date = d instanceof Date ? d : new Date(d);
	return Number.isNaN(date.getTime()) ? isoDate(new Date()) : date.toISOString().slice(0, 10);
};

// `suffix` distinguishes reports that would otherwise collide on the same day - two exports
// from one firm on one date would overwrite each other in the downloads folder.
export function downloadName({ firm, ext, date, suffix = "" } = {}) {
	const parts = [isoDate(date), firmSlug(firm)];
	if (suffix) parts.push(firmSlug(suffix, ""));
	return `${parts.filter(Boolean).join("_")}.${String(ext || "").replace(/^\./, "")}`;
}
