-- Migrate existing 'member' role to 'deployer' for RBAC.
UPDATE users SET role = 'deployer' WHERE role = 'member';
