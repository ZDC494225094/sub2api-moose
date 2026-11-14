-- NULL follows the global threshold; zero explicitly removes the threshold.
ALTER TABLE invoice_user_access ADD COLUMN min_amount NUMERIC(20,2)
    CHECK (min_amount IS NULL OR (min_amount >= 0 AND min_amount <= 100000000));
