// Float arithmetic leaves tails like 67.799999999 and 69.999999997 on computed totals, and
// those reach the UI whenever a value is rendered raw instead of through toFixed(2).
// round2 returns a Number for maths; formatAmount returns the display string.
export const round2 = (value) => {
	const n = Number(value);
	if (!Number.isFinite(n)) return 0;
	// +Number.EPSILON nudges values that sit a hair below the .005 boundary (1.005 is
	// actually 1.00499999...) so they round the way a person expects.
	return Math.round((n + Number.EPSILON) * 100) / 100;
};

export const formatAmount = (value) => round2(value).toFixed(2);

export default { round2, formatAmount };
