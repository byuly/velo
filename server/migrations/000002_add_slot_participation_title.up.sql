ALTER TABLE slot_participations ADD COLUMN title TEXT;
ALTER TABLE slot_participations ADD CONSTRAINT slot_participations_title_length
    CHECK (char_length(title) <= 30);
