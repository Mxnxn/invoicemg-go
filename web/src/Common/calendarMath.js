// Calendar arithmetic for DateField, kept pure and free of Date parsing.
//
// Everything works on "YYYY-MM-DD" strings and plain {year, month, day} parts. Deliberately
// NOT via new Date("2026-08-26"): that parses as UTC midnight, so in any timezone behind UTC
// it reads back as the 25th - the same bug that made a sheet's date show a day off, which is
// why Views/dashboardCards.js reads ISO strings through parseISO below rather than through
// Date. Constructing from parts uses local time and avoids it entirely.

export const WEEKDAYS = ["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"];

export const MONTHS = [
    "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December",
];

const pad = (n) => String(n).padStart(2, "0");

export const toISO = ({ year, month, day }) => `${year}-${pad(month + 1)}-${pad(day)}`;

// "YYYY-MM-DD" -> { year, month (0-11), day }, or null for anything else.
export const parseISO = (value) => {
    const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(value || ""));
    if (!m) return null;
    const year = Number(m[1]);
    const month = Number(m[2]) - 1;
    const day = Number(m[3]);
    if (month < 0 || month > 11 || day < 1 || day > daysInMonth(year, month)) return null;
    return { year, month, day };
};

export const toMonthISO = ({ year, month }) => `${year}-${pad(month + 1)}`;

// "YYYY-MM" -> { year, month }. Same reason as parseISO for not going through Date.
export const parseMonthISO = (value) => {
    const m = /^(\d{4})-(\d{2})$/.exec(String(value || ""));
    if (!m) return null;
    const month = Number(m[2]) - 1;
    if (month < 0 || month > 11) return null;
    return { year: Number(m[1]), month };
};

export const formatMonthDisplay = (value) => {
    const parts = parseMonthISO(value);
    if (!parts) return "";
    return `${MONTHS[parts.month].slice(0, 3)} ${parts.year}`;
};

export const todayParts = () => {
    const now = new Date();
    return { year: now.getFullYear(), month: now.getMonth(), day: now.getDate() };
};

export const daysInMonth = (year, month) => new Date(year, month + 1, 0).getDate();

// Monday-first index of the 1st, so the grid lines up under WEEKDAYS.
const firstWeekday = (year, month) => (new Date(year, month, 1).getDay() + 6) % 7;

export const addMonths = ({ year, month }, delta) => {
    const total = year * 12 + month + delta;
    return { year: Math.floor(total / 12), month: ((total % 12) + 12) % 12 };
};

// Six rows of seven, always - a grid that changes height as you page through months makes
// the buttons move under the cursor. Days outside the month come back as null.
export const monthGrid = (year, month) => {
    const lead = firstWeekday(year, month);
    const count = daysInMonth(year, month);
    const cells = [];
    for (let i = 0; i < 42; i += 1) {
        const day = i - lead + 1;
        cells.push(day >= 1 && day <= count ? day : null);
    }
    return cells;
};

export const isSameDay = (a, b) => Boolean(a && b && a.year === b.year && a.month === b.month && a.day === b.day);

// What the closed field shows. Long-form so "08/09" is never ambiguous between day and month.
export const formatDisplay = (value) => {
    const parts = parseISO(value);
    if (!parts) return "";
    return `${pad(parts.day)} ${MONTHS[parts.month].slice(0, 3)} ${parts.year}`;
};

export const MONTHS_SHORT = MONTHS.map((m) => m.slice(0, 3));

// The year grid pages in blocks. 12 at a time fits the same 3-column shape the month grid
// uses, so stepping year -> month -> day never changes the size of the panel.
export const YEARS_PER_PAGE = 12;

// The block a year belongs to, anchored so the same set always comes back for a given year
// rather than shifting depending on how you arrived at it.
export const yearPageStart = (year) => Math.floor(year / YEARS_PER_PAGE) * YEARS_PER_PAGE;

export const yearPage = (year) => {
    const start = yearPageStart(year);
    return Array.from({ length: YEARS_PER_PAGE }, (_, i) => start + i);
};
