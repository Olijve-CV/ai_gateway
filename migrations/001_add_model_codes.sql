-- Add model_codes column to provider_configs table
ALTER TABLE provider_configs ADD COLUMN model_codes TEXT DEFAULT '';