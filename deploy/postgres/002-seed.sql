-- Enough data to open the app and see something.
--
-- Runs once, after the schema, on an empty data directory. This is a LOCAL development stack -
-- the password below is published in this file on purpose, and this script must never be
-- pointed at anything real.
--
--   email     owner@local.test
--   password  password123
--
-- The hash is bcrypt cost 10, generated with bcryptjs so it is the same shape the Node app
-- writes - proving the Go service reads hashes the existing system produced rather than only
-- ones it produced itself.

INSERT INTO users (id, email, password, name, firm, role)
VALUES (
    '6a8b8c48ae946c0049e824a8',
    'owner@local.test',
    '$2a$10$.KybhKt.pYOwK0ptMK5bRu4en0jUKH2PC1fKI6DSKqR8Qw0RFwRg2',
    'Local Owner',
    'Manan Graphics',
    'admin'
);

INSERT INTO companies (id, uid, name, firm, phone, gst, address, url, is_default)
VALUES ('6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'Manan Graphics',
        'Manan Graphics LLP', '919000000000', '24ABCDE1234F1Z5',
        '12 Print Street, Ahmedabad 380001', '/uploads/default-logo.png', true);

INSERT INTO clients (id, company_id, uid, client_name, client_firm, client_phone) VALUES
    ('c00000000000000000000001', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'Priya',  'Acme Signs',   '919000000001'),
    ('c00000000000000000000002', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'Ravi',   'Bolt Print',   '919000000002'),
    ('c00000000000000000000003', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'Sam',    'Cobalt Media', '919000000003');

-- Jobs across several days, so the Daily list has more than one tile and the "still open"
-- alert has something to count. Deliberately mixed: some fully Done, some part-done, one on a
-- day that has no sheet at all - which is the case that used to be reported as a data fault.
INSERT INTO jobs (id, company_id, uid, client_id, challan_number, received_date, total) VALUES
    ('a00000000000000000000001', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'c00000000000000000000001', 'JOB/26-27/000001', current_date - 9, 12000),
    ('a00000000000000000000002', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'c00000000000000000000002', 'JOB/26-27/000002', current_date - 9,  4500),
    ('a00000000000000000000003', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'c00000000000000000000003', 'JOB/26-27/000003', current_date - 5, 26000),
    ('a00000000000000000000004', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'c00000000000000000000001', 'JOB/26-27/000004', current_date - 2,  8800),
    ('a00000000000000000000005', '6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', 'c00000000000000000000002', 'JOB/26-27/000005', current_date,     15250);

INSERT INTO job_rows (job_id, row_id, position, material, description, qty, rate, cgst, sgst, queue) VALUES
    -- Every card Done: this job-id must NOT appear in the open-work alert.
    ('a00000000000000000000001', 'JOB/26-27/000001-000001', 1, 'Vinyl',    'Shop front',      2,  3000, 9, 9, 'Done'),
    ('a00000000000000000000001', 'JOB/26-27/000001-000002', 2, 'Flex',     'Backlit panel',   1,  6000, 9, 9, 'Done'),

    ('a00000000000000000000002', 'JOB/26-27/000002-000001', 1, 'Foam',     'Counter sign',    3,  1500, 9, 9, 'Printing'),

    ('a00000000000000000000003', 'JOB/26-27/000003-000001', 1, 'ACP',      'Facade',          4,  5000, 9, 9, 'Done'),
    ('a00000000000000000000003', 'JOB/26-27/000003-000002', 2, 'Acrylic',  'Letters',        12,   500, 9, 9, 'Ready-to-Pickup'),
    ('a00000000000000000000003', 'JOB/26-27/000003-000003', 3, 'Vinyl',    'Window graphics', 1,  2000, 9, 9, 'Created'),

    ('a00000000000000000000004', 'JOB/26-27/000004-000001', 1, 'Flex',     'Hoarding',        1,  8800, 9, 9, 'Created'),

    -- An interstate job: the whole rate on IGST, nothing on CGST/SGST. The CHECK constraint
    -- in the schema is what makes the other combination impossible.
    ('a00000000000000000000005', 'JOB/26-27/000005-000001', 1, 'Vinyl',    'Exhibition set',  5,  2000, 0, 0, 'Printing'),
    ('a00000000000000000000005', 'JOB/26-27/000005-000002', 2, 'Standee',  'Roll-up',         3,  1750, 0, 0, 'Done');

UPDATE job_rows SET igst = 18 WHERE job_id = 'a00000000000000000000005';

-- Sheets for only SOME of those days, on purpose: the day list is built from
-- jobs.received_date, so every day appears regardless, and the ones with a sheet also carry
-- its id as a link target for older URLs.
INSERT INTO sheets (company_id, uid, date) VALUES
    ('6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', current_date - 9),
    ('6a8b8c48ae946c0049e824a9', '6a8b8c48ae946c0049e824a8', current_date - 5);

-- A review on the fully-Done job-id, so the alert page's reviewed state has something to show
-- on the local stack.
INSERT INTO job_reviews (uid, company_id, job_id, client_id, jobcard_id, challan_number,
                         client_name, quality, speed, communication, satisfaction, overall, comment)
VALUES ('6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9', 'a00000000000000000000001',
        'c00000000000000000000001', 'JOB/26-27/000001-000001', 'JOB/26-27/000001', 'Acme Signs',
        5, 4, 5, 5, 5, 'Fast and clean work.');

-- Bank accounts, so /bank/list has rows on the local stack.
INSERT INTO banks (id, uid, company_id, name, opening_balance) VALUES
    ('b0000000000000000000000a', '6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9', 'Cash Drawer',  2500),
    ('b0000000000000000000000b', '6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9', 'HDFC Current', 50000);

-- Products, so the Products tab has rows on the local stack.
INSERT INTO materials (id, uid, company_id, material_name, material_rate, purchase_rate, unit, hsn, tax) VALUES
    ('d00000000000000000000001', '6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9', 'Vinyl Sticker', 45, 30, 'SQ. Ft', '4911', 18),
    ('d00000000000000000000002', '6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9', 'Flex Banner',   25, 15, 'SQ. Ft', '4911', 18);
INSERT INTO material_price_history (material_id, material_rate, purchase_rate) VALUES
    ('d00000000000000000000001', 45, 30),
    ('d00000000000000000000002', 25, 15);

-- People: an employee (with permissions) and a supplier, so both Configure tabs have rows.
INSERT INTO persons (uid, name, type, email, phone, firm, permissions, notify_po_created) VALUES
    ('6a8b8c48ae946c0049e824a8', 'Anita', 'Employee', 'anita@local.test', '919000001111', '', ARRAY['products','customers'], NULL);
INSERT INTO persons (id, uid, name, type, phone, firm, gst, opening_balance, notify_po_created) VALUES
    ('f00000000000000000000001', '6a8b8c48ae946c0049e824a8', 'Metro Papers', 'Supplier', '919000002222', 'Metro Papers Pvt Ltd', '24AAAAA0000A1Z5', 12000, true);

-- A quotation with two rows for Priya, so the Quotations screen has content.
INSERT INTO quotations (id, uid, company_id, client_id, quotation_number, date) VALUES
    ('e00000000000000000000001', '6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9',
     'c00000000000000000000001', 'QT/26-27/000001', '2026-09-10');
INSERT INTO quotation_rows (quotation_id, position, material, description, qty, rate, cgst, sgst) VALUES
    ('e00000000000000000000001', 1, 'Vinyl Sticker', 'Window decals', 10, 45, 9, 9),
    ('e00000000000000000000001', 2, 'Flex Banner',   'Entrance banner', 1, 2500, 9, 9);

-- A supplier bill from Metro Papers, so the Purchase Invoices screen has content.
INSERT INTO purchase_invoices (id, uid, company_id, supplier_id, date, invoice_number, total, amount) VALUES
    ('10000000000000000000000a', '6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9',
     'f00000000000000000000001', '2026-09-08', 'MP-4471', 11800, 5000);
INSERT INTO purchase_invoice_rows (invoice_id, position, description, material, hsn, gst, rate, qty, unit) VALUES
    ('10000000000000000000000a', 1, 'Art paper 300gsm', 'Art Paper', '4802', 18, 100, 100, 'Sheet');

-- An invoice for Priya with two billed entries, so the Invoices screen has content.
INSERT INTO invoices (id, uid, company_id, client_id, invoice_id, amount, total_amount, date) VALUES
    ('20000000000000000000000a', '6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9',
     'c00000000000000000000001', 'INV/26-27/000001', 5000, 11800, '2026-09-11');
INSERT INTO entries (uid, company_id, client_id, invoice_id, description, material, hsn, rate, qty,
                     has_dimensions, length, width, date, amount, cgst, sgst, advance, total, has_issued) VALUES
    ('6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9', 'c00000000000000000000001', '20000000000000000000000a',
     'Window decals', 'Vinyl Sticker', '4911', 45, 10, false, '1', '1', '2026-09-11', 4500, 9, 9, 3000, 5310, true),
    ('6a8b8c48ae946c0049e824a8', '6a8b8c48ae946c0049e824a9', 'c00000000000000000000001', '20000000000000000000000a',
     'Entrance banner', 'Flex Banner', '4911', 2500, 1, false, '1', '1', '2026-09-11', 2500, 9, 9, 2000, 950, true);

-- A couple of challans so the Challan tab has content.
INSERT INTO challans (uid, company_id, company_name, description, date, type, quantity, amount) VALUES
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','Acme Signs','Delivered banners','2026-09-12','Delivery',5,4500),
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','Bolt Print','Returned samples','2026-09-13','Return',2,0);

-- Expenses so the Expenses view has content.
INSERT INTO expenses (uid, company_id, bank_id, date, amount, notes) VALUES
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','b0000000000000000000000b','2026-09-10',1200,'Ink cartridges'),
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','b0000000000000000000000a','2026-09-12',500,'Courier');

-- A payment received against the seeded invoice, so revenue's collected series is non-zero.
INSERT INTO invoice_received (uid, company_id, client_id, invoice_id, date, amount) VALUES
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','c00000000000000000000001','20000000000000000000000a','2026-09-13',5000);

-- Wastage rows.
INSERT INTO wastages (uid, company_id, material_name, rate, purchase_rate, cost_total, length, height, total, date) VALUES
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','Vinyl Sticker',45,30,90,2,1,2,'2026-09-11'),
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','Flex Banner',25,15,45,3,1,3,'2026-09-12');

-- A batch receive allocated to a job, so Bank Transfers shows a transfer with a destination.
INSERT INTO batch_receives (uid, company_id, client_id, bank_id, date, amount, note, mode, allocations) VALUES
    ('6a8b8c48ae946c0049e824a8','6a8b8c48ae946c0049e824a9','c00000000000000000000001','b0000000000000000000000b','2026-09-13',3000,'NEFT ref 8891','manual',
     '[{"job_id":"a00000000000000000000001","amount":3000}]');
