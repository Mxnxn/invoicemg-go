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
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
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
