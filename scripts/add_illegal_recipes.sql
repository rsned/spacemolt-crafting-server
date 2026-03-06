-- SpaceMolt Federation Crafting Bans
-- Effective: 2026-03-06
-- Game patch banned private production of ciphers and authenticators

BEGIN TRANSACTION;

-- Wire Trade Authenticator - banned by Federation
INSERT INTO illegal_recipes (recipe_id, ban_reason, legal_location)
VALUES (
    'wire_trade_authenticator',
    'The Federation has outlawed private production.',
    'Haven Station''s Haven Authenticator Foundry'
) ON CONFLICT(recipe_id) DO UPDATE SET
    ban_reason = excluded.ban_reason,
    legal_location = excluded.legal_location,
    updated_at = CURRENT_TIMESTAMP;

-- Encode Trade Cipher - banned by Federation
INSERT INTO illegal_recipes (recipe_id, ban_reason, legal_location)
VALUES (
    'encode_trade_cipher',
    'The Federation has outlawed private production.',
    'Haven Station''s Haven Authenticator Foundry'
) ON CONFLICT(recipe_id) DO UPDATE SET
    ban_reason = excluded.ban_reason,
    legal_location = excluded.legal_location,
    updated_at = CURRENT_TIMESTAMP;

COMMIT;

-- Verification query
SELECT
    r.id as recipe_id,
    r.name as recipe_name,
    ir.ban_reason,
    ir.legal_location
FROM illegal_recipes ir
JOIN recipes r ON ir.recipe_id = r.id
ORDER BY r.name;
