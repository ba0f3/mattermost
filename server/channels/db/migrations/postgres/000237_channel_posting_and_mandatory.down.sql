DROP INDEX IF EXISTS idx_channels_teamid_mandatory;

ALTER TABLE Channels
    DROP COLUMN IF EXISTS PostingPolicy,
    DROP COLUMN IF EXISTS PostingUserIDs,
    DROP COLUMN IF EXISTS Mandatory;
