ALTER TABLE slot_participations DROP CONSTRAINT IF EXISTS slot_participations_title_length;
ALTER TABLE slot_participations DROP COLUMN IF EXISTS title;
