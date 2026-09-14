export const getDate = (date) => {
	let dd = new Date(date).getDate();
	let mm = new Date(date).getMonth() + 1;
	const yy = new Date(date).getFullYear();

	mm = mm < 10 ? "0" + mm : mm;
	dd = dd < 10 ? "0" + dd : dd;
	return `${dd}-${mm}-${yy}`;
};

const MONTH_NAMES = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// Formats a date as "MMM DD, YYYY" (e.g. "Aug 03, 2026") for display on invoices.
export const getDateForInvoice = (date) => {
	const d = new Date(date);
	const dd = String(d.getDate()).padStart(2, "0");
	const mmm = MONTH_NAMES[d.getMonth()];
	const yy = d.getFullYear();
	return `${mmm} ${dd}, ${yy}`;
};

export const getDateForEntry = (date) => {
	let dd = new Date(date).getDate();
	let mm = new Date(date).getMonth() + 1;
	const yy = new Date(date).getFullYear();

	mm = mm < 10 ? "0" + mm : mm;
	dd = dd < 10 ? "0" + dd : dd;
	return `${yy}-${mm}-${dd}`;
};
