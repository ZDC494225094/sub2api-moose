-- Site-customization extension migration marker
-- This migration marks the announcements table and custom UI settings as belonging
-- to the site-customization extension module.

-- Announcements table was created in 045_add_announcements.sql
-- It includes: id, title, content, status, notify_mode, targeting, starts_at, ends_at, etc.

-- Custom UI settings in the settings table:
--   - custom_menu_items (JSON array of menu items)
--   - footer_friend_links (JSON array of footer links)
--   - custom_endpoints (JSON array of API endpoint configs)
--   - site_name (site branding, considered basic functionality, NOT controlled by extension)

-- Extension control behavior:
-- When site-customization extension is DISABLED:
--   1. Backend filters custom_menu_items, footer_friend_links, custom_endpoints from public API
--   2. Admin API rejects new announcement creation (returns CUSTOM_EXTENSION_DISABLED)
--   3. Existing published announcements continue to display (historical content preserved)
--   4. Announcement read status continues to function
--   5. Frontend hides custom UI elements via useSiteCustomizationAdmission() hook

-- When site-customization extension is ENABLED:
--   All custom UI settings are exposed and all announcement management is available.

-- No schema changes needed; this is a marker for extension ownership.
-- Extension metadata:
--   module: site-customization
--   tables: announcements, announcement_reads
--   settings: custom_menu_items, footer_friend_links, custom_endpoints
--   native: false (二开功能，上游不存在)
