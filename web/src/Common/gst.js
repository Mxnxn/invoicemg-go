// One place that turns a product's GST rate into row fields.
//
// Products carry a single combined rate (Material.tax, e.g. 18). Rows store the components
// separately, because GstReport reads cgst/sgst/igst per row to build the filing - putting
// the whole rate into cgst and leaving sgst at 0 still charges the customer the right money,
// but reports the split at twice the true CGST and nothing on SGST.
//
// CGST and SGST are always exactly half each for an intrastate sale; that is what the halves
// mean. This was duplicated across CreateJobModal (twice) and CreateQuotationModal, which is
// how one of them eventually drifts.

// Intrastate: the rate is halved across CGST and SGST.
//
// Deliberately returns ONLY cgst/sgst. Call sites spread this straight onto a row
// (updateRow(i, splitGst(value))), so adding igst:0 here would silently clear an interstate
// rate the user had set. Use igstOnly() when the sale is interstate.
export const splitGst = (tax) => {
	const rate = Number(tax) || 0;
	const half = rate / 2;
	return { cgst: half, sgst: half };
};

// Interstate: the whole rate sits on IGST instead.
export const igstOnly = (tax) => ({ cgst: 0, sgst: 0, igst: Number(tax) || 0 });

// The combined rate a row represents, whichever way it was recorded.
export const combinedGst = (row = {}) =>
	(Number(row.cgst) || 0) + (Number(row.sgst) || 0) + (Number(row.igst) || 0);

export default { splitGst, igstOnly, combinedGst };
